package bundle_test

import (
	"testing"
	"testing/fstest"

	"github.com/StevenACoffman/gnosis/internal/bundle"
)

const validID = "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"

// concept renders a minimal document. fstest.MapFS is used rather than a real
// temporary directory: Load takes an fs.FS precisely so the walk can be
// exercised without touching disk, and a map is both faster and easier to read
// than a fixture tree.
func concept(id, title string) *fstest.MapFile {
	body := "---\ntype: Reference\n"
	if id != "" {
		body += "gnosis_id: " + id + "\n"
	}
	if title != "" {
		body += "title: " + title + "\n"
	}
	return &fstest.MapFile{Data: []byte(body + "---\nbody\n")}
}

func TestLoadReadsConcepts(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"c/" + validID + "-alpha.md": concept(validID, "Alpha"),
	}
	docs, err := bundle.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1", len(docs))
	}
	d := docs[0]
	switch {
	case d.ID.String() != validID:
		t.Errorf("ID = %q, want %q", d.ID, validID)
	case d.Type != "Reference":
		t.Errorf("Type = %q, want Reference", d.Type)
	case d.Title != "Alpha":
		t.Errorf("Title = %q, want Alpha", d.Title)
	case d.Hash == "":
		t.Error("Hash is empty; the content hash drives change detection")
	case d.Invalid != nil:
		t.Errorf("Invalid = %v, want nil", d.Invalid)
	}
}

// TestAbsentConceptDirectoryIsAnEmptyCorpus: a freshly initialised bundle has no
// c/ yet, and every command must work against it. Treating that as an error
// would make `init` produce something the next command rejects.
func TestAbsentConceptDirectoryIsAnEmptyCorpus(t *testing.T) {
	t.Parallel()
	docs, err := bundle.Load(fstest.MapFS{"index.md": &fstest.MapFile{}})
	if err != nil {
		t.Fatalf("Load on a bundle with no c/: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("got %d documents, want 0", len(docs))
	}
}

// TestOneBadDocumentDoesNotHideTheRest is the property that keeps a corpus
// readable. A malformed file is reported on its own Document, so the caller can
// diagnose it while still seeing everything else.
func TestOneBadDocumentDoesNotHideTheRest(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"c/" + validID + "-good.md": concept(validID, "Good"),
		"c/broken.md":               &fstest.MapFile{Data: []byte("no frontmatter here\n")},
	}
	docs, err := bundle.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("got %d documents, want both", len(docs))
	}

	var good, bad int
	for _, d := range docs {
		if d.Invalid == nil {
			good++
			continue
		}
		bad++
	}
	if good != 1 || bad != 1 {
		t.Errorf("got %d valid and %d invalid, want 1 and 1", good, bad)
	}
}

// TestReservedFilenamesAreNotConcepts: OKF §3.1 gives index.md and log.md a
// defined meaning, so a walk must not read them as claims.
func TestReservedFilenamesAreNotConcepts(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"c/index.md":                &fstest.MapFile{Data: []byte("# Contents\n")},
		"c/log.md":                  &fstest.MapFile{Data: []byte("## 2026-08-20\n")},
		"c/" + validID + "-real.md": concept(validID, "Real"),
	}
	docs, err := bundle.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want only the concept: %+v", len(docs), docs)
	}
}

// TestMalformedIdentifierIsReportedNotAdopted: an identifier gnosis cannot parse
// must not be carried forward as if valid, because the whole redundancy scheme
// depends on one spelling per identifier.
func TestMalformedIdentifierIsReportedNotAdopted(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{"c/bad-id.md": concept("not-a-uuid", "Bad")}
	docs, err := bundle.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1", len(docs))
	}
	if docs[0].ID != "" {
		t.Errorf("ID = %q; a malformed identifier must not be adopted", docs[0].ID)
	}
	if docs[0].Invalid == nil {
		t.Error("Invalid is nil; a malformed identifier must be reported")
	}
}

// TestObservedPreservesAnEmptyIdentifier: Reconcile reads an empty ID as
// "created outside gnosis" and quarantines it, so the projection must not drop
// the document or invent a value.
func TestObservedPreservesAnEmptyIdentifier(t *testing.T) {
	t.Parallel()
	docs, err := bundle.Load(fstest.MapFS{"c/anon.md": concept("", "Anon")})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	obs := bundle.Observed(docs)
	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1", len(obs))
	}
	if obs[0].ID != "" {
		t.Errorf("ID = %q, want empty", obs[0].ID)
	}
	if obs[0].Path != "c/anon.md" {
		t.Errorf("Path = %q, want c/anon.md", obs[0].Path)
	}
}

func TestLoadLog(t *testing.T) {
	t.Parallel()
	t.Run("absent", func(t *testing.T) {
		t.Parallel()
		lines, present, err := bundle.LoadLog(fstest.MapFS{})
		if err != nil {
			t.Fatalf("LoadLog: %v", err)
		}
		if present {
			t.Error("present = true for a bundle with no log.md")
		}
		if lines != nil {
			t.Errorf("lines = %v, want nil", lines)
		}
	})
	t.Run("present", func(t *testing.T) {
		t.Parallel()
		fsys := fstest.MapFS{"log.md": &fstest.MapFile{
			Data: []byte("# Log\n\n## 2026-08-20\n* a thing\n"),
		}}
		lines, present, err := bundle.LoadLog(fsys)
		if err != nil {
			t.Fatalf("LoadLog: %v", err)
		}
		if !present {
			t.Fatal("present = false for a bundle with a log")
		}
		if len(lines) != 4 {
			t.Errorf("got %d lines, want 4: %q", len(lines), lines)
		}
	})
}

// TestSnapshotGathersBothHalvesOfTierZero. The closure check is pure and compares two
// sets; this is the shell filling them, which is the part that can be wrong without
// any check noticing — an empty RecordedText would make every archived file look
// orphaned, and the check would report a corpus with nothing wrong with it.
func TestSnapshotGathersBothHalvesOfTierZero(t *testing.T) {
	t.Parallel()

	const (
		archived  = "evidence/text/aa/aaaa.md"
		orphaned  = "evidence/text/bb/bbbb.md"
		recordDir = "evidence/fetch/cc/"
	)
	// A record naming the first file and not the second.
	record := `{"uri":"https://example.org/a.md","source_sha256":"aaaa",` +
		`"byte_size":4,"disposition":"archived","archive_path":"` + archived + `"}`

	fsys := fstest.MapFS{
		"c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-a.md": concept(
			"01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d", "A"),
		archived:                &fstest.MapFile{Data: []byte("archived text\n")},
		orphaned:                &fstest.MapFile{Data: []byte("orphaned text\n")},
		recordDir + "cccc.json": &fstest.MapFile{Data: []byte(record)},
	}

	snap, err := bundle.Snapshot(fsys, bundle.IndexState{}, bundle.FreshnessState{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !snap.ArchivedText[archived] || !snap.ArchivedText[orphaned] {
		t.Errorf("ArchivedText = %v, want both files", snap.ArchivedText)
	}
	if !snap.RecordedText[archived] {
		t.Errorf("RecordedText = %v, want the recorded path", snap.RecordedText)
	}
	if snap.RecordedText[orphaned] {
		t.Error("RecordedText names a path no record names")
	}
}

// TestAReferencedRecordAccountsForNoFile. A `referenced` record stored no text, so
// there is no file for it to account for — and counting it would make every weak
// source look like a record naming an absent file.
func TestAReferencedRecordAccountsForNoFile(t *testing.T) {
	t.Parallel()

	record := `{"uri":"https://example.org/big.pdf","source_sha256":"bbbb",` +
		`"byte_size":9,"disposition":"referenced","reject_reason":"extension-not-allowed"}`
	fsys := fstest.MapFS{
		"evidence/fetch/dd/dddd.json": &fstest.MapFile{Data: []byte(record)},
	}

	snap, err := bundle.Snapshot(fsys, bundle.IndexState{}, bundle.FreshnessState{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.RecordedText) != 0 {
		t.Errorf("RecordedText = %v, want nothing for a referenced record", snap.RecordedText)
	}
}
