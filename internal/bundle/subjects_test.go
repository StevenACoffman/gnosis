package bundle_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// population is a corpus with one declared subject reached by two spellings.
func population(docs ...lint.Document) *lint.Snapshot {
	return &lint.Snapshot{
		Documents: docs,
		Vocabulary: lint.Vocabulary{
			Declared: true,
			SubjectOf: map[gnosis.Surface]gnosis.SubjectKey{
				"retry.max_attempts": "retry.max_attempts",
				"retry budget":       "retry.max_attempts",
				"request.timeout":    "request.timeout",
			},
		},
	}
}

// TestEvidenceNobodyCitedIsNotDisjoint is the adversarial case, and it is the one
// that would make this instrument useless rather than merely wrong.
//
// Two empty evidence sets are disjoint by the letter of the definition. A fold that
// took that literally would report the trigger condition true for every subject in
// every hand-written corpus — where no document cites anything — and the signal would
// fire everywhere on day one, which is indistinguishable from firing nowhere.
func TestEvidenceNobodyCitedIsNotDisjoint(t *testing.T) {
	t.Parallel()
	snap := population(
		lint.Document{Path: "c/a.md", Claims: []lint.Claim{
			{ID: "a1", Subject: "retry budget"},
		}},
		lint.Document{Path: "c/b.md", Claims: []lint.Claim{
			{ID: "b1", Subject: "retry.max_attempts"},
		}},
	)
	got := bundle.Subjects(snap)
	if len(got.Subjects) != 1 {
		t.Fatalf("want one subject, got %d", len(got.Subjects))
	}
	if got.Subjects[0].DisjointEvidence {
		t.Error("two documents citing nothing were reported as disjoint evidence;" +
			" that is true of every hand-written corpus and so says nothing")
	}
}

// TestDisjointEvidenceIsTheRecordedTrigger is the condition §5.8.2.1's detector waits
// for, observable with no threshold: two documents about one key, neither reading
// anything the other read.
func TestDisjointEvidenceIsTheRecordedTrigger(t *testing.T) {
	t.Parallel()

	shared := population(
		lint.Document{Path: "c/a.md", Claims: []lint.Claim{
			{ID: "a1", Subject: "retry budget", ArchivePaths: []string{"e/one.md"}},
		}},
		lint.Document{Path: "c/b.md", Claims: []lint.Claim{
			{ID: "b1", Subject: "retry budget", ArchivePaths: []string{"e/one.md", "e/two.md"}},
		}},
	)
	if bundle.Subjects(shared).Subjects[0].DisjointEvidence {
		t.Error("documents sharing a source were reported disjoint")
	}

	apart := population(
		lint.Document{Path: "c/a.md", Claims: []lint.Claim{
			{ID: "a1", Subject: "retry budget", ArchivePaths: []string{"e/one.md"}},
		}},
		lint.Document{Path: "c/b.md", Claims: []lint.Claim{
			{ID: "b1", Subject: "retry budget", ArchivePaths: []string{"e/two.md"}},
		}},
	)
	if !bundle.Subjects(apart).Subjects[0].DisjointEvidence {
		t.Error("two documents reading nothing in common were not reported")
	}
}

// TestSurfacesRecordWhatAuthorsActuallyWrote is the "cluster of new aliases" signal in
// its observable form. The key is one thing; four spellings of it is the fact a
// detector would later be calibrated against.
func TestSurfacesRecordWhatAuthorsActuallyWrote(t *testing.T) {
	t.Parallel()
	snap := population(
		lint.Document{Path: "c/a.md", Claims: []lint.Claim{
			{ID: "a1", Subject: "retry budget"},
			{ID: "a2", Subject: "retry.max_attempts"},
			{ID: "a3", Subject: "retry budget"},
		}},
	)
	got := bundle.Subjects(snap).Subjects[0]
	if got.Claims != 3 || got.Documents != 1 {
		t.Errorf("claims = %d and documents = %d, want 3 and 1", got.Claims, got.Documents)
	}
	// Distinct, sorted, and not one entry per claim.
	want := []string{"retry budget", "retry.max_attempts"}
	if len(got.Surfaces) != len(want) {
		t.Fatalf("surfaces = %v, want %v", got.Surfaces, want)
	}
	for i := range want {
		if got.Surfaces[i] != want[i] {
			t.Errorf("surfaces = %v, want %v", got.Surfaces, want)
		}
	}
}

// TestAnUnresolvedSubjectIsCountedAndNotFiledUnderAKey keeps the totals honest: a
// phrase resolving to nothing belongs to no key, and quietly dropping it would make
// the population look complete.
func TestAnUnresolvedSubjectIsCountedAndNotFiledUnderAKey(t *testing.T) {
	t.Parallel()
	snap := population(
		lint.Document{Path: "c/a.md", Claims: []lint.Claim{
			{ID: "a1", Subject: "retru budget"},
			{ID: "a2", Subject: "retry budget"},
		}},
	)
	got := bundle.Subjects(snap)
	if got.Unresolved != 1 {
		t.Errorf("unresolved = %d, want 1", got.Unresolved)
	}
	if len(got.Subjects) != 1 || got.Subjects[0].Claims != 1 {
		t.Errorf("the unresolved claim was filed under a key: %+v", got.Subjects)
	}
	// request.timeout is declared and unclaimed; retry.max_attempts is claimed.
	if got.Undeclared != 1 {
		t.Errorf("undeclared keys = %d, want 1", got.Undeclared)
	}
}

// TestAnEmptyCorpusReportsNothingRatherThanZeroes is §17's distinction, and the
// reason Any() exists beside the counts.
func TestAnEmptyCorpusReportsNothingRatherThanZeroes(t *testing.T) {
	t.Parallel()
	if bundle.Subjects(&lint.Snapshot{}).Any() {
		t.Error("an empty corpus reported a population")
	}
}
