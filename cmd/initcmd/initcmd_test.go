package initcmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/cmd/initcmd"
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

// initResult runs init under --jsonl and decodes the payload.
func initResult(t *testing.T, dir string) initcmd.Result {
	t.Helper()
	stdout, _, err := run(t, "--bundle", dir, "--jsonl", "init")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	var got struct {
		root.Outcome
		Data initcmd.Result `json:"data"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &got); uerr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", uerr, stdout)
	}
	if got.Status != root.StatusOK {
		t.Fatalf("status = %q, want %q", got.Status, root.StatusOK)
	}
	return got.Data
}

// TestInitScaffoldsAUsableBundle asserts the whole point: what init leaves
// behind must pass lint, because a corpus that is dirty the moment it is created
// teaches a reader that findings are noise.
func TestInitScaffoldsAUsableBundle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result := initResult(t, dir)
	if len(result.Created) == 0 {
		t.Fatal("init created nothing in an empty directory")
	}

	for _, want := range []string{
		"ontology.toml", "index.md", "log.md", ".gitignore",
		"c", ".gnosis/index.db",
	} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("%s is missing after init: %v", want, err)
		}
	}

	if _, _, err := run(t, "--bundle", dir, "lint"); err != nil {
		t.Errorf("a freshly initialised bundle does not lint clean: %v", err)
	}
}

// TestInitIsIdempotent: this is the command most likely to be run twice, or in
// the wrong directory. Overwriting a live corpus's vocabulary would be the
// costliest thing gnosis could do, so the second run must change nothing.
func TestInitIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first := initResult(t, dir)
	before, err := os.ReadFile(filepath.Join(dir, "ontology.toml"))
	if err != nil {
		t.Fatalf("read vocabulary: %v", err)
	}

	second := initResult(t, dir)
	if len(second.Created) != 0 {
		t.Errorf("the second run created %v, want nothing", second.Created)
	}
	if len(second.Existing) != len(first.Created) {
		t.Errorf("the second run reported %d existing, want the %d it created first",
			len(second.Existing), len(first.Created))
	}

	after, err := os.ReadFile(filepath.Join(dir, "ontology.toml"))
	if err != nil {
		t.Fatalf("read vocabulary: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the second run rewrote ontology.toml")
	}
}

// TestInitDoesNotOverwriteAnEditedVocabulary is the case idempotency exists for:
// the file has been edited by a person, and init must leave it exactly as found.
func TestInitDoesNotOverwriteAnEditedVocabulary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	edited := "version = 1\n\n[[types]]\nkey = \"Runbook\"\ndesc = \"ours\"\n"
	if err := os.WriteFile(filepath.Join(dir, "ontology.toml"), []byte(edited), 0o600); err != nil {
		t.Fatalf("seed an edited vocabulary: %v", err)
	}

	result := initResult(t, dir)

	got, err := os.ReadFile(filepath.Join(dir, "ontology.toml"))
	if err != nil {
		t.Fatalf("read vocabulary: %v", err)
	}
	if string(got) != edited {
		t.Errorf("init overwrote an edited vocabulary:\n%s", got)
	}
	if len(result.Existing) == 0 {
		t.Error("init did not report that it kept an existing file")
	}
}

// TestInitReportsExistingFilesOnStderr keeps stdout parseable: under the human
// form, stdout lists what was created and nothing else.
func TestInitReportsExistingFilesOnStderr(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	stdout, stderr, err := run(t, "--bundle", dir, "init")
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("the second run wrote to stdout, having created nothing:\n%s", stdout)
	}
	if !strings.Contains(stderr, "kept existing ontology.toml") {
		t.Errorf("stderr does not say what was kept:\n%s", stderr)
	}
}
