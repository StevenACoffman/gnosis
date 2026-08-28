package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// bounds builds a snapshot from readings alone, which is all this check reads.
func bounds(bs ...lint.Bound) *lint.Snapshot {
	snap := &lint.Snapshot{Bounds: map[string]*lint.Bound{}}
	for i := range bs {
		snap.Bounds[string(rune('a'+i))] = &bs[i]
	}
	return snap
}

// TestASubjectWrittenInAnotherDimensionIsReported is §5.8.2.1's silent drift, in the one
// shape a single snapshot can see: a key declared `count` whose claims say "400ms" is two
// groups using one word for two things.
func TestASubjectWrittenInAnotherDimensionIsReported(t *testing.T) {
	t.Parallel()
	snap := bounds(
		lint.Bound{
			SubjectKey: "retry.budget", Dimension: "count",
			Written: "duration", Op: "<=", Value: 400, Raw: "400ms",
		},
		lint.Bound{
			SubjectKey: "retry.budget", Dimension: "count",
			Written: "duration", Op: "<=", Value: 200, Raw: "200ms",
		},
	)
	got := runNamed(t, snap, "dimension-drift")
	if len(got) != 1 {
		t.Fatalf("want one finding per subject, got %d:\n%s",
			len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{"retry.budget", "count", "duration", "2 claims"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not mention %q:\n%s", want, got[0])
		}
	}
}

// TestABareNumberIsNotAMismatch is the adversarial case, and the one that would report
// most of a real corpus.
//
// "Retries must be no more than 3" writes no unit, and a bare number is what *every*
// dimension's value looks like when the author omitted it. Reading that as a count would
// manufacture a mismatch out of ordinary shorthand — on a `duration` subject, every
// unqualified claim.
func TestABareNumberIsNotAMismatch(t *testing.T) {
	t.Parallel()
	snap := bounds(
		lint.Bound{
			SubjectKey: "timeout", Dimension: "duration",
			Op: "<=", Value: 400, Raw: "400",
		},
	)
	if reason := skipReason(t, snap, "dimension-drift"); !strings.Contains(reason, "unit") {
		t.Errorf("dimension-drift skipped for %q, which does not say what is missing",
			reason)
	}
}

// TestAMatchingUnitIsSilent keeps the check from reporting the corpus that is right.
func TestAMatchingUnitIsSilent(t *testing.T) {
	t.Parallel()
	snap := bounds(
		lint.Bound{
			SubjectKey: "timeout", Dimension: "duration",
			Written: "duration", Op: "<=", Value: 400, Raw: "400ms",
		},
		lint.Bound{
			SubjectKey: "payload", Dimension: "bytes",
			Written: "bytes", Op: "<=", Value: 5, Raw: "5mb",
		},
	)
	if got := runNamed(t, snap, "dimension-drift"); len(got) != 0 {
		t.Errorf("a corpus writing the declared units was reported:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestOneFindingPerSubjectNotPerClaim is the noise rule. A subject whose meaning drifted
// has usually drifted across many claims, so per claim this would be loudest exactly on
// the corpus that has the problem worst — and the remedy is one edit either way.
func TestOneFindingPerSubjectNotPerClaim(t *testing.T) {
	t.Parallel()
	bs := make([]lint.Bound, 0, 6)
	for range 6 {
		bs = append(bs, lint.Bound{
			SubjectKey: "retry.budget", Dimension: "count",
			Written: "duration", Op: "<=", Value: 400, Raw: "400ms",
		})
	}
	got := runNamed(t, bounds(bs...), "dimension-drift")
	if len(got) != 1 {
		t.Errorf("six drifted claims produced %d findings, want 1", len(got))
	}
}
