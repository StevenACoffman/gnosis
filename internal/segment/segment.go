// Package segment cuts prose into independently verifiable claims.
//
// It exists because a sentence is the wrong unit. *"The cache is enabled by
// default, but it is not shared across sessions"* is one sentence carrying two
// assertions, and a verifier that attaches one verdict to it reports the whole
// sentence supported when a quote validates only the first half. That is a silent
// false pass in the check the corpus most depends on (SPEC §9.4).
//
// One rule governs everything here:
//
//	Every emitted claim stands on its own, or the cut is not made.
//
// The rule is what makes over-splitting safe rather than merely tolerable. A
// fragment whose subject sat in a discarded sibling fails it, so the cut is refused
// and the sentence stays whole — a claim too coarse still validates honestly,
// whereas a claim missing its subject validates against anything.
//
// Everything here is pure: no model, no network, same input, same output.
package segment

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Claim is one independently verifiable assertion.
//
// Text and Anchor differ when a subject was recovered, and the distinction is
// load-bearing. Anchor is the span **as it appears in the document**, which is what
// SPEC §5.5.1 addresses a claim by and what `claim-anchor` looks for. Text is the
// claim **as it will be verified**, which after substitution is a sentence the
// document does not literally contain. Collapsing the two would either make the
// anchor unfindable or make the claim unverifiable.
type Claim struct {
	Text        string
	Anchor      string
	Substituted bool
}

// copulas are the verbs a subject is recognised by sitting in front of.
//
// This is a deliberately small closed list rather than a parser. Subject
// identification is only needed to answer one question — can this fragment stand
// alone — and a wrong answer must fall on the side of refusing the cut. A list
// that misses a construction refuses a cut that could have been made, which costs
// a coarser claim; a parser that guesses wrong emits a claim missing its subject,
// which costs a false pass.
func copulas() []string {
	return []string{
		"is", "are", "was", "were", "be", "been",
		"has", "have", "had",
		"does", "do", "did",
		"can", "cannot", "could", "may", "might", "must", "shall", "should",
		"will", "would",
	}
}

// pronouns are the anaphoric subjects a clause cannot stand on.
//
// A clause beginning with one of these refers to something outside itself, so it
// is exactly the fragment the rule above exists to catch.
func pronouns() []string {
	return []string{"it", "they", "this", "these", "that", "those", "he", "she", "we"}
}

// conjunctions are the joins between two independent assertions.
//
// Only coordinating joins appear here. A subordinating join ("because", "when",
// "although") makes one clause depend on the other, so cutting there would produce
// a claim whose truth conditions changed — which the rule forbids for the same
// reason it forbids a missing subject.
func conjunctions() []string {
	return []string{", but ", ", and ", "; however, ", "; ", ", however, "}
}

// Claims cuts text into independently verifiable claims.
//
// Requires: text is prose. Fenced code blocks should be removed by the caller;
// inline spans are harmless because the sentence rules below do not split inside a
// token.
// Ensures: every returned claim stands alone — no claim's subject sits in a
// discarded sibling. Concatenating the anchors in order reconstructs the prose
// minus its separators, so no assertion is dropped. Returns an empty slice, never
// nil. It is pure.
func Claims(text string, dependent []string) []Claim {
	out := make([]Claim, 0)
	for _, sentence := range Sentences(text) {
		out = append(out, split(sentence, dependent)...)
	}
	return out
}

// split cuts one sentence at a coordinating conjunction, or declines to.
func split(sentence string, dependent []string) []Claim {
	whole := []Claim{{Text: sentence, Anchor: sentence}}

	at, join := earliestJoin(sentence)
	if at < 0 {
		return whole
	}
	left := strings.TrimSpace(sentence[:at])
	right := strings.TrimSpace(sentence[at+len(join):])
	if left == "" || right == "" {
		return whole
	}

	// **The copula test below cannot see this and it is why the words are data.**
	// "The retry budget is three, and because the SLA is 400ms." cuts cleanly by
	// every rule above, and the right half — "Because the SLA is 400ms." — is a
	// fragment whose main clause is in its sibling. standsAlone accepts it because
	// it finds a copula, and a copula is exactly what such a fragment has. Only
	// knowing what *because* does refuses it (§9.4.1).
	if opensDependent(right, dependent) {
		return whole
	}

	// The right clause stands on its own already: cut and keep both as written.
	if !startsWithPronoun(right) {
		if !standsAlone(right) {
			return whole
		}
		return []Claim{
			{Text: left, Anchor: left},
			{Text: capitalize(right), Anchor: right},
		}
	}

	// It does not. Recover the subject from the left clause or refuse the cut —
	// this is the branch the whole package exists for.
	subject, ok := subjectOf(left)
	if !ok {
		return whole
	}
	return []Claim{
		{Text: left, Anchor: left},
		{Text: substitute(subject, right), Anchor: right, Substituted: true},
	}
}

// earliestJoin returns the offset and text of the first coordinating join.
func earliestJoin(sentence string) (int, string) {
	best, found := -1, ""
	lower := strings.ToLower(sentence)
	for _, j := range conjunctions() {
		if at := strings.Index(lower, j); at >= 0 && (best < 0 || at < best) {
			best, found = at, j
		}
	}
	return best, found
}

// startsWithPronoun reports whether a clause opens with an anaphoric subject.
func startsWithPronoun(clause string) bool {
	first := strings.ToLower(firstWord(clause))
	for _, p := range pronouns() {
		if first == p {
			return true
		}
	}
	return false
}

// standsAlone reports whether a clause has both a subject and a verb of its own.
//
// The test is that a copula appears with at least one word before it. A clause with
// no recognised verb might still be independent, and is refused rather than
// guessed at.
func standsAlone(clause string) bool {
	_, ok := subjectOf(clause)
	return ok
}

// subjectOf returns the words before a clause's first copula.
//
// Requires: nothing.
// Ensures: reports false when no copula is found or when nothing precedes it,
// which are the two states in which a subject cannot be recovered. The result is
// the raw leading text, so substitution preserves the author's wording rather than
// normalising it.
func subjectOf(clause string) (string, bool) {
	words := strings.Fields(clause)
	for i, w := range words {
		if i == 0 {
			continue
		}
		if isCopula(w) {
			return strings.Join(words[:i], " "), true
		}
	}
	return "", false
}

// isCopula reports whether a word is one of the recognised verbs, ignoring
// trailing punctuation so "is," and "is" are one word.
func isCopula(word string) bool {
	word = strings.ToLower(strings.Trim(word, `,.;:!?"'`))
	for _, c := range copulas() {
		if word == c {
			return true
		}
	}
	return false
}

// substitute replaces a clause's leading pronoun with a recovered subject.
func substitute(subject, clause string) string {
	words := strings.Fields(clause)
	if len(words) == 0 {
		return clause
	}
	words[0] = subject
	return capitalize(strings.Join(words, " "))
}

// capitalize upper-cases the first rune, so a clause promoted to a claim reads as
// a sentence. Only the first rune is touched: the rest is the author's, and a
// clause containing an acronym must not have it flattened.
func capitalize(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	if width == 0 || unicode.IsUpper(first) {
		return s
	}
	return string(unicode.ToUpper(first)) + s[width:]
}

// firstWord returns the leading whitespace-delimited token.
func firstWord(s string) string {
	if fields := strings.Fields(s); len(fields) > 0 {
		return strings.Trim(fields[0], `,.;:!?"'`)
	}
	return ""
}

// opensDependent reports whether a clause begins with a word making it depend on
// its neighbour.
//
// Requires: markers are lower-cased; an empty list refuses nothing, which is the
// behaviour before the word list existed.
// Ensures: matches on a word boundary, so "since" matches and "sincere" does not.
// Pure.
//
// **Matching is a prefix rather than a search**, because these words only make a
// clause dependent when they introduce it. "We keep three because the SLA is tight"
// is one independent assertion containing the word; refusing every clause that
// merely mentioned it would stop segmentation almost entirely.
func opensDependent(clause string, markers []string) bool {
	lower := strings.ToLower(strings.TrimSpace(clause))
	for _, m := range markers {
		if !strings.HasPrefix(lower, m) {
			continue
		}
		rest := lower[len(m):]
		if rest == "" || !isWordByte(rest[0]) {
			return true
		}
	}
	return false
}

// isWordByte reports whether b continues a word, so a marker must end at a boundary.
func isWordByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	default:
		return b == '_' || b == '-'
	}
}
