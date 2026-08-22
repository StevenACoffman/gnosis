package index_test

import (
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/index"
)

// openSources opens a migrated, empty database in a temporary directory. A real
// SQLite file rather than a fake: the properties worth testing here are the
// primary key and the replace-in-one-transaction, and neither exists in a stub.
func openSources(t *testing.T) *index.DB {
	t.Helper()
	db, err := index.Open(t.Context(), filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func sourceRows() []index.SourceRow {
	return []index.SourceRow{
		{
			RecordSHA256: "rec-b", URI: "https://example.org/b.md",
			SourceSHA256: "src-b", ByteSize: 200, Disposition: "archived",
			ArchivePath: "evidence/text/bb/src-b.md",
		},
		{
			RecordSHA256: "rec-a", URI: "https://example.org/a.md",
			SourceSHA256: "src-a", ByteSize: 100, Disposition: "referenced",
			RejectReason: "extension-not-allowed",
		},
	}
}

// TestSourcesRoundTrip, including the fields that distinguish one disposition
// from another: an extracted record's provenance is the whole reason §4.2 records
// it, and a projection that dropped it would make a re-extraction invisible.
func TestSourcesRoundTrip(t *testing.T) {
	t.Parallel()
	db := openSources(t)

	rows := append(sourceRows(), index.SourceRow{
		RecordSHA256: "rec-c", URI: "https://example.org/c.pdf",
		SourceSHA256: "src-c", ByteSize: 900, Disposition: "extracted",
		ArchivePath: "evidence/text/cc/out.md", Extractor: "html-to-markdown",
		ExtractorVersion: "v2.5.2", ExtractedFrom: "src-c",
	})
	if err := db.ReplaceSources(t.Context(), rows); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := db.Sources(t.Context())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	// Ordered by URI, so two reads over one state are comparable.
	if got[0].URI != "https://example.org/a.md" || got[2].URI != "https://example.org/c.pdf" {
		t.Errorf("rows are not ordered by URI: %v, %v, %v",
			got[0].URI, got[1].URI, got[2].URI)
	}
	if got[2].Extractor != "html-to-markdown" || got[2].ExtractorVersion != "v2.5.2" {
		t.Errorf("extractor provenance was lost: %+v", got[2])
	}
	if got[0].RejectReason != "extension-not-allowed" {
		t.Errorf("a referenced row lost its reason: %+v", got[0])
	}
}

// TestTwoVersionsOfOneSourceBothSurvive. Keying on the URI would keep whichever
// was written last, which is exactly the history §4.1 preserves by appending a
// record per version rather than overwriting one.
func TestTwoVersionsOfOneSourceBothSurvive(t *testing.T) {
	t.Parallel()
	db := openSources(t)

	const uri = "https://example.org/a.md"
	err := db.ReplaceSources(t.Context(), []index.SourceRow{
		{RecordSHA256: "rec-1", URI: uri, SourceSHA256: "v1", Disposition: "archived"},
		{RecordSHA256: "rec-2", URI: uri, SourceSHA256: "v2", Disposition: "archived"},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := db.Sources(t.Context())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want both versions", len(got))
	}
}

// TestReplaceIsReplaceNotMerge. The table is derived, so a record deleted from
// tier 0 must disappear here; a merge would leave it as the only surviving trace
// of something the corpus no longer holds.
func TestReplaceIsReplaceNotMerge(t *testing.T) {
	t.Parallel()
	db := openSources(t)

	if err := db.ReplaceSources(t.Context(), sourceRows()); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	if err := db.ReplaceSources(t.Context(), sourceRows()[:1]); err != nil {
		t.Fatalf("second replace: %v", err)
	}

	got, err := db.Sources(t.Context())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d rows after replacing with one: %+v", len(got), got)
	}
}

// TestAnEmptyProjectionIsEmptyNotNil, so a caller need not distinguish "nothing
// fetched" from "no result" — and a corpus that has fetched nothing is the
// ordinary state of a fresh bundle.
func TestAnEmptyProjectionIsEmptyNotNil(t *testing.T) {
	t.Parallel()
	got, err := openSources(t).Sources(t.Context())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil {
		t.Error("an empty projection returned nil")
	}
	if len(got) != 0 {
		t.Errorf("a fresh index holds %d source rows", len(got))
	}
}

// TestReplacingWithNothingEmptiesTheTable, which is what a rebuild of a bundle
// whose archive was pruned should do.
func TestReplacingWithNothingEmptiesTheTable(t *testing.T) {
	t.Parallel()
	db := openSources(t)

	if err := db.ReplaceSources(t.Context(), sourceRows()); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := db.ReplaceSources(t.Context(), nil); err != nil {
		t.Fatalf("replace with nothing: %v", err)
	}

	got, err := db.Sources(t.Context())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d rows after replacing with nothing", len(got))
	}
}
