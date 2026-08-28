package bundle_test

import (
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// window is the report's boundary in these cases.
func window() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) }

// TestAClaimWithNoLeadIsNotReportedUnreached is the adversarial case.
//
// A claim with no lead is not in `claims_fts` (§5.5.3), so `search --claims` cannot
// return it at any ranking. Listing it here would blame a claim for a gap in extraction —
// and on a corpus mid-extraction that is most of them, which would make this report a
// second, worse rendering of the shortfall `search --claims` already prints.
func TestAClaimWithNoLeadIsNotReportedUnreached(t *testing.T) {
	t.Parallel()

	claims := []bundle.QuietClaim{
		{ClaimID: "c1", Path: "c/a.md", Lead: "cap retries at three"},
		{ClaimID: "c2", Path: "c/a.md"},
	}
	got := bundle.Unreached(claims, map[string]bundle.Retrieval{}, window())
	if got.Claims != 1 {
		t.Errorf("Claims = %d, want 1; a claim with no lead is not retrievable",
			got.Claims)
	}
	if len(got.Quiet) != 1 || got.Quiet[0].ClaimID != "c1" {
		t.Errorf("Quiet = %+v, want only c1", got.Quiet)
	}
}

// TestARetrievalBeforeTheWindowDoesNotCount is what makes this a report over a period
// rather than a permanent record of first contact. A claim returned once a year ago and
// never since is exactly what the report exists to surface.
func TestARetrievalBeforeTheWindowDoesNotCount(t *testing.T) {
	t.Parallel()

	claims := []bundle.QuietClaim{{ClaimID: "c1", Path: "c/a.md", Lead: "a lead"}}
	log := map[string]bundle.Retrieval{
		"c1": {ClaimID: "c1", Returns: 4, LastAt: window().AddDate(0, 0, -1)},
	}
	got := bundle.Unreached(claims, log, window())
	if got.Observed != 0 || len(got.Quiet) != 1 {
		t.Errorf("a retrieval older than the window counted: observed=%d quiet=%d",
			got.Observed, len(got.Quiet))
	}
}

// TestARetrievalOnTheBoundaryCounts pins the inclusive edge, so a report run with
// --since set to the day of a search does not disown it.
func TestARetrievalOnTheBoundaryCounts(t *testing.T) {
	t.Parallel()

	claims := []bundle.QuietClaim{{ClaimID: "c1", Path: "c/a.md", Lead: "a lead"}}
	log := map[string]bundle.Retrieval{
		"c1": {ClaimID: "c1", Returns: 1, LastAt: window()},
	}
	got := bundle.Unreached(claims, log, window())
	if got.Observed != 1 || len(got.Quiet) != 0 {
		t.Errorf("a retrieval exactly at the window was excluded: observed=%d quiet=%d",
			got.Observed, len(got.Quiet))
	}
}

// TestTheReportIsOrderedForRepeatedRuns keeps two runs over one state from producing
// different bytes, which is what makes a report diffable.
func TestTheReportIsOrderedForRepeatedRuns(t *testing.T) {
	t.Parallel()

	claims := []bundle.QuietClaim{
		{ClaimID: "c9", Path: "c/z.md", Lead: "z"},
		{ClaimID: "c2", Path: "c/a.md", Lead: "a2"},
		{ClaimID: "c1", Path: "c/a.md", Lead: "a1"},
	}
	got := bundle.Unreached(claims, map[string]bundle.Retrieval{}, window())
	want := []string{"c1", "c2", "c9"}
	for i, q := range got.Quiet {
		if q.ClaimID != want[i] {
			t.Errorf("Quiet[%d] = %s, want %s", i, q.ClaimID, want[i])
		}
	}
}
