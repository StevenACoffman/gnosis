package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// covered builds a corpus of claims on one subject, some parsed and some not.
func covered(ops ...string) *lint.Snapshot {
	snap := &lint.Snapshot{Bounds: map[string]*lint.Bound{}}
	doc := lint.Document{Path: "c/a.md", Type: "Rule"}
	for i, op := range ops {
		id := string(rune('a' + i))
		doc.Claims = append(doc.Claims, lint.Claim{ID: id, Anchor: "An assertion."})
		b := lint.Bound{SubjectKey: "retry.max_attempts", Dimension: "count"}
		if op != "" {
			b.Op, b.Value = op, 3
		}
		snap.Bounds[id] = &b
	}
	snap.Documents = []lint.Document{doc}
	return snap
}

// TestAKeyWhoseClaimsAllParsedIsSilent is the adversarial case: this check exists to
// point at a gap in the pattern set, and a key with no gap is not a finding. Reporting
// every key would turn a backlog signal into a per-run tax.
func TestAKeyWhoseClaimsAllParsedIsSilent(t *testing.T) {
	t.Parallel()
	if got := runNamed(t, covered("<=", ">="), "constraint-coverage"); len(got) != 0 {
		t.Errorf("a fully-read key was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestAnUnreadClaimIsReportedWithBothCauses is §10.2.3's whole point: poor coverage has
// exactly two causes and the message must name both, because only a reader can tell them
// apart and the remedies are opposite — add a pattern, or accept that the claim states no
// quantity.
func TestAnUnreadClaimIsReportedWithBothCauses(t *testing.T) {
	t.Parallel()
	got := runNamed(t, covered("<=", ""), "constraint-coverage")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(got), got)
	}
	for _, want := range []string{
		"retry.max_attempts",
		"1 claim parsed to a value", "1 claim did not",
		"c/a.md b",
		"carry no quantity", "operators.toml misses",
	} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not carry %q:\n%s", want, got[0])
		}
	}
}

// TestNoRatioIsReported keeps the count from becoming a target. §17 forbids presenting a
// count as health, and a coverage percentage is the most target-shaped number available:
// it looks like progress and rises when somebody deletes the claims that do not parse.
func TestNoRatioIsReported(t *testing.T) {
	t.Parallel()
	got := runNamed(t, covered("<=", "", "", ""), "constraint-coverage")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d", len(got))
	}
	for _, forbidden := range []string{"%", "25", "coverage of", "0.25"} {
		if strings.Contains(got[0], forbidden) {
			t.Errorf("the finding carries a rate (%q):\n%s", forbidden, got[0])
		}
	}
}

// TestNoDeclaredSubjectSkipsRatherThanPasses is the absence-of-the-ruler case: a corpus
// whose claims name no declared subject has no key to report coverage for, and
// "coverage is fine" would be a statement about nothing.
func TestNoDeclaredSubjectSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Documents: []lint.Document{{
		Path: "c/a.md", Type: "Rule",
		Claims: []lint.Claim{{ID: "c1", Anchor: "An assertion."}},
	}}}
	reason := skipReason(t, snap, "constraint-coverage")
	if !strings.Contains(reason, "no key to report coverage for") {
		t.Errorf("the skip does not say why: %q", reason)
	}
}
