package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// prose builds a corpus of one document with the given body, using the shipped markers'
// shape: longest first, as the loader sorts them.
func prose(body string) *lint.Snapshot {
	return &lint.Snapshot{
		LanguageMarkers: []lint.LanguageMarker{
			{Phrase: "significantly better", Role: "comparison"},
			{Phrase: "industry-leading", Role: "comparison"},
			{Phrase: "needless to say", Role: "assurance"},
			{Phrase: "studies show", Role: "weasel"},
			{Phrase: "generally", Role: "hedge"},
			{Phrase: "clearly", Role: "assurance"},
		},
		Documents: []lint.Document{{Path: "c/a.md", Type: "Reference", Body: body}},
	}
}

// TestOneFindingPerDocumentNamingThePhrases is the adversarial case, and §17.3.1's
// `coverage` already showed what the alternative costs.
//
// A word list applied to every body is the loudest thing this tool can do. Reported per
// occurrence, a page saying "clearly" four times produces four findings and teaches its
// reader to skip the category. One line naming the phrases lets them judge the register
// of the page and move on.
func TestOneFindingPerDocumentNamingThePhrases(t *testing.T) {
	t.Parallel()
	snap := prose("Clearly this works. Clearly it scales. Clearly, studies show it.")
	got := runNamed(t, snap, "language")
	if len(got) != 1 {
		t.Fatalf("want one finding for one document, got %d:\n%s",
			len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{`"clearly" (assurance)`, `"studies show" (weasel)`} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not name %q:\n%s", want, got[0])
		}
	}
	// Named, never counted: §17 forbids a count presented as health, and an occurrence
	// count is the number somebody would drive to zero by rewording rather than by
	// thinking.
	if strings.Contains(got[0], "3 ") {
		t.Errorf("the finding counts occurrences:\n%s", got[0])
	}
}

// TestALongerPhraseSuppressesAShorterOneInsideIt keeps one phrase from being reported
// twice under two names. "needless to say" contains no listed word here, but the rule it
// exercises is the general one: markers arrive longest-first and a match is consumed.
func TestALongerPhraseSuppressesAShorterOneInsideIt(t *testing.T) {
	t.Parallel()
	snap := prose("This is significantly better than the alternative.")
	got := runNamed(t, snap, "language")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d", len(got))
	}
	if !strings.Contains(got[0], "1 marked phrase") {
		t.Errorf("one phrase was reported as more than one:\n%s", got[0])
	}
}

// TestPlainProseIsSilent keeps the check from firing on the writing it is asking for.
func TestPlainProseIsSilent(t *testing.T) {
	t.Parallel()
	snap := prose("The service retries three times, then returns the last error.")
	if got := runNamed(t, snap, "language"); len(got) != 0 {
		t.Errorf("plain prose was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestAWordBoundaryIsRequiredForPhrases keeps "generally" from matching inside a longer
// word — the same guard every lexical check in this package needs.
func TestAWordBoundaryIsRequiredForPhrases(t *testing.T) {
	t.Parallel()
	snap := prose("The generallyaccepted spelling is wrong but not hedged.")
	if got := runNamed(t, snap, "language"); len(got) != 0 {
		t.Errorf("a word containing a marker was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestNoLanguageMarkersSkipsRatherThanPasses is the absence-of-the-ruler case.
func TestNoLanguageMarkersSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := prose("Clearly this works.")
	snap.LanguageMarkers = nil
	if reason := skipReason(t, snap, "language"); !strings.Contains(reason, "language.toml") {
		t.Errorf("the skip does not name the missing file: %q", reason)
	}
}
