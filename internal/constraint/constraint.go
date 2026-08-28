// Package constraint parses a claim's prose into the comparable value §10.2 needs.
//
// **The prose is authoritative and this is a cached reading of it** (§10.2.1). A
// constraint never redefines what a claim says: it is regenerable from the text, so an
// improved pattern set fixes every affected claim retroactively on the next reindex, and
// nothing here has to be authored by hand.
//
// Everything is pure. The patterns arrive as a parameter rather than as an import,
// because a parser must not import `internal/standards` — the same rule that shaped
// `internal/segment`'s word list and `internal/lint`'s vocabulary.
package constraint

import (
	"strconv"
	"strings"

	"github.com/rodaine/numwords"

	"github.com/StevenACoffman/skillet/textnorm"
)

// The operators a pattern may yield. OpUnset is the zero value and names nothing, so a
// Constraint nobody populated cannot be read as an assertion about zero.
const (
	OpUnset   OpKind = ""
	OpAtMost  OpKind = "<="
	OpAtLeast OpKind = ">="
	OpExactly OpKind = "=="
)

// OpKind is how a constraint bounds its value.
type OpKind string

// Pattern is one phrasing that yields an operator, as `standards/` declares it.
type Pattern struct {
	// ID names the pattern, so a finding can say which one produced its reading
	// (§10.2.2) and `claim_subjects.pattern_id` can record it.
	ID string

	// Phrase is the text that introduces the value, matched under fold on a word
	// boundary. Lower case.
	Phrase string

	// Op is what the phrase means.
	Op OpKind
}

// Constraint is a claim's parsed reading: an operator, a value, and the text it came
// from.
type Constraint struct {
	Op OpKind

	// Value is the quantity, normalised. A float because §7.3's decisive argument was
	// that the quantities here are "2.5 seconds" and "99.9%" — an integer-only reading
	// fails on the common case.
	Value float64

	// PatternID is which pattern produced this reading.
	PatternID string

	// Raw is the span the value was read from, shown beside a finding so an
	// adjudicator sees the parse rather than only a verdict (§10.2.2).
	Raw string
}

// Valid reports whether k is one of the three operators.
//
// Requires: nothing.
// Ensures: false for OpUnset, which is what makes a pin stating no operator readable as
// "there is no pin" rather than as a bound the zero value would otherwise assert. Pure.
func (k OpKind) Valid() bool {
	switch k {
	case OpAtMost, OpAtLeast, OpExactly:
		return true
	case OpUnset:
		return false
	default:
		return false
	}
}

// Parse reads a constraint out of prose.
//
// Requires: patterns are lower-cased; text is a claim's anchor.
// Ensures: comma-ok. A text that matches no pattern, or matches one and carries no
// number after it, yields false — never a zero-valued Constraint, which would assert a
// bound of zero. Pure.
//
// **Longest phrase wins**, which is the whole reason inversions are the first cases in
// the pattern file: `"no fewer than three"` and `"no more than three"` differ by one word
// and invert the operator, so a matcher that stopped at the first hit would read a floor
// as a ceiling. Sorting by phrase length is what makes that a property of the data rather
// than of the order somebody wrote the rows in.
func Parse(text string, patterns []Pattern) (Constraint, bool) {
	// Spelled-out numbers first, in place: §7.3 pins `numwords` for exactly this, so
	// "no more than three retries" becomes "no more than 3 retries" before any pattern
	// has to know about English.
	//
	// **Punctuation is separated from words before that call, and the reason was found
	// by running the tool.** `numwords` does not convert a number word with punctuation
	// attached: "three." stays "three.". A claim's anchor is a sentence and ends in a
	// period, so without this the *commonest* form silently parsed to nothing — and the
	// first test corpus missed it because its cases were written without punctuation,
	// which is §11.0.2's warning about an instrument authored from imagination.
	normalised := numwords.ParseString(spacePunctuation(text))
	folded := " " + strings.ToLower(textnorm.Fold(normalised)) + " "

	best := -1
	var chosen Pattern
	for _, p := range patterns {
		at := strings.Index(folded, " "+p.Phrase+" ")
		if at < 0 {
			continue
		}
		if len(p.Phrase) > len(chosen.Phrase) {
			best, chosen = at+1+len(p.Phrase), p
		}
	}
	if best < 0 {
		return Constraint{}, false
	}

	if introducesAnArticle(text, chosen.Phrase) {
		return Constraint{}, false
	}
	value, raw, ok := firstNumber(folded[best:])
	if !ok {
		return Constraint{}, false
	}
	return Constraint{
		Op: chosen.Op, Value: value, PatternID: chosen.ID, Raw: strings.TrimSpace(raw),
	}, true
}

// String renders a constraint the way §10.2.2 requires a finding to show its parse.
func (c Constraint) String() string {
	return "{op: " + string(c.Op) + ", value: " +
		strconv.FormatFloat(c.Value, 'g', -1, 64) + "}"
}

// spacePunctuation separates sentence punctuation from the words beside it.
//
// Requires: nothing.
// Ensures: every rune is preserved, so nothing is lost — only spaced. A `.` or `,`
// between two digits is left attached. Pure.
//
// **The digit guard is §9.4's own list of naive-splitting failures, one layer down.**
// That section records that `split(".")` cuts `2.5 seconds` in half; spacing a decimal
// point does the same damage here, and the first version of this function turned
// "over 99.9%" into a bound of 99. Only marks that actually end or divide a clause are
// spaced, and only where they are not inside a number.
//
// An apostrophe or a hyphen inside a word is never touched: splitting "don't" or
// "sub-second" would create tokens no pattern matches and no number parses from.
func spacePunctuation(text string) string {
	runes := []rune(text)
	var b strings.Builder
	b.Grow(len(text) + 8)
	for i, r := range runes {
		if !divides(r) || insideNumber(runes, i) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte(' ')
		b.WriteRune(r)
		b.WriteByte(' ')
	}
	return b.String()
}

// divides reports whether a rune ends or separates a clause.
func divides(r rune) bool {
	switch r {
	case '.', ',', ';', ':', '!', '?', '(', ')', '[', ']':
		return true
	default:
		return false
	}
}

// insideNumber reports whether the rune at i sits between two digits.
//
// Only `.` and `,` can: a decimal point and a thousands separator. A semicolon between
// digits is not a number and spacing it loses nothing.
func insideNumber(runes []rune, i int) bool {
	if runes[i] != '.' && runes[i] != ',' {
		return false
	}
	if i == 0 || i+1 >= len(runes) {
		return false
	}
	return isDigit(runes[i-1]) && isDigit(runes[i+1])
}

// isDigit reports whether r is an ASCII digit.
func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// introducesAnArticle reports whether the phrase is followed by "a" or "an" in the
// original text.
//
// **`numwords` reads the article as the number one**, so without this guard "no more
// than a handful of retries" parses to `{op: <=, value: 1}` — a confident bound on prose
// that states none. §7.3 pinned that library for its floats and fractions and this is
// the cost: it cannot distinguish the article from the numeral, because in English they
// are the same word.
//
// Checked against the *unnormalised* text, since after normalisation the article is
// already a digit and the evidence is gone. The phrase is located independently rather
// than by offset, because numwords changes byte lengths.
func introducesAnArticle(text, phrase string) bool {
	folded := " " + strings.ToLower(textnorm.Fold(text)) + " "
	at := strings.Index(folded, " "+phrase+" ")
	if at < 0 {
		return false
	}
	rest := strings.TrimLeft(folded[at+1+len(phrase):], " ")
	next, _, _ := strings.Cut(rest, " ")
	return next == "a" || next == "an"
}

// firstNumber reads the leading number out of text, with the span it came from.
//
// Requires: text has been through numwords, so spelled-out numbers are digits.
// Ensures: false when no number appears before the next non-numeric word. Pure.
//
// **It reads the *first* number and stops.** "no more than 3 retries in 60 seconds"
// carries two, and a parser that took the last would silently bound the wrong quantity.
// Which of the two a subject means is a question the dimension answers and this function
// cannot, so it takes the one the phrase introduced and leaves the rest to
// `constraint-coverage` to report as a phrasing the patterns do not fully read.
func firstNumber(text string) (float64, string, bool) {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n'
	})
	for _, f := range fields {
		trimmed := strings.TrimRight(f, ".,;:%)")
		if trimmed == "" {
			continue
		}
		v, err := strconv.ParseFloat(trimUnit(trimmed), 64)
		if err != nil {
			// A word before any number means the phrase introduced prose rather than
			// a quantity: "no more than a handful" states no constraint.
			return 0, "", false
		}
		return v, f, true
	}
	return 0, "", false
}

// trimUnit removes a unit suffix written against its number.
//
// **"400ms" parsed to nothing until 2026-08-27, and so did "5MB".** That is §18.4.1's
// recorded failure recurring one field over — a value glued to a neighbouring token,
// exactly as `99.9%` was — and it hit the two dimensions whose conventional written form
// *always* glues the unit. Nobody writes "400 ms" in a latency budget. Two of the four
// declared dimensions had no working ordinary form, and every case in the corpus was
// spaced.
//
// **The field must begin with a digit**, so a word is still a word: "handful" and "v2"
// trim to themselves and fail to parse, which is what makes "no more than a handful"
// state no constraint rather than an invented one.
//
// **The unit is dropped from the value and kept in Raw**, which is not a detail. The
// dimension a claim is compared under comes from the subject's declaration, so a claim
// writing "400ms" about a subject declared `count` is a category error — and the only
// record that it said "ms" at all is the raw span the finding shows.
func trimUnit(field string) string {
	if field == "" || !isDigit(rune(field[0])) {
		return field
	}
	end := len(field)
	for end > 0 {
		r := rune(field[end-1])
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			break
		}
		end--
	}
	if end == 0 {
		return field
	}
	return field[:end]
}

// NormalizeNumbers rewrites spelled-out numbers in text as numerals (§7.3).
//
// Requires: text is a claim's anchor.
// Ensures: the same normalisation Parse applies before reading a quantity, so a caller
// comparing a rendered value against prose compares it against the text Parse saw. Pure.
//
// Exported for the shell to hand `constraint-drift` a claim's text with "three" already
// "3": §10.2.1's drift mechanism is to look for the pinned value in the prose, and
// `internal/lint` takes values rather than importing a parser (PLAN §0.1).
func NormalizeNumbers(text string) string {
	return numwords.ParseString(spacePunctuation(text))
}
