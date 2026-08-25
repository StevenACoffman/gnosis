package bundle_test

import (
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// landed is a successful row of the given operation.
func landed(op audit.Op, at time.Time, paths ...string) audit.Row {
	return audit.Row{
		At: at, Op: op, Actor: "human:priya",
		Paths: paths, Outcome: string(gnosis.StatusOK),
	}
}

// TestOnlyWhatLandedCounts is the dynamic this report must not invert.
//
// Hamming's argument is that a corpus showing only problems rewards contributing less.
// Counting *attempts* would correct that by rewarding trying, which is the same mistake
// with the sign flipped: a refused promotion added no document.
func TestOnlyWhatLandedCounts(t *testing.T) {
	t.Parallel()

	refusedPromote := audit.Row{
		At: noon, Op: audit.OpPromote, Actor: "human:priya",
		Paths: []string{"c/a.md"}, Outcome: string(gnosis.StatusBlocked),
	}
	got := bundle.Gained(&bundle.Trail{Rows: []audit.Row{
		landed(audit.OpPromote, noon, "c/a.md"),
		refusedPromote,
	}}, time.Time{})

	if got.Promoted != 1 {
		t.Errorf("promoted = %d, want 1", got.Promoted)
	}
}

// TestEveryKindOfGainIsCounted, including the one somebody will argue about.
func TestEveryKindOfGainIsCounted(t *testing.T) {
	t.Parallel()

	got := bundle.Gained(&bundle.Trail{Rows: []audit.Row{
		landed(audit.OpPromote, noon, "c/a.md"),
		landed(audit.OpAdmit, noon, "c/b.md"),
		// One fetch row naming three sources: a fetch of three is one row and three
		// gains, because Paths is what reached the disk.
		landed(audit.OpFetch, noon, "e/1.json", "e/2.json", "e/3.json"),
		// A declined draft is a gain: the corpus now holds a judgement it did not
		// hold before, and counting only additions would make deciding-against
		// invisible.
		landed(audit.OpDiscard, noon, "c/c.md"),
		// Scaffolding and cache rebuilds are not gains: the corpus knows nothing new.
		landed(audit.OpInit, noon, "index.md"),
		landed(audit.OpRebuild, noon, ".gnosis/index.db"),
	}}, time.Time{})

	switch {
	case got.Promoted != 1:
		t.Errorf("promoted = %d, want 1", got.Promoted)
	case got.Admitted != 1:
		t.Errorf("admitted = %d, want 1", got.Admitted)
	case got.Archived != 3:
		t.Errorf("archived = %d, want 3", got.Archived)
	case got.Declined != 1:
		t.Errorf("declined = %d, want 1", got.Declined)
	}
	if !got.Any() {
		t.Error("a corpus that gained four things reports nothing")
	}
}

// TestTheWindowIsRespected keeps the report from becoming a number that only grows.
//
// A total since the beginning says nothing — it cannot go down and it cannot be
// compared — so the window is the whole of what makes the count interpretable.
func TestTheWindowIsRespected(t *testing.T) {
	t.Parallel()

	old := noon.AddDate(0, 0, -30)
	got := bundle.Gained(&bundle.Trail{Rows: []audit.Row{
		landed(audit.OpPromote, old, "c/old.md"),
		landed(audit.OpPromote, noon, "c/new.md"),
	}}, noon.AddDate(0, 0, -1))

	if got.Promoted != 1 {
		t.Errorf("promoted = %d, want 1 — the window was not applied", got.Promoted)
	}
	// And the window travels with the answer, because a count with no period is
	// uninterpretable.
	if got.Since.IsZero() {
		t.Error("the report does not say what period it covers")
	}
}

// TestNothingGainedIsNotNothingLookedAt is the distinction every state in this codebase
// keeps: "we gained nothing this week" and "we did not look" must not render alike.
func TestNothingGainedIsNotNothingLookedAt(t *testing.T) {
	t.Parallel()

	got := bundle.Gained(&bundle.Trail{}, noon)
	if got.Any() {
		t.Error("an empty trail reported gains")
	}
	if !got.Complete() {
		t.Error("an intact empty trail reported an incomplete count")
	}

	damaged := bundle.Gained(&bundle.Trail{Malformed: []int{3}}, noon)
	if damaged.Complete() {
		t.Error("a damaged trail reported totals rather than floors")
	}
}
