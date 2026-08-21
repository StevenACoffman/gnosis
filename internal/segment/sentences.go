package segment

import (
	"strings"
	"unicode"
)

// terminators end a sentence, subject to the guards in Sentences.
const terminators = ".!?"

// Sentences splits prose into sentences without an abbreviation list.
//
// Requires: text is prose; fenced code should already be removed.
// Ensures: concatenating the results with their separators reconstructs the input,
// so nothing is dropped. Returns an empty slice for empty input. It is pure.
//
// # Why there is no abbreviation list
//
// A list is the obvious approach and it does not work: it is language-specific,
// never complete, and it fails open — an abbreviation nobody listed splits a
// sentence in half and the fragment gets verified on its own. The guards below are
// structural instead, and each corresponds to one of the failure cases SPEC §9.4
// names:
//
//	2.5 seconds                  no space after the period
//	README.md, foo.bar()         no space after the period
//	https://example.com/a.html   no space after the period
//	e.g. the case                next word is lower-case
//	e.g. The case                preceding token contains a period
//	A. Turing                    preceding token is a single letter
//
// The last two are the ones a list is usually written for, and both fall out of
// looking at the token rather than consulting a dictionary. Where the guards are
// wrong they under-split, leaving a sentence joined to its neighbour — a coarser
// claim, which §9.4 tolerates, rather than a fragment, which it does not.
func Sentences(text string) []string {
	out := make([]string, 0)
	start := 0
	runes := []rune(text)

	for i, r := range runes {
		if !strings.ContainsRune(terminators, r) {
			continue
		}
		if !endsSentence(runes, i) {
			continue
		}
		if s := strings.TrimSpace(string(runes[start : i+1])); s != "" {
			out = append(out, s)
		}
		start = i + 1
	}
	if s := strings.TrimSpace(string(runes[start:])); s != "" {
		out = append(out, s)
	}
	return out
}

// endsSentence reports whether the terminator at i closes a sentence.
func endsSentence(runes []rune, i int) bool {
	next, hasNext := nextNonSpace(runes, i+1)
	// End of input always closes.
	if !hasNext {
		return true
	}
	// A terminator glued to what follows is inside a token: a version, a
	// filename, a URL, a call.
	if !unicode.IsSpace(runes[i+1]) {
		return false
	}
	// A lower-case continuation is an abbreviation, not a new sentence.
	if !unicode.IsUpper(next) {
		return false
	}
	// "!" and "?" are not used in abbreviations, so the token guards below —
	// which exist only for the period — would reject valid boundaries.
	if runes[i] != '.' {
		return true
	}
	token := precedingToken(runes, i)
	// "A. Turing": an initial, not a sentence end.
	if len([]rune(token)) == 1 {
		return false
	}
	// "e.g.", "i.e.", "U.S.A.": a token already containing a period is an
	// abbreviation whatever it happens to be.
	return !strings.Contains(token, ".")
}

// nextNonSpace returns the first non-space rune at or after i.
func nextNonSpace(runes []rune, i int) (rune, bool) {
	for ; i < len(runes); i++ {
		if !unicode.IsSpace(runes[i]) {
			return runes[i], true
		}
	}
	return 0, false
}

// precedingToken returns the run of non-space runes immediately before i.
func precedingToken(runes []rune, i int) string {
	end := i
	start := end
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	return string(runes[start:end])
}
