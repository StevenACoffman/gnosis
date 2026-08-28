package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// leads builds a corpus of one normative claim with the given lead.
func leads(lead string) *lint.Snapshot {
	return &lint.Snapshot{
		Indicators: lint.Indicators{
			Reason: []string{"although", "because", "since", "while", "when", "if"},
			Conclusion: []string{
				"it follows that",
				"consequently",
				"therefore",
				"hence",
				"thus",
				"so",
			},
		},
		Vocabulary: lint.Vocabulary{
			Declared: true,
			Types:    []lint.VocabType{{Key: "Rule", Normative: true}},
		},
		Documents: []lint.Document{{
			Path: "c/a.md", Type: "Rule",
			Claims: []lint.Claim{{ID: "c1", Anchor: "An assertion.", Lead: lead}},
		}},
	}
}

// TestAConclusionMarkerAtTheStartIsCorrect is the adversarial case, and the one that
// decides whether authors keep their connectives.
//
// "Therefore, cap retries at three." *is* conclusion-first — the marker is a connective
// attached to a conclusion, not a sign that one was buried. A check reporting it would
// teach authors to strip the words that make prose readable, which is the opposite of
// what §17.4 wants from them.
func TestAConclusionMarkerAtTheStartIsCorrect(t *testing.T) {
	t.Parallel()
	for _, lead := range []string{
		"Therefore, cap retries at three.",
		"Thus retries are capped at three.",
		"So the retry budget is three.",
	} {
		if got := runNamed(t, leads(lead), "lead"); len(got) != 0 {
			t.Errorf("a conclusion-first lead was reported: %q\n%s",
				lead, strings.Join(got, "\n"))
		}
	}
}

// TestALeadOpeningWithAReasonIsReported is the first of §17.4's two shapes: the claim
// leads with its derivation, so an agent taking the first few words gets background.
func TestALeadOpeningWithAReasonIsReported(t *testing.T) {
	t.Parallel()
	got := runNamed(t, leads("Because latency is high, cap retries at three."), "lead")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(got), got)
	}
	for _, want := range []string{"opens with a reason", "context budget"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not say %q:\n%s", want, got[0])
		}
	}
}

// TestALeadBuryingItsConclusionIsReported is the second shape: the conclusion is present
// but sits behind the reasoning that produced it.
func TestALeadBuryingItsConclusionIsReported(t *testing.T) {
	t.Parallel()
	got := runNamed(t, leads("Latency is high, therefore cap retries at three."), "lead")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "behind the reasoning") {
		t.Errorf("the finding does not say which shape it is:\n%s", got[0])
	}
}

// TestAPlainConclusionIsSilent keeps the check from firing on the thing it is asking
// for. Most correct leads carry no marker at all.
func TestAPlainConclusionIsSilent(t *testing.T) {
	t.Parallel()
	for _, lead := range []string{
		"Cap retries at three.",
		"The retry budget is three attempts.",
	} {
		if got := runNamed(t, leads(lead), "lead"); len(got) != 0 {
			t.Errorf("a plain conclusion was reported: %q\n%s",
				lead, strings.Join(got, "\n"))
		}
	}
}

// TestAWordBoundaryIsRequiredForMarkers keeps "sofa" from matching "so" and "iffy" from
// matching "if" — the same guard segmentation needed for the same word list.
func TestAWordBoundaryIsRequiredForMarkers(t *testing.T) {
	t.Parallel()
	for _, lead := range []string{
		"Software caps retries at three.",
		"Iffy configurations are capped at three.",
	} {
		if got := runNamed(t, leads(lead), "lead"); len(got) != 0 {
			t.Errorf("a word merely starting with a marker was reported: %q\n%s",
				lead, strings.Join(got, "\n"))
		}
	}
}

// TestANonNormativeTypeIsNotChecked is §17.4's own scope: "On a normative claim it is
// checked". A Reference records rather than prescribes, and its lead is convention.
func TestANonNormativeTypeIsNotChecked(t *testing.T) {
	t.Parallel()
	snap := leads("Because latency is high, cap retries at three.")
	snap.Vocabulary.Types[0].Normative = false
	if reason := skipReason(t, snap, "lead"); !strings.Contains(reason, "prescribing") {
		t.Errorf("the skip does not name the scope: %q", reason)
	}
}

// TestAClaimWithNoLeadIsSilent is §5.8.3's argument one field over: a lead is optional in
// a reply, and reporting its absence would make a review signal into an authoring rule.
func TestAClaimWithNoLeadIsSilent(t *testing.T) {
	t.Parallel()
	snap := leads("Cap retries at three.")
	snap.Documents[0].Claims = append(snap.Documents[0].Claims,
		lint.Claim{ID: "c2", Anchor: "Another assertion."})

	for _, g := range runNamed(t, snap, "lead") {
		if strings.Contains(g, "c2") {
			t.Errorf("a claim with no lead was reported:\n%s", g)
		}
	}
}

// TestNoIndicatorWordsSkipsRatherThanPasses is the absence-of-the-ruler case.
func TestNoIndicatorWordsSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := leads("Because latency is high, cap retries.")
	snap.Indicators = lint.Indicators{}
	if reason := skipReason(t, snap, "lead"); !strings.Contains(reason, "indicators.toml") {
		t.Errorf("the skip does not name the missing file: %q", reason)
	}
}
