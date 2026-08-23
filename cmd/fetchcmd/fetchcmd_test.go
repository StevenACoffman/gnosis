package fetchcmd_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
)

// run invokes the dispatcher directly with injected I/O, which is the seam
// rules.md §7 describes for exercising a command without a subprocess.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(t.Context(), args, strings.NewReader(""), &out, &errOut)
	return out.String(), errOut.String(), err
}

// payload decodes the machine envelope's data into a Result-shaped value. The
// command's own type is not reused: an agent parses JSON, and a test that decoded
// through the producer's struct would not notice a renamed field.
func payload(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env struct {
		Status root.Status    `json:"status"`
		Data   map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", err, stdout)
	}
	if env.Status != root.StatusOK {
		t.Fatalf("status = %q, want ok:\n%s", env.Status, stdout)
	}
	return env.Data
}

// sourceDir writes files to fetch and returns the directory holding them.
func sourceDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func TestArchivesALocalFile(t *testing.T) {
	t.Parallel()
	bundleDir := t.TempDir()
	src := filepath.Join(
		sourceDir(t, map[string]string{"note.md": "# Note\n\nA claim.\n"}),
		"note.md",
	)

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	data := payload(t, stdout)
	if got := data["durable"]; got != float64(1) {
		t.Errorf("durable = %v, want 1", got)
	}
	if got := data["wrote"]; got != float64(1) {
		t.Errorf("wrote = %v, want 1", got)
	}

	// The record must be on disk, named by its own hash — the invariant that
	// makes append-only structural.
	rec := onlyRecord(t, bundleDir)
	raw, err := os.ReadFile(filepath.Join(bundleDir, rec))
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	sum := sha256.Sum256(raw)
	if want := hex.EncodeToString(sum[:]) + ".json"; filepath.Base(rec) != want {
		t.Errorf("record is named %s but hashes to %s", filepath.Base(rec), want)
	}
}

// TestRefetchWritesNothing is §9.2's no-op, and the property that omitting the
// timestamp exists to buy. It cannot be observed in the pure package: only the
// shell knows whether anything reached the disk.
func TestRefetchWritesNothing(t *testing.T) {
	t.Parallel()
	bundleDir := t.TempDir()
	src := filepath.Join(sourceDir(t, map[string]string{"a.md": "stable\n"}), "a.md")

	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	before := treeOf(t, bundleDir)

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	data := payload(t, stdout)
	if got := data["wrote"]; got != float64(0) {
		t.Errorf("a re-fetch of unchanged bytes wrote %v files, want 0", got)
	}
	if got := data["unchanged"]; got != float64(1) {
		t.Errorf("unchanged = %v, want 1", got)
	}
	if after := treeOf(t, bundleDir); after != before {
		t.Errorf("the evidence tree changed on a re-fetch:\n%s\n%s", before, after)
	}
}

// TestChangedSourceAppends: a source that actually changed must add a record and
// leave the first one exactly as it was.
func TestChangedSourceAppends(t *testing.T) {
	t.Parallel()
	bundleDir := t.TempDir()
	dir := sourceDir(t, map[string]string{"a.md": "version one\n"})
	src := filepath.Join(dir, "a.md")

	if _, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	first := onlyRecord(t, bundleDir)
	firstBytes, err := os.ReadFile(filepath.Join(bundleDir, first))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if werr := os.WriteFile(src, []byte("version two\n"), 0o600); werr != nil {
		t.Fatalf("rewrite source: %v", werr)
	}
	if _, _, ferr := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src); ferr != nil {
		t.Fatalf("second fetch: %v", ferr)
	}

	if got := len(recordPaths(t, bundleDir)); got != 2 {
		t.Errorf("a changed source produced %d records, want 2", got)
	}
	after, err := os.ReadFile(filepath.Join(bundleDir, first))
	if err != nil {
		t.Fatalf("the first record is gone: %v", err)
	}
	if !bytes.Equal(after, firstBytes) {
		t.Error("a changed source rewrote its predecessor")
	}
}

// TestDryRunWritesNothing: preview and apply are one command differing in one
// field, and the preview must not touch the disk.
func TestDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	bundleDir := t.TempDir()
	src := filepath.Join(sourceDir(t, map[string]string{"a.md": "hello\n"}), "a.md")

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", "--dry-run", src)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if got := payload(t, stdout)["durable"]; got != float64(1) {
		t.Errorf("a dry run reported durable = %v, want the same answer as a real run", got)
	}
	if paths := recordPaths(t, bundleDir); len(paths) != 0 {
		t.Errorf("a dry run wrote %v", paths)
	}
}

// TestDryRunPredictsTheRealRun is what makes §9.4's guarantee checkable: the
// preview and the apply must agree on every disposition and every path.
func TestDryRunPredictsTheRealRun(t *testing.T) {
	t.Parallel()
	dir := sourceDir(t, map[string]string{
		"a.md":      "text\n",
		"b.bin":     "\x00\x01binary\n",
		"c/d.txt":   "nested\n",
		"e.unknown": "not on the allowlist\n",
	})

	previewOut, _, err := run(t, "--bundle", t.TempDir(), "--jsonl", "fetch", "--dry-run", dir)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	applyOut, _, err := run(t, "--bundle", t.TempDir(), "--jsonl", "fetch", dir)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	preview, apply := dispositions(t, previewOut), dispositions(t, applyOut)
	if len(preview) != len(apply) {
		t.Fatalf("preview saw %d sources, apply saw %d", len(preview), len(apply))
	}
	for uri, want := range preview {
		if got := apply[uri]; got != want {
			t.Errorf("%s: preview said %q, apply did %q", uri, want, got)
		}
	}
}

// TestUnarchivableFallsThroughRatherThanFailing: `referenced` is a supported
// outcome, and a run containing one must still exit ok.
func TestUnarchivableFallsThroughRatherThanFailing(t *testing.T) {
	t.Parallel()
	bundleDir := t.TempDir()
	src := filepath.Join(sourceDir(t, map[string]string{"paper.pdf": "%PDF-1.7\n"}), "paper.pdf")

	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", src)
	if err != nil {
		t.Fatalf("an unarchivable source failed the run: %v", err)
	}

	data := payload(t, stdout)
	if got := data["weak"]; got != float64(1) {
		t.Errorf("weak = %v, want 1", got)
	}
	sources, _ := data["sources"].([]any)
	if len(sources) != 1 {
		t.Fatalf("want one source, got %v", sources)
	}
	s, _ := sources[0].(map[string]any)
	if s["disposition"] != "referenced" {
		t.Errorf("disposition = %v, want referenced", s["disposition"])
	}
	if s["reject_reason"] != "extension-not-allowed" {
		t.Errorf("reject_reason = %v, want the extension gate", s["reject_reason"])
	}
	// Even storing nothing, the record must exist: for exactly these the ledger
	// is the only evidence there will be.
	if paths := recordPaths(t, bundleDir); len(paths) != 1 {
		t.Errorf("a referenced source produced %d records, want 1", len(paths))
	}
}

func TestExtractsHTML(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><nav>skip</nav><article>` +
			`<h1>Heading</h1><p>The quotable sentence.</p></article></body></html>`))
	}))
	defer srv.Close()

	bundleDir := t.TempDir()
	stdout, _, err := run(t, "--bundle", bundleDir, "--jsonl", "fetch", srv.URL+"/page.html")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	sources, _ := payload(t, stdout)["sources"].([]any)
	s, _ := sources[0].(map[string]any)
	if s["disposition"] != "extracted" {
		t.Fatalf("disposition = %v, want extracted (%v)", s["disposition"], s["reject_reason"])
	}

	stored, err := os.ReadFile(
		filepath.Join(bundleDir, filepath.FromSlash(s["archive_path"].(string))),
	)
	if err != nil {
		t.Fatalf("read extraction: %v", err)
	}
	// The extraction is what a quote will be validated against, so the prose has
	// to survive even though the markup did not.
	if !strings.Contains(string(stored), "The quotable sentence.") {
		t.Errorf("the extraction lost the prose:\n%s", stored)
	}
}

// TestAnHTTPErrorIsNotEvidence: a 404 page is a document, and archiving it would
// store the error page under the URI of the thing that is missing.
func TestAnHTTPErrorIsNotEvidence(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	bundleDir := t.TempDir()
	_, stderr, err := run(t, "--bundle", bundleDir, "fetch", srv.URL+"/missing.md")
	if err == nil {
		t.Fatal("a 404 was archived as evidence")
	}
	if !strings.Contains(stderr, "404") {
		t.Errorf("stderr does not name the status: %q", stderr)
	}
	if paths := recordPaths(t, bundleDir); len(paths) != 0 {
		t.Errorf("a failed fetch left records: %v", paths)
	}
}

func TestNoAdapterIsAUsageError(t *testing.T) {
	t.Parallel()
	_, _, err := run(t, "--bundle", t.TempDir(), "fetch", "gopher://example.org/x")
	if err == nil {
		t.Fatal("an unclaimed scheme succeeded")
	}
}

// dispositions maps each reported URI to its disposition.
func dispositions(t *testing.T, stdout string) map[string]string {
	t.Helper()
	out := map[string]string{}
	sources, _ := payload(t, stdout)["sources"].([]any)
	for _, raw := range sources {
		s, _ := raw.(map[string]any)
		uri, _ := s["uri"].(string)
		d, _ := s["disposition"].(string)
		out[filepath.Base(uri)] = d
	}
	return out
}

// recordPaths lists every fetch record beneath a bundle, relative to it.
func recordPaths(t *testing.T, bundleDir string) []string {
	t.Helper()
	var out []string
	fetchDir := filepath.Join(bundleDir, "evidence", "fetch")
	err := filepath.WalkDir(fetchDir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		rel, rerr := filepath.Rel(bundleDir, path)
		if rerr != nil {
			return rerr
		}
		out = append(out, rel)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walk records: %v", err)
	}
	return out
}

func onlyRecord(t *testing.T, bundleDir string) string {
	t.Helper()
	paths := recordPaths(t, bundleDir)
	if len(paths) != 1 {
		t.Fatalf("want exactly one record, got %v", paths)
	}
	return paths[0]
}

// treeOf renders every path beneath evidence/ with its size, so a test can assert
// that a run changed nothing at all rather than merely reporting that it did not.
func treeOf(t *testing.T, bundleDir string) string {
	t.Helper()
	var b strings.Builder
	evidenceDir := filepath.Join(bundleDir, "evidence")
	err := filepath.WalkDir(evidenceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		rel, rerr := filepath.Rel(bundleDir, path)
		if rerr != nil {
			return rerr
		}
		_, _ = b.WriteString(rel)
		_, _ = b.WriteString(" ")
		_, _ = b.WriteString(info.Mode().String())
		if !d.IsDir() {
			_, _ = b.WriteString(" ")
			_, _ = b.WriteString(strconv.FormatInt(info.Size(), 10))
		}
		_, _ = b.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatalf("walk evidence: %v", err)
	}
	return b.String()
}
