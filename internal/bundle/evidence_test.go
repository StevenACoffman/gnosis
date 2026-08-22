package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/skillet/errs"
)

// outcomeFor builds a decided candidate the store can be handed.
func outcomeFor(t *testing.T, body string) archive.Outcome {
	t.Helper()
	out := archive.Decide(&archive.Candidate{
		URI: "https://example.org/a.md", Bytes: []byte(body), Extension: ".md",
	}, archive.Gates{
		Allowlist: []string{".md"}, PerFileCap: 262144, EmbeddedPayloadCap: 8192,
	})
	if out.Record.Disposition != archive.Archived {
		t.Fatalf("fixture was not archived: %q", out.Record.RejectReason)
	}
	return out
}

// TestStoringTwiceIsANoOp: the ordinary case, and the one the third outcome must
// not disturb. Identical bytes at a content-addressed path is what a re-fetch
// looks like.
func TestStoringTwiceIsANoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := outcomeFor(t, "stable text\n")

	first, err := bundle.StoreEvidence(dir, &out)
	if err != nil {
		t.Fatalf("first store: %v", err)
	}
	if !first.Wrote {
		t.Error("the first store wrote nothing")
	}

	second, err := bundle.StoreEvidence(dir, &out)
	if err != nil {
		t.Fatalf("a re-store of identical bytes errored: %v", err)
	}
	if second.Wrote {
		t.Error("a re-store of identical bytes wrote again")
	}
}

// TestDifferentBytesAtOnePathIsCorruption is the whole point of the change.
// Every path here is the hash of its own content, so two contents cannot
// legitimately arrive at one path; reporting that as "already present" is the
// failure §4.3.1's append-only argument rests on not happening.
func TestDifferentBytesAtOnePathIsCorruption(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := outcomeFor(t, "the real text\n")
	if _, err := bundle.StoreEvidence(dir, &out); err != nil {
		t.Fatalf("store: %v", err)
	}

	// Rewrite the archived text in place without renaming it — exactly what a
	// careless edit or a damaged file looks like.
	full := filepath.Join(dir, filepath.FromSlash(out.Record.ArchivePath))
	if err := os.WriteFile(full, []byte("tampered\n"), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	_, err := bundle.StoreEvidence(dir, &out)
	if err == nil {
		t.Fatal("a path holding different bytes stored cleanly")
	}
	if errs.ErrorCode(err) != errs.ECONFLICT {
		t.Errorf("code = %q, want ECONFLICT", errs.ErrorCode(err))
	}
	if !strings.Contains(err.Error(), out.Record.ArchivePath) {
		t.Errorf("the error does not name the path: %v", err)
	}
	if !strings.Contains(err.Error(), "corruption") {
		t.Errorf("the error does not say what this is: %v", err)
	}
}

// TestACorruptRecordIsAlsoCaught: the ledger entry is content-addressed too, and
// it is the one that matters most — for a `referenced` source it is the only
// record there will ever be.
func TestACorruptRecordIsAlsoCaught(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := outcomeFor(t, "some text\n")

	stored, err := bundle.StoreEvidence(dir, &out)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	full := filepath.Join(dir, filepath.FromSlash(stored.RecordPath))
	if err = os.WriteFile(full, []byte(`{"uri":"elsewhere"}`+"\n"), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	if _, err = bundle.StoreEvidence(dir, &out); err == nil {
		t.Fatal("a corrupted fetch record stored cleanly")
	}
}

// TestCorruptionDoesNotOverwrite. Refusing is only useful if the bytes that were
// there survive to be looked at.
func TestCorruptionDoesNotOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := outcomeFor(t, "the real text\n")
	if _, err := bundle.StoreEvidence(dir, &out); err != nil {
		t.Fatalf("store: %v", err)
	}

	full := filepath.Join(dir, filepath.FromSlash(out.Record.ArchivePath))
	const tampered = "tampered\n"
	if err := os.WriteFile(full, []byte(tampered), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, err := bundle.StoreEvidence(dir, &out); err == nil {
		t.Fatal("expected a conflict")
	}

	after, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) != tampered {
		t.Errorf("the refusal overwrote the evidence: %q", after)
	}
}
