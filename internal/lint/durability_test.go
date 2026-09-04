package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// The three documents the fixtures below build a corpus from.
const (
	centralID  = gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-000000000001")
	outerID    = gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-000000000002")
	provableID = gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-000000000003")
)

// TestDurabilityReportsOnlyWhatTheCorpusLeansOn is the check's whole argument: an
// unprovable page nothing depends on was admitted deliberately (§4.3), and listing it
// would flood a reader with the cases the admission policy already accepted.
func TestDurabilityReportsOnlyWhatTheCorpusLeansOn(t *testing.T) {
	t.Parallel()

	snap := &lint.Snapshot{
		InDegreeCut: 2,
		Documents: []lint.Document{
			weakDoc(centralID, "c/central.md"),
			weakDoc(outerID, "c/outer.md"),
			provableDoc(provableID, "c/provable.md"),
		},
		// Two documents link to the central page and none to the outer one, so the
		// cut of two separates them.
		Links: []lint.Link{
			{FromID: outerID, ToID: centralID},
			{FromID: provableID, ToID: centralID},
		},
	}

	got := runNamed(t, snap, "durability")
	if len(got) != 2 {
		t.Fatalf("want one finding and one suppression line, got %d:\n%s",
			len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "c/central.md") ||
		!strings.Contains(got[0], "load-bearing-weak") {
		t.Errorf("the central page was not reported as load-bearing:\n%s", got[0])
	}
	if strings.Contains(strings.Join(got, "\n"), "c/outer.md") {
		t.Errorf("the peripheral page was listed rather than counted:\n%s",
			strings.Join(got, "\n"))
	}
	// §14.4.1's second guard: a check that silently drops most of its findings reads
	// as coverage, so the suppressed count is reported.
	if !strings.Contains(got[1], "durability-peripheral") ||
		!strings.Contains(got[1], "1 unprovable document") {
		t.Errorf("the suppressed count was not reported:\n%s", got[1])
	}
}

// TestDurabilityReportsProvableWorkRestingOnWeakGround. Off-centre and still reported,
// because a provable claim resting on unprovable ground is provable work with a hole
// under it.
func TestDurabilityReportsProvableWorkRestingOnWeakGround(t *testing.T) {
	t.Parallel()

	snap := &lint.Snapshot{
		InDegreeCut: 5,
		Documents: []lint.Document{
			weakDoc(outerID, "c/outer.md"),
			provableDoc(provableID, "c/provable.md"),
		},
		Links: []lint.Link{{FromID: provableID, ToID: outerID}},
	}

	got := runNamed(t, snap, "durability")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.Contains(got[0], "cited-by-provable") ||
		!strings.Contains(got[0], "c/outer.md") {
		t.Errorf("the cited page was not reported:\n%s", got[0])
	}
}

// TestDurabilityIsSilentOnAProvableCorpus. Nothing to report and no suppression line,
// because a "0 suppressed" line on every clean run is the kind a reader learns to skip.
func TestDurabilityIsSilentOnAProvableCorpus(t *testing.T) {
	t.Parallel()

	snap := &lint.Snapshot{
		InDegreeCut: 2,
		Documents: []lint.Document{
			provableDoc(provableID, "c/provable.md"),
			partlyProvableDoc(centralID, "c/mixed.md"),
		},
		Links: []lint.Link{{FromID: provableID, ToID: centralID}},
	}

	if got := runNamed(t, snap, "durability"); len(got) != 0 {
		t.Errorf("a corpus whose evidence is archived produced findings:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestDurabilitySkipsWithoutACut is the adversarial case, and the direction is what
// makes it worth a test: an in-degree of zero is at or above a cut of zero, so a
// corpus whose standards did not load would have **every** unprovable document
// reported as load-bearing — the loudest possible reading of a missing threshold.
func TestDurabilitySkipsWithoutACut(t *testing.T) {
	t.Parallel()

	snap := &lint.Snapshot{
		Documents: []lint.Document{weakDoc(outerID, "c/outer.md")},
	}

	reason := skipReason(t, snap, "durability")
	if !strings.Contains(reason, "in_degree_cut") {
		t.Errorf("the skip does not name the missing threshold: %q", reason)
	}
	// And it must not assert that the file declares none: an incomplete
	// standards/archive.toml is rejected whole, so a reader can be looking at the
	// value while this check sees zero.
	if strings.Contains(reason, "declares no in_degree_cut") {
		t.Errorf("the skip asserts the file declares none, which it cannot know: %q",
			reason)
	}
}

// TestDurabilitySkipsOnACorpusRestingOnNothing. Most Phase 2 documents are written by
// hand and cite no tier-0 record, and "durability is fine" would be a statement about
// nothing.
func TestDurabilitySkipsOnACorpusRestingOnNothing(t *testing.T) {
	t.Parallel()

	snap := &lint.Snapshot{
		InDegreeCut: 2,
		Documents:   []lint.Document{{ID: outerID, Path: "c/bare.md"}},
	}

	reason := skipReason(t, snap, "durability")
	if !strings.Contains(reason, "nothing to be provable against") {
		t.Errorf("the skip does not say what is missing: %q", reason)
	}
}

// weakDoc is a document whose every source is `referenced`.
func weakDoc(id gnosis.ID, path string) lint.Document {
	return lint.Document{ID: id, Path: path, Evidence: []lint.Evidence{
		{URI: "https://example.org/" + path + ".pdf", Support: gnosis.SupportWeak},
	}}
}

// provableDoc is a document whose every source is archived.
func provableDoc(id gnosis.ID, path string) lint.Document {
	return lint.Document{ID: id, Path: path, Evidence: []lint.Evidence{
		{URI: "https://example.org/" + path + ".md", Support: gnosis.SupportDurable},
	}}
}

// partlyProvableDoc rests on one of each, which is neither reported nor suppressed:
// §14.4 gives it its own state and §14.4.1 classifies only the unprovable.
func partlyProvableDoc(id gnosis.ID, path string) lint.Document {
	return lint.Document{ID: id, Path: path, Evidence: []lint.Evidence{
		{URI: "https://example.org/" + path + ".md", Support: gnosis.SupportDurable},
		{URI: "https://example.org/" + path + ".pdf", Support: gnosis.SupportWeak},
	}}
}
