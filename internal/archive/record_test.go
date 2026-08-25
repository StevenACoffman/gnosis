package archive_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/archive"
)

// TestRecordNamesItself is the invariant that makes append-only structural: the
// sha256 of a record file equals its own filename, so a rewritten record lands
// somewhere else and tampering is visible rather than absorbed by a merge.
func TestRecordNamesItself(t *testing.T) {
	t.Parallel()
	rec := sample()

	canonical, err := rec.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	h, err := rec.Hash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if got := hex.EncodeToString(sha256Sum(canonical)); got != h {
		t.Errorf("sha256 of the file is %s, but the record is named %s", got, h)
	}

	p, err := rec.Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if want := "evidence/fetch/" + h[:2] + "/" + h + ".json"; p != want {
		t.Errorf("path = %q, want %q", p, want)
	}
}

// TestTwoWritersAgree is §4.6.1: two users who independently fetch one source
// must produce byte-identical records, or the ledger conflicts instead of merging.
func TestTwoWritersAgree(t *testing.T) {
	t.Parallel()
	body := []byte("shared source\n")
	c := archive.Candidate{URI: "https://example.org/s.md", Bytes: body, Extension: ".md"}

	first := archive.Decide(&c, gates())
	for range 50 {
		again := archive.Decide(&c, gates())
		if again.Record != first.Record {
			t.Fatalf("two decisions differ:\n%+v\n%+v", first.Record, again.Record)
		}
	}

	a, err := first.Record.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	second := archive.Decide(&c, gates())
	b, err := second.Record.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("the encodings differ:\n%s\n%s", a, b)
	}
}

// TestRefetchOfUnchangedBytesIsANoOp is the point of omitting the timestamp: the
// same source produces the same path, so the writer finds the record present and
// writes nothing. With a timestamp this would be a new file on every sweep.
func TestRefetchOfUnchangedBytesIsANoOp(t *testing.T) {
	t.Parallel()
	c := archive.Candidate{
		URI:       "https://example.org/s.md",
		Bytes:     []byte("stable\n"),
		Extension: ".md",
	}

	first := archive.Decide(&c, gates())
	firstPath, err := first.Record.Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	later := archive.Decide(&c, gates())
	laterPath, err := later.Record.Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if firstPath != laterPath {
		t.Errorf("a re-fetch of unchanged bytes moved: %q then %q", firstPath, laterPath)
	}
}

// TestChangedSourceAppendsAVersion: a changed source must land at a second path
// and leave the first untouched, which is what "append a version rather than
// overwrite one" means on a filesystem (§4.1).
func TestChangedSourceAppendsAVersion(t *testing.T) {
	t.Parallel()
	uri := "https://example.org/s.md"
	before := archive.Decide(&archive.Candidate{
		URI: uri, Bytes: []byte("version one\n"), Extension: ".md",
	}, gates())
	after := archive.Decide(&archive.Candidate{
		URI: uri, Bytes: []byte("version two\n"), Extension: ".md",
	}, gates())

	bp, err := before.Record.Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	ap, err := after.Record.Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if bp == ap {
		t.Fatal("a changed source overwrote its predecessor")
	}
	if before.Record.ArchivePath == after.Record.ArchivePath {
		t.Error("two versions of a source share one archived text path")
	}
}

// TestNoTimestampField guards §4.3.1's load-bearing decision against a future
// edit that reintroduces one. The field would make tier 0 grow when somebody
// checks rather than when the corpus learns something.
func TestNoTimestampField(t *testing.T) {
	t.Parallel()
	rec := sample()
	b, err := rec.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	for _, banned := range []string{"fetched_at", "timestamp", "_at\"", "time"} {
		if strings.Contains(string(b), banned) {
			t.Errorf("the record encoding contains %q: %s", banned, b)
		}
	}
}

// TestCanonicalIsOneLine, because the file is written as it is hashed and a
// reader checks it with shasum.
func TestCanonicalIsOneLine(t *testing.T) {
	t.Parallel()
	b, err := sample().Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if n := strings.Count(string(b), "\n"); n != 1 {
		t.Errorf("canonical form has %d newlines, want the single trailing one", n)
	}
	if !strings.HasSuffix(string(b), "\n") {
		t.Error("canonical form does not end in a newline")
	}
}

// TestEmptyFieldsAreOmitted: an absent field must not appear, or an archived
// record and a referenced one would differ by more than what actually happened.
func TestEmptyFieldsAreOmitted(t *testing.T) {
	t.Parallel()
	rec := archive.Record{
		URI: "u", SourceSHA256: "ab", ByteSize: 2, Disposition: archive.Archived,
		ArchivePath: "evidence/text/ab/ab.md",
	}
	b, err := rec.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	for _, absent := range []string{"extractor", "extracted_from", "reject_reason", "media_type"} {
		if strings.Contains(string(b), absent) {
			t.Errorf("an unset field %q was encoded: %s", absent, b)
		}
	}
}

func sample() *archive.Record {
	return &archive.Record{
		URI:          "https://example.org/a.md",
		SourceSHA256: strings.Repeat("a", 64),
		ByteSize:     35,
		MediaType:    "text/markdown",
		Disposition:  archive.Archived,
		ArchivePath:  "evidence/text/aa/" + strings.Repeat("a", 64) + ".md",
	}
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

// TestARevisionDoesNotReachTheRecord is a guard against the obvious next commit.
//
// `Candidate.Revision` is provenance for the person doing the fetch, and putting it on
// the record is the change somebody will reach for — it is right there, and a record
// is where provenance usually goes. §4.3.1 is what it would break: a record's name is
// the hash of its own content, so a field that varies with the *repository's* activity
// would make one unrelated push re-record every file in the tree, identical to its
// predecessor but for a hash nobody reads. Tier 0 grows when the corpus learns
// something, not when somebody pushes.
//
// Asserted on the canonical bytes rather than by inspecting fields, because that is
// the thing the invariant is actually about: two fetches of one file must produce the
// same record, byte for byte, whatever the repository did in between.
func TestARevisionDoesNotReachTheRecord(t *testing.T) {
	t.Parallel()

	const text = "Vendor documentation. The queue drains in order and never reorders.\n"
	gates := archive.Gates{
		Allowlist: []string{".md"}, PerFileCap: 262144, EmbeddedPayloadCap: 8192,
		ScanText: archive.NoScan,
	}

	plain := archive.Decide(&archive.Candidate{
		URI:   "git://example.org/repo.git#docs/queue.md",
		Bytes: []byte(text), Extension: ".md",
	}, gates)
	stamped := archive.Decide(&archive.Candidate{
		URI:   "git://example.org/repo.git#docs/queue.md",
		Bytes: []byte(text), Extension: ".md",
		Revision: "9f2c1b7ae4d05836f1c0aa2b3d4e5f60718293a4",
	}, gates)

	want, err := plain.Record.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	got, err := stamped.Record.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("a revision changed the record:\n%s\n%s", want, got)
	}
}
