package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// withClaims builds a snapshot holding one document whose claims name paths, and
// an archive holding the paths listed as present.
func withClaims(claims []lint.Claim, present ...string) *lint.Snapshot {
	archived := make(map[string]bool, len(present))
	for _, p := range present {
		archived[p] = true
	}
	return &lint.Snapshot{
		Documents:    []lint.Document{{Path: "c/a.md", Claims: claims}},
		ArchivedText: archived,
	}
}

// only runs the archive-path check alone and returns its diagnostics, so a
// failure names this check rather than whatever else the registry reported.
func only(t *testing.T, snap *lint.Snapshot) lint.Report {
	t.Helper()
	for _, c := range lint.Checks() {
		if c.Name == "archive-path" {
			return lint.Run(snap, []lint.Check{c})
		}
	}
	t.Fatal("no archive-path check is registered")
	return lint.Report{}
}

// TestADanglingPathIsReported is the case: a claim says where its evidence lives
// and the file is not there, so the claim can be neither verified nor refuted.
func TestADanglingPathIsReported(t *testing.T) {
	t.Parallel()
	snap := withClaims([]lint.Claim{
		{ID: "claim-1", ArchivePaths: []string{"evidence/text/ab/gone.md"}},
	})

	got := only(t, snap)
	if len(got.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(got.Diagnostics), got.Diagnostics)
	}
	d := got.Diagnostics[0]
	if d.Path != "c/a.md" {
		t.Errorf("path = %q, want the document", d.Path)
	}
	if !strings.Contains(d.Message, "claim-1") {
		t.Errorf("the message does not name the claim: %q", d.Message)
	}
	if !strings.Contains(d.Message, "gone.md") {
		t.Errorf("the message does not name the missing file: %q", d.Message)
	}
	// The consequence, not just the fact. A reader has to know why it matters.
	if !strings.Contains(d.Message, "verified or refuted") {
		t.Errorf("the message does not say what the corpus lost: %q", d.Message)
	}
}

// TestAResolvedPathIsSilent, or the check reports every claim in a healthy corpus.
func TestAResolvedPathIsSilent(t *testing.T) {
	t.Parallel()
	const path = "evidence/text/ab/abcd.md"
	snap := withClaims([]lint.Claim{
		{ID: "claim-1", ArchivePaths: []string{path}},
	}, path)

	if got := only(t, snap); len(got.Diagnostics) != 0 {
		t.Errorf("a resolved path was reported: %+v", got.Diagnostics)
	}
}

// TestOneFindingPerClaim, not per path. A claim citing three files from a source
// that was pruned has one problem, and three findings would make the report about
// the paths instead of about the claim that lost its evidence.
func TestOneFindingPerClaim(t *testing.T) {
	t.Parallel()
	snap := withClaims([]lint.Claim{{
		ID: "claim-1",
		ArchivePaths: []string{
			"evidence/text/aa/one.md",
			"evidence/text/bb/two.md",
			"evidence/text/cc/three.md",
		},
	}})

	got := only(t, snap)
	if len(got.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1 for one claim", len(got.Diagnostics))
	}
	if !strings.Contains(got.Diagnostics[0].Message, "3 archived file") {
		t.Errorf("the message does not count them: %q", got.Diagnostics[0].Message)
	}
}

// TestAPartiallyResolvedClaimReportsOnlyWhatIsMissing: naming a file that is
// present would send a reader to check something that is fine, and the point of
// the message is to be actionable without a second lookup.
func TestAPartiallyResolvedClaimReportsOnlyWhatIsMissing(t *testing.T) {
	t.Parallel()
	const here = "evidence/text/aa/here.md"
	snap := withClaims([]lint.Claim{{
		ID:           "claim-1",
		ArchivePaths: []string{here, "evidence/text/bb/gone.md"},
	}}, here)

	got := only(t, snap)
	if len(got.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(got.Diagnostics))
	}
	msg := got.Diagnostics[0].Message
	if strings.Contains(msg, "here.md") {
		t.Errorf("the message names a file that is present: %q", msg)
	}
	if !strings.Contains(msg, "gone.md") {
		t.Errorf("the message omits the missing file: %q", msg)
	}
}

// TestManyMissingPathsAreBounded, so one badly-broken document cannot make the
// report unreadable.
func TestManyMissingPathsAreBounded(t *testing.T) {
	t.Parallel()
	paths := make([]string, 0, 9)
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		paths = append(paths, "evidence/text/"+n+n+"/"+n+".md")
	}
	snap := withClaims([]lint.Claim{{ID: "claim-1", ArchivePaths: paths}})

	msg := only(t, snap).Diagnostics[0].Message
	if !strings.Contains(msg, "and 6 more") {
		t.Errorf("the list is not bounded: %q", msg)
	}
}

// TestTheCheckSkipsACorpusWithNoClaims. Derived applicability, per §12: reporting
// nothing found is different from reporting there was nothing to look for, and a
// check that ran on every Phase-1 corpus would be a check people learn to ignore.
func TestTheCheckSkipsACorpusWithNoClaims(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Documents: []lint.Document{{Path: "c/a.md"}}}

	got := only(t, snap)
	if len(got.Skipped) != 1 {
		t.Fatalf("the check ran on a corpus with no claims: %+v", got)
	}
	if !strings.Contains(got.Skipped[0].Reason, "no document declares claims") {
		t.Errorf("the skip reason is unhelpful: %q", got.Skipped[0].Reason)
	}
}

// TestAClaimNamingNoPathIsNotDangling. A claim with no archive path has no
// address to dangle; whether it should have one is the gate's question.
func TestAClaimNamingNoPathIsNotDangling(t *testing.T) {
	t.Parallel()
	snap := withClaims([]lint.Claim{{ID: "claim-1"}})

	if got := only(t, snap); len(got.Diagnostics) != 0 {
		t.Errorf("a claim with no paths was reported: %+v", got.Diagnostics)
	}
}
