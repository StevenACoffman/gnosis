package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// strengths is the marker set the shipped seed produces, in miniature.
func strengths() lint.Strengths {
	return lint.Strengths{
		Universal: []string{"must not", "guarantees", "always", "never", "every", "must"},
		Hedged:    []string{"in most cases", "typically", "usually", "may"},
	}
}

// weighed builds a corpus with one claim of type Rule.
//
// The type is fixed because it is not what varies here — `normative` is, and it is a
// property of the declaration rather than of the key. A parameter every caller passed
// "Rule" to claimed a choice nobody was making.
func weighed(normative bool, anchor string, quotes ...string) *lint.Snapshot {
	return &lint.Snapshot{
		Strength: strengths(),
		Vocabulary: lint.Vocabulary{
			Declared: true,
			Types:    []lint.VocabType{{Key: "Rule", Normative: normative}},
		},
		Documents: []lint.Document{{
			Path: "c/a.md", Type: "Rule",
			Claims: []lint.Claim{{ID: "c1", Anchor: anchor, Quotes: quotes}},
		}},
	}
}

// TestAHedgedClaimIsSilentEvenOnOneQuotation is the adversarial case, and the one that
// would make the check counter-productive.
//
// §17.3.1's remedy is usually to weaken the claim. A claim that already says "typically"
// has done exactly that — reporting it anyway would teach authors that hedging buys
// nothing, which is the opposite of what the check is for.
func TestAHedgedClaimIsSilentEvenOnOneQuotation(t *testing.T) {
	t.Parallel()
	snap := weighed(false,
		"Retries are typically capped at three.", "retries are capped at three")
	if got := runNamed(t, snap, "coverage"); len(got) != 0 {
		t.Errorf("a hedged claim was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestAUniversalClaimOnOneQuotationIsReported is §17.3.1's own example: the evidence did
// not change, the claim did.
func TestAUniversalClaimOnOneQuotationIsReported(t *testing.T) {
	t.Parallel()
	snap := weighed(false,
		"Retries are always capped at three.", "retries are capped at three")
	got := runNamed(t, snap, "coverage")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{"always", "1 quotation", "weaken the wording"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not mention %q:\n%s", want, got[0])
		}
	}
}

// TestBothAUniversalAndAHedgeIsSilence is the boundary the two lists exist to create. A
// claim carrying each is a reading no lexical check can settle, and saying nothing is
// the honest answer rather than picking whichever list is consulted first.
func TestBothAUniversalAndAHedgeIsSilence(t *testing.T) {
	t.Parallel()
	snap := weighed(false,
		"Retries must typically be capped at three.", "retries are capped at three")
	if got := runNamed(t, snap, "coverage"); len(got) != 0 {
		t.Errorf("a claim that qualified itself was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestTheStakesAreInTheMessageAndNotInTheThreshold is §14.4.1 expressed where it can be
// acted on, and the correction of a number I invented.
//
// The first version raised the bar by one for a normative type, which silently required
// *three* quotations — a figure §17.3.1 never states and §6.2 forbids inventing. There is
// one comparison in that section and this check implements exactly it. The stakes belong
// in the message, where a reader triaging a queue can use them.
func TestTheStakesAreInTheMessageAndNotInTheThreshold(t *testing.T) {
	t.Parallel()

	two := []string{"retries are capped at three", "the cap is three attempts"}
	if got := runNamed(t, weighed(true,
		"Retries are always capped.", two...), "coverage"); len(got) != 0 {
		t.Errorf("a normative claim on two quotations was reported, which needs a "+
			"threshold nobody stated:\n%s", strings.Join(got, "\n"))
	}

	one := runNamed(t, weighed(true, "Retries are always capped.", two[0]),
		"coverage")
	if len(one) != 1 {
		t.Fatalf("a normative universal on one quotation was not reported: %v", one)
	}
	if !strings.Contains(one[0], "prescribes") {
		t.Errorf("the finding does not mark the stakes for triage:\n%s", one[0])
	}

	plain := runNamed(t, weighed(false, "Retries are always capped.", two[0]),
		"coverage")
	if len(plain) != 1 {
		t.Fatalf("a non-normative universal on one quotation was not reported: %v", plain)
	}
	if strings.Contains(plain[0], "prescribes") {
		t.Errorf("a non-normative claim was marked as prescribing:\n%s", plain[0])
	}
}

// TestAWordBoundaryIsRequired keeps "allow" from matching "all" and "mustard" from
// matching "must".
func TestAWordBoundaryIsRequired(t *testing.T) {
	t.Parallel()
	snap := weighed(false,
		"Retries allow mustard on the request.", "retries are capped at three")
	if got := runNamed(t, snap, "coverage"); len(got) != 0 {
		t.Errorf("a word merely containing a marker was reported:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestTheLongestMarkerWins keeps "must not" from being reported as "must" with the
// negation dropped — a finding that quoted the wrong word back would read as a bug.
func TestTheLongestMarkerWins(t *testing.T) {
	t.Parallel()
	snap := weighed(false,
		"Retries must not exceed three.", "retries are capped at three")
	got := runNamed(t, snap, "coverage")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %v", got)
	}
	if !strings.Contains(got[0], "must not") {
		t.Errorf("the finding quoted the wrong marker:\n%s", got[0])
	}
}

// TestAClaimWithNoEvidenceIsNotThisChecksFinding keeps the report about thin evidence
// rather than absent evidence. A claim offering none has a conformance problem, and
// saying its evidence is thin would misdescribe it.
func TestAClaimWithNoEvidenceIsNotThisChecksFinding(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Strength: strengths(),
		Vocabulary: lint.Vocabulary{
			Declared: true, Types: []lint.VocabType{{Key: "Rule"}},
		},
		Documents: []lint.Document{
			{Path: "c/a.md", Type: "Rule", Claims: []lint.Claim{
				{ID: "c1", Anchor: "Retries are always capped."},
			}},
			// One claim with evidence, so the check applies at all.
			{Path: "c/b.md", Type: "Rule", Claims: []lint.Claim{
				{ID: "c2", Anchor: "Something.", Quotes: []string{"something"}},
			}},
		},
	}
	for _, g := range runNamed(t, snap, "coverage") {
		if strings.Contains(g, "c1") {
			t.Errorf("a claim with no evidence was reported for thin evidence:\n%s", g)
		}
	}
}

// TestNoMarkersSkipsRatherThanPasses is the absence-of-the-ruler case: a corpus whose
// strength file did not load must be told so, not told it is clean.
func TestNoMarkersSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Documents: []lint.Document{{Path: "c/a.md", Type: "Rule", Claims: []lint.Claim{
			{ID: "c1", Anchor: "Retries are always capped.", Quotes: []string{"x"}},
		}}},
	}
	if reason := skipReason(t, snap, "coverage"); !strings.Contains(reason, "strength.toml") {
		t.Errorf("the skip does not name the missing file: %q", reason)
	}
}
