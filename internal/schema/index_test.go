package schema_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/schema"
)

// body renders the one index region for comparison.
func body(t *testing.T, docs []schema.DocEntry) string {
	t.Helper()
	regions := schema.IndexRegions(docs)
	if len(regions) != 1 || regions[0].Name != schema.RegionIndex {
		t.Fatalf("want one index region, got %+v", regions)
	}
	return regions[0].Body
}

// TestAnUntypedDocumentStillGetsAHeading is the defect running the binary found, and it
// is the one an example-based test would have missed.
//
// The grouping compared each document's type against the previous one, starting from
// "". A document declaring no type has type "" too, so the comparison said "same group"
// and the first heading was never written — the untyped documents, which most need
// labelling because `conformance` reports them, were the ones rendered bare.
func TestAnUntypedDocumentStillGetsAHeading(t *testing.T) {
	t.Parallel()
	got := body(t, []schema.DocEntry{
		{Type: "", Title: "Nameless", Path: "c/a.md"},
		{Type: "Rule", Title: "Retry Budget", Path: "c/b.md"},
	})
	if !strings.Contains(got, "**(no type declared)**") {
		t.Errorf("the untyped group has no heading:\n%s", got)
	}
	if !strings.Contains(got, "**Rule**") {
		t.Errorf("the typed group has no heading:\n%s", got)
	}
	// And the untyped heading comes first, because "" sorts before every key — which
	// is the right order: a reader scanning for problems finds them at the top.
	if strings.Index(got, "(no type declared)") > strings.Index(got, "**Rule**") {
		t.Errorf("the untyped group is not first:\n%s", got)
	}
}

// TestAnEmptyCorpusSaysSoRatherThanRenderingNothing is §17's distinction applied to a
// generated region: an empty region reads as a generator that failed, and a suppressed
// one would make the first run after the first document rewrite a file the reader
// thought was stable.
func TestAnEmptyCorpusSaysSoRatherThanRenderingNothing(t *testing.T) {
	t.Parallel()
	got := body(t, nil)
	if strings.TrimSpace(got) == "" {
		t.Error("an empty corpus rendered an empty region")
	}
	if !strings.Contains(got, "no documents yet") {
		t.Errorf("the empty case does not say so: %q", got)
	}
}

// TestTheListingIsStableAcrossRuns is what `--check` rests on: a region that reordered
// itself between two runs over one corpus would report drift against itself.
func TestTheListingIsStableAcrossRuns(t *testing.T) {
	t.Parallel()
	docs := []schema.DocEntry{
		{Type: "Rule", Title: "Zebra", Path: "c/z.md"},
		{Type: "Reference", Title: "Alpha", Path: "c/a.md"},
		{Type: "Rule", Title: "Apple", Path: "c/ap.md"},
	}
	first := body(t, docs)

	// Reversed input, same output: the region sorts, so the caller's order is not
	// part of the contract.
	reversed := []schema.DocEntry{docs[2], docs[1], docs[0]}
	if second := body(t, reversed); second != first {
		t.Errorf("the listing depends on input order:\n%q\n%q", first, second)
	}
	// Within a type, by title.
	if strings.Index(first, "Apple") > strings.Index(first, "Zebra") {
		t.Errorf("entries are not sorted by title within a type:\n%s", first)
	}
}
