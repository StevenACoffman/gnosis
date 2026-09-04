package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// TestWarrantReportsADecisionWithNoReasoning. §10.6.4 calls the rationale the real
// gate, and this is the check that keeps it from being a formality.
func TestWarrantReportsADecisionWithNoReasoning(t *testing.T) {
	t.Parallel()

	snap := adjudicated(&lint.Claim{
		ID: "c1", Warrant: gnosis.Warrant{By: "human:priya", At: "2026-08-19T14:02:11Z"},
	})

	got := runNamed(t, snap, "warrant")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "no rationale") {
		t.Errorf("the finding does not name the missing field:\n%s", got[0])
	}
}

// TestWarrantReportsASupersessionWithNoWarrant is the half of §12's row that needs no
// self-assertion: a claim that displaced another was adjudicated whether or not
// anybody wrote the decision down, and the edge is the evidence.
func TestWarrantReportsASupersessionWithNoWarrant(t *testing.T) {
	t.Parallel()

	snap := adjudicated(&lint.Claim{
		ID: "c1", Supersedes: []string{"01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"},
	})

	got := runNamed(t, snap, "warrant")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "never written down") {
		t.Errorf("the finding does not say what is missing:\n%s", got[0])
	}
}

// TestWarrantIsSilentOnAReasonedDecision is the other half of the pair: the check
// reports a decision with no reasoning and says nothing about one that carries it.
func TestWarrantIsSilentOnAReasonedDecision(t *testing.T) {
	t.Parallel()

	snap := adjudicated(&lint.Claim{
		ID: "c1",
		Warrant: gnosis.Warrant{
			By:        "human:priya",
			Rationale: "the vendor's published limit postdates the blog post",
		},
	})

	if got := runNamed(t, snap, "warrant"); len(got) != 0 {
		t.Errorf("a reasoned decision was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestWarrantSkipsOnACorpusThatHasAdjudicatedNothing, which is every corpus before its
// first conflict. Reporting there would report the absence of a decision nobody had to
// make.
func TestWarrantSkipsOnACorpusThatHasAdjudicatedNothing(t *testing.T) {
	t.Parallel()

	snap := &lint.Snapshot{Documents: []lint.Document{{
		ID: outerID, Path: "c/a.md", Claims: []lint.Claim{{ID: "c1"}},
	}}}

	if reason := skipReason(t, snap, "warrant"); !strings.Contains(reason, "adjudicated nothing") {
		t.Errorf("the skip does not say why: %q", reason)
	}
}

// TestCoSignReportsAnEscalatedClaimWithNoSecondSignature. Normative by type, decided
// under paired, and nobody countersigned.
func TestCoSignReportsAnEscalatedClaimWithNoSecondSignature(t *testing.T) {
	t.Parallel()

	snap := adjudicated(&lint.Claim{
		ID: "c1",
		Warrant: gnosis.Warrant{
			By: "human:priya", Authority: "paired", Rationale: "chose the vendor limit",
		},
	})
	snap.Documents[0].Type = "Rule"
	snap.Vocabulary = normativeRule()

	got := runNamed(t, snap, "co-sign")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "co_signed_by") || !strings.Contains(got[0], "normative") {
		t.Errorf("the finding does not say what is required or why:\n%s", got[0])
	}
}

// TestCoSignAcceptsARecordedOverride. Recording the waiver is the whole mechanism: a
// gate with no override is one people route around, and a gate whose overrides are
// countable is still a gate.
func TestCoSignAcceptsARecordedOverride(t *testing.T) {
	t.Parallel()

	snap := adjudicated(&lint.Claim{
		ID: "c1",
		Warrant: gnosis.Warrant{
			By: "human:priya", Authority: "paired", Rationale: "chose the vendor limit",
			OverrideReason: "marcus on leave until 09-02; this blocks the writeup",
		},
	})
	snap.Documents[0].Type = "Rule"
	snap.Vocabulary = normativeRule()

	if got := runNamed(t, snap, "co-sign"); len(got) != 0 {
		t.Errorf("a recorded override was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestCoSignIsSilentAtSole. A single-curator corpus is a supported configuration:
// there is no second signer to require, so escalation records rather than blocks.
func TestCoSignIsSilentAtSole(t *testing.T) {
	t.Parallel()

	snap := adjudicated(&lint.Claim{
		ID: "c1",
		Warrant: gnosis.Warrant{
			By: "human:priya", Authority: "sole", Rationale: "chose the vendor limit",
		},
	})
	snap.Documents[0].Type = "Rule"
	snap.Vocabulary = normativeRule()

	if got := runNamed(t, snap, "co-sign"); len(got) != 0 {
		t.Errorf("sole authority demanded a co-signer:\n%s", strings.Join(got, "\n"))
	}
}

// TestCoSignIsSilentOnAnUnescalatedClaim. Neither normative nor load-bearing, so
// §10.6.1's ordinary path applies: anyone decides, and nobody countersigns.
func TestCoSignIsSilentOnAnUnescalatedClaim(t *testing.T) {
	t.Parallel()

	snap := adjudicated(&lint.Claim{
		ID: "c1",
		Warrant: gnosis.Warrant{
			By: "human:priya", Authority: "quorum", Rationale: "chose the vendor limit",
		},
	})

	if got := runNamed(t, snap, "co-sign"); len(got) != 0 {
		t.Errorf("an ordinary claim demanded a co-signer:\n%s", strings.Join(got, "\n"))
	}
}

// adjudicated builds a one-document corpus holding one claim.
func adjudicated(claim *lint.Claim) *lint.Snapshot {
	return &lint.Snapshot{
		InDegreeCut: 5,
		Documents: []lint.Document{{
			ID: outerID, Path: "c/a.md", Claims: []lint.Claim{*claim},
		}},
	}
}

// normativeRule declares one type, which prescribes.
func normativeRule() lint.Vocabulary {
	return lint.Vocabulary{
		Declared: true,
		Types:    []lint.VocabType{{Key: "Rule", Normative: true}},
	}
}
