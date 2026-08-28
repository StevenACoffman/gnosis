package bundle_test

import (
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// TestRecordingIsCumulativeAndIdempotentInShape is the round trip: two searches
// returning one claim leave one row with a count of two, not two rows.
//
// Upsert rather than append, for `RecordChecks`'s reason — nothing consumes the sequence,
// and an append-only log here would grow without bound in a file whose only reader wants
// the latest.
func TestRecordingIsCumulativeAndIdempotentInShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	if err := bundle.RecordRetrievals(dir, first, []string{"c1", "c2"}); err != nil {
		t.Fatalf("first record: %v", err)
	}
	if err := bundle.RecordRetrievals(dir, second, []string{"c1"}); err != nil {
		t.Fatalf("second record: %v", err)
	}

	log, err := bundle.LoadRetrievals(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(log) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(log), log)
	}
	if log["c1"].Returns != 2 {
		t.Errorf("c1 returns = %d, want 2", log["c1"].Returns)
	}
	if !log["c1"].LastAt.Equal(second) {
		t.Errorf("c1 last_at = %v, want %v", log["c1"].LastAt, second)
	}
	if log["c2"].Returns != 1 {
		t.Errorf("c2 returns = %d, want 1", log["c2"].Returns)
	}
}

// TestAnAbsentLogIsEmptyRatherThanAnError is the day-one case, and the one that decides
// whether the report can skip cleanly: a corpus nobody has searched must be
// distinguishable from one whose log will not open.
func TestAnAbsentLogIsEmptyRatherThanAnError(t *testing.T) {
	t.Parallel()

	log, err := bundle.LoadRetrievals(t.TempDir())
	if err != nil {
		t.Fatalf("an absent log is an error: %v", err)
	}
	if len(log) != 0 {
		t.Errorf("an absent log yielded %d rows", len(log))
	}
}

// TestRecordingNothingWritesNothing keeps a search that matched nothing from creating a
// state file. The file's existence is what `audit --unretrieved` reads as "somebody has
// searched", and a query returning no hits has not established that anything is reachable.
func TestRecordingNothingWritesNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := bundle.RecordRetrievals(dir, time.Now(), nil); err != nil {
		t.Fatalf("record nothing: %v", err)
	}
	log, err := bundle.LoadRetrievals(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(log) != 0 {
		t.Errorf("an empty search wrote %d rows", len(log))
	}
}
