package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// twoVersions is a corpus where one source has been fetched twice.
func twoVersions() map[string]lint.SourceVersion {
	return map[string]lint.SourceVersion{
		"evidence/text/aa/old.txt": {URI: "https://x/doc", SHA256: "aaaaaaaaaaaa1111"},
		"evidence/text/bb/new.txt": {URI: "https://x/doc", SHA256: "bbbbbbbbbbbb2222"},
		"evidence/text/cc/oth.txt": {URI: "https://y/other", SHA256: "cccccccccccc3333"},
	}
}

// asserted builds a document asserting one claim on one archive path.
func asserted(path, claimID, anchor, archive string) lint.Document {
	return lint.Document{
		Path: path, Type: "Rule",
		Claims: []lint.Claim{{ID: claimID, Anchor: anchor, ArchivePaths: []string{archive}}},
	}
}

// TestTwoSitesOnOneVersionIsCorroborationNotDivergence is the adversarial case, and the
// one that decides whether the check is worth having.
//
// The same claim in two documents citing the *same* archived bytes says nothing is
// wrong — that is the corpus agreeing with itself. A predicate that reported it would
// fire on every well-maintained corpus and be switched off within a week.
func TestTwoSitesOnOneVersionIsCorroborationNotDivergence(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Sources: twoVersions(),
		Documents: []lint.Document{
			asserted("c/a.md", "c1", "Retries are capped at three.", "evidence/text/aa/old.txt"),
			asserted("c/b.md", "c2", "Retries are capped at three.", "evidence/text/aa/old.txt"),
		},
	}
	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("two claims on one version were reported as divergent:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestOneClaimOnTwoVersionsOfOneSourceDiverges is the predicate, and it is the only
// mechanical reading of "archived texts that disagree" that survives §10.3's refusal of
// a similarity threshold: byte identity.
func TestOneClaimOnTwoVersionsOfOneSourceDiverges(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Sources: twoVersions(),
		Documents: []lint.Document{
			asserted("c/a.md", "c1", "Retries are capped at three.", "evidence/text/aa/old.txt"),
			asserted("c/b.md", "c2", "Retries are capped at three.", "evidence/text/bb/new.txt"),
		},
	}
	got := runNamed(t, snap, "conflict")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	// §10.2.2: the finding shows its reasoning. Both versions, both documents, and
	// what to run next — a false conflict that shows its working is dismissible in
	// seconds and one that shows a verdict erodes the queue.
	for _, want := range []string{
		"2 versions", "https://x/doc", "aaaaaaaaaaaa", "bbbbbbbbbbbb",
		"c/a.md", "c/b.md", "fetch --recheck",
	} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not show %q:\n%s", want, got[0])
		}
	}
}

// TestDifferentSourcesAreNotDivergence keeps the comparison inside one source. Two
// documents making one claim from two different pages is corroboration across sources,
// which is what a corpus is for.
func TestDifferentSourcesAreNotDivergence(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Sources: twoVersions(),
		Documents: []lint.Document{
			asserted("c/a.md", "c1", "Retries are capped at three.", "evidence/text/aa/old.txt"),
			asserted("c/b.md", "c2", "Retries are capped at three.", "evidence/text/cc/oth.txt"),
		},
	}
	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("two sources were reported as one diverging:\n%s", strings.Join(got, "\n"))
	}
}

// TestDifferentClaimsOnTwoVersionsAreNotCompared keeps the predicate keyed on the claim.
// A source moving is ordinary; `audit --churn` reports it. What this check adds is that
// one *assertion* rests on both sides of the move.
func TestDifferentClaimsOnTwoVersionsAreNotCompared(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Sources: twoVersions(),
		Documents: []lint.Document{
			asserted("c/a.md", "c1", "Retries are capped at three.", "evidence/text/aa/old.txt"),
			asserted("c/b.md", "c2", "Backoff is exponential.", "evidence/text/bb/new.txt"),
		},
	}
	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("two unrelated claims were compared:\n%s", strings.Join(got, "\n"))
	}
}

// TestAReflowedClaimIsTheSameClaim keeps the grouping under fold, matching every other
// claim comparison in the corpus: a claim re-typed with different whitespace is the same
// claim, and treating it as two would hide the divergence this check exists to find.
func TestAReflowedClaimIsTheSameClaim(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Sources: twoVersions(),
		Documents: []lint.Document{
			asserted("c/a.md", "c1", "Retries are capped at three.", "evidence/text/aa/old.txt"),
			asserted("c/b.md", "c2", "Retries   are capped\nat three.", "evidence/text/bb/new.txt"),
		},
	}
	if got := runNamed(t, snap, "conflict"); len(got) != 1 {
		t.Errorf("a reflowed restatement was treated as a different claim: %v", got)
	}
}

// TestNothingToCompareSkipsRatherThanPasses is the absence-of-the-ruler case, and the
// skip has to name what *both* predicates would have needed.
//
// This check began with evidence divergence and gated on archived sources. Adding the
// interval predicate made that too narrow: a corpus stating two contradictory bounds and
// having fetched nothing would have been told there was nothing to examine — derived
// applicability drifting behind what the check does, which is §12's own warning pointed
// at itself.
func TestNothingToCompareSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Documents: []lint.Document{
		asserted("c/a.md", "c1", "Retries are capped.", "evidence/text/aa/old.txt"),
	}}
	reason := skipReason(t, snap, "conflict")
	for _, want := range []string{"source version", "parses to a bound"} {
		if !strings.Contains(reason, want) {
			t.Errorf("the skip does not name %q as missing: %q", want, reason)
		}
	}
}

// TestBoundsAloneAreEnoughToRun is the other half of that correction: a corpus with
// parsed constraints and no fetch records has something to compare.
func TestBoundsAloneAreEnoughToRun(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Bounds: map[string]*lint.Bound{
			"c1": {SubjectKey: "retry.max_attempts", Dimension: "count", Op: "<=", Value: 3},
		},
		Documents: []lint.Document{{
			Path: "c/a.md", Type: "Rule",
			Claims: []lint.Claim{{ID: "c1", Anchor: "Retries are at most three."}},
		}},
	}
	if got := runNamed(t, snap, "conflict"); len(got) != 0 {
		t.Errorf("one bound produced a conflict:\n%s", strings.Join(got, "\n"))
	}
}
