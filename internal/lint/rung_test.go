package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// registers is the smallest register list that can tell the three states apart.
//
// **The bare "associated with" is here because the first version omitted it and two
// tests failed for what looked like a code defect.** Real prose writes *are* associated,
// *was* associated, *were* associated, and a list carrying only the "is" form matches
// none of them. `standards/registers.toml` ships both; a fixture that shipped one would
// have tested a list nobody uses. The shipped file is pinned against a real sentence in
// internal/standards.
func registers() lint.Registers {
	return lint.Registers{
		Intervention: []string{"leads to", "causes"},
		Association:  []string{"is associated with", "associated with", "correlates with"},
	}
}

// rungSnap is one claim with its anchor and quotations.
func rungSnap(anchor string, quotes ...string) *lint.Snapshot {
	return &lint.Snapshot{
		Registers: registers(),
		Documents: []lint.Document{{
			Path:   "c/a.md",
			Claims: []lint.Claim{{ID: "c1", Anchor: anchor, Quotes: quotes}},
		}},
	}
}

// TestACausalClaimOnObservationalEvidenceIsReported is §17.3.1.1's own case: the claim
// says one thing was done and the source says two things were seen together.
func TestACausalClaimOnObservationalEvidenceIsReported(t *testing.T) {
	t.Parallel()
	snap := rungSnap(
		"Restarting the pod causes the leak to clear.",
		"pod restarts are associated with lower memory use",
	)
	got := runNamed(t, snap, "rung")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	// Both readings, because a reader who is told only "rung mismatch" cannot tell
	// which half to change.
	for _, want := range []string{"causes", "associated with", "weaken the wording"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not mention %q:\n%s", want, got[0])
		}
	}
}

// TestEvidenceThatSaysNothingAboutCausationIsSilent is the adversarial case, and the one
// that would do real damage.
//
// Most quotations carry no register word at all. If silence read as "observational",
// this check would report nearly every causal claim in a corpus on the strength of
// evidence it never actually examined — asserting from absence, which is exactly what
// §9.4's `Unchecked` outcome exists to refuse one layer down.
func TestEvidenceThatSaysNothingAboutCausationIsSilent(t *testing.T) {
	t.Parallel()
	snap := rungSnap(
		"Restarting the pod causes the leak to clear.",
		"the pod was restarted at 14:02 and memory returned to 40MB",
	)
	if got := runNamed(t, snap, "rung"); len(got) != 0 {
		t.Errorf("a quotation that said nothing about causation was read as "+
			"observational:\n%s", strings.Join(got, "\n"))
	}
}

// TestOneInterventionalQuotationClearsTheClaim keeps the check from demanding that every
// quotation carry the causal word. A claim resting on three passages needs one of them to
// support the intervention; requiring all three would report a well-evidenced claim for
// the sin of also citing context.
func TestOneInterventionalQuotationClearsTheClaim(t *testing.T) {
	t.Parallel()
	snap := rungSnap(
		"Restarting the pod causes the leak to clear.",
		"restarts are associated with lower memory use",
		"the fix causes the allocation to be freed",
	)
	if got := runNamed(t, snap, "rung"); len(got) != 0 {
		t.Errorf("a claim with interventional evidence was reported:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestAnObservationalClaimIsNotReported is the other silent state: a claim that already
// said "is associated with" matches whatever observed it, and reporting it would tell an
// author their careful wording was the problem.
func TestAnObservationalClaimIsNotReported(t *testing.T) {
	t.Parallel()
	snap := rungSnap(
		"Pod restarts are associated with lower memory use.",
		"restarts correlates with lower memory use",
	)
	if got := runNamed(t, snap, "rung"); len(got) != 0 {
		t.Errorf("an observational claim was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestInterventionWinsWhenAClaimSaysBoth is the asymmetry the check turns on. "X is
// associated with Y and causes Z" asserts causation somewhere in it; reading it as
// observational would let one hedged clause launder a causal claim.
func TestInterventionWinsWhenAClaimSaysBoth(t *testing.T) {
	t.Parallel()
	snap := rungSnap(
		"Restarting is associated with lower memory and causes the leak to clear.",
		"restarts are associated with lower memory use",
	)
	if got := runNamed(t, snap, "rung"); len(got) != 1 {
		t.Errorf("a claim asserting both rungs was read as observational: %d finding(s)",
			len(got))
	}
}

// TestUndeclaredRegistersSkipRatherThanCondemn is §12's applicability rule, and the
// reason this is its own check rather than a second branch inside `coverage`: a corpus
// that declares strength markers but no registers must be told which half is
// unavailable, and a check that silently declines is indistinguishable from one that
// found nothing.
func TestUndeclaredRegistersSkipRatherThanCondemn(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Documents: []lint.Document{{
			Path: "c/a.md",
			Claims: []lint.Claim{
				{ID: "c1", Anchor: "Restarting causes it.", Quotes: []string{"seen"}},
			},
		}},
	}
	if reason := skipReason(t, snap, "rung"); !strings.Contains(reason, "registers.toml") {
		t.Errorf("rung skipped for %q, which does not name the missing file", reason)
	}
}
