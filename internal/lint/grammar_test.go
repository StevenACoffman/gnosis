package lint_test

import (
	"regexp"
	"strings"
	"testing"
)

// singularCountVerb matches a count of one followed by a noun and a plural verb form.
//
// **A detector, not an allowlist, and the difference is which way it fails.** A verb
// missing from this list means one defect slips through; it can never bless a message
// that is wrong. §12.1's hand-maintained check list failed the other way — it claimed
// enforcement that had been deleted — which is why that one was replaced by something
// derived and this one is acceptable as a list.
//
// The pattern requires the literal `1` because that is the only count that disagrees:
// "2 documents declare" is correct, and so is "0 documents declare".
//
// **The noun is one word, and a relative pronoun may follow it.** An earlier version
// allowed a second noun word and read "no document is of 1 declared type Reference" as a
// disagreement, because `reference` was on the verb list. A false positive here is the
// damaging direction — it makes `noun`'s own output look like the defect, and somebody
// would "fix" a correct message by breaking it.
//
// **Verbs that are also ordinary nouns are left off for the same reason**: state, use,
// match, hold, report, point, span and reference all appear as nouns in this corpus's
// vocabulary. So "1 document state" is missed. That is the miss this list buys, and it is
// the cheap direction.
var singularCountVerb = regexp.MustCompile(
	`\b1 [a-z][a-z-]* (?:that |which )?` +
		`(?:are|were|do|don't|have|declare|name|resolve|carry|assert|` +
		`require|expect|contain|cite|parse|exceed|quote|remain|refer|lack)\b`)

// grammarOf reports the count-noun disagreements in one rendered finding.
//
// Requires: message is a diagnostic as `gnosis lint` would print it.
// Ensures: an empty slice for a message that agrees. Pure.
func grammarOf(message string) []string {
	return singularCountVerb.FindAllString(strings.ToLower(message), -1)
}

// TestTheDetectorFindsTheThreeMessagesThatShipped is §18.4.1's rule applied to a detector:
// the cases are taken from the artifact, and every one of these is a sentence this tool
// actually printed.
//
// The defect has three recorded occurrences — "1 document declare", "1 claim name", and
// "1 command that do not resolve" — each caught by running the binary and none by a
// substring assertion, because an assertion looking for `1 document` finds it and stops.
func TestTheDetectorFindsTheThreeMessagesThatShipped(t *testing.T) {
	t.Parallel()

	shipped := []string{
		"1 document declare a type the vocabulary does not hold",
		"1 claim name a subject that does not resolve",
		"AGENTS.md names 1 command that do not resolve: gnosis frobnicate",
	}
	for _, message := range shipped {
		if got := grammarOf(message); len(got) == 0 {
			t.Errorf("the detector misses a message this tool shipped: %q", message)
		}
	}
}

// TestTheDetectorLeavesTheFixedFormsAlone is the adversarial half, and the one that would
// do real damage: `Noun(n, word)` renders "1 unresolvable command" and "1 quotation", and
// a detector that flagged those would make the remedy look like the defect. Somebody would
// then "fix" a correct message by breaking it.
func TestTheDetectorLeavesTheFixedFormsAlone(t *testing.T) {
	t.Parallel()

	correct := []string{
		"names 1 unresolvable command: gnosis frobnicate",
		"claim c1 asserts \"always\" on 1 quotation",
		"no document is of 1 declared type Reference",
		"2 documents declare a type the vocabulary does not hold",
		"0 documents declare a type the vocabulary does not hold",
		"1 claim parsed to a value, 2 claims lost",
		"the index holds 1 object the migrations do not describe",
	}
	for _, message := range correct {
		if got := grammarOf(message); len(got) != 0 {
			t.Errorf("a correct message was flagged as %v: %q", got, message)
		}
	}
}
