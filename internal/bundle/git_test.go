package bundle_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// originRepo builds a real git repository on disk and returns its path.
//
// It shells out to git rather than constructing one with go-git, deliberately:
// the adapter's job is to read what git itself produces, and a fixture built by
// the same library would only prove go-git agrees with itself.
func originRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	for _, args := range [][]string{
		{"init", "--initial-branch", "main"},
		{"config", "user.email", "test@example.org"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		//nolint:gosec // G204: args comes from the literal table above it; no
		// value from outside this file reaches the command line.
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// TestClonesARepository: a `.git` remote goes to the git adapter, and every file
// in the working tree becomes a candidate.
func TestClonesARepository(t *testing.T) {
	t.Parallel()
	origin := originRepo(t, map[string]string{
		"README.md":     "# Origin\n\nA quotable claim.\n",
		"docs/note.txt": "nested\n",
	})

	var f bundle.Fetcher
	got, err := f.Fetch(t.Context(), origin+"/.git")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(got), got)
	}
	for i := range got {
		if strings.Contains(got[i].URI, "gnosis-clone-") {
			t.Errorf("a candidate records the temporary clone, not the remote: %q", got[i].URI)
		}
		if !strings.Contains(got[i].URI, "#") {
			t.Errorf("a candidate does not name its path within the repository: %q", got[i].URI)
		}
	}
}

// TestGitBookkeepingIsNotEvidence: `.git` is the repository's own record-keeping
// and archiving it would fill tier 0 with pack files that no quotation cites.
func TestGitBookkeepingIsNotEvidence(t *testing.T) {
	t.Parallel()
	origin := originRepo(t, map[string]string{"a.md": "text\n"})

	var f bundle.Fetcher
	got, err := f.Fetch(t.Context(), origin+"/.git")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	for i := range got {
		if strings.Contains(got[i].URI, "/.git/") || strings.HasSuffix(got[i].URI, "#.git") {
			t.Errorf("a candidate came from .git: %q", got[i].URI)
		}
	}
}

// TestCloneIsRemoved: a fetch leaves nothing behind but records. A clone left in
// place would be a copy of somebody's repository sitting in a temp directory
// nobody knows to clean.
func TestCloneIsRemoved(t *testing.T) {
	t.Parallel()
	origin := originRepo(t, map[string]string{"a.md": "text\n"})
	root := t.TempDir()

	f := bundle.Fetcher{Root: root}
	if _, err := f.Fetch(t.Context(), origin+"/.git"); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	left, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("the clone was left behind: %v", left)
	}
}

// TestTwoFetchesOfOneRepoAgree is what keeps the no-op property: the recorded URI
// carries no commit, so an unrelated push must not change any record.
func TestTwoFetchesOfOneRepoAgree(t *testing.T) {
	t.Parallel()
	origin := originRepo(t, map[string]string{"a.md": "stable\n", "b.md": "also\n"})

	var f bundle.Fetcher
	first, err := f.Fetch(t.Context(), origin+"/.git")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	// A commit that does not touch a.md must leave a.md's candidate identical.
	if werr := os.WriteFile(
		filepath.Join(origin, "b.md"),
		[]byte("changed\n"),
		0o600,
	); werr != nil {
		t.Fatalf("rewrite: %v", werr)
	}
	commit(t, origin)

	second, err := f.Fetch(t.Context(), origin+"/.git")
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	before, after := byURI(first), byURI(second)
	for uri, want := range before {
		if !strings.HasSuffix(uri, "#a.md") {
			continue
		}
		if got := after[uri]; got != want {
			t.Errorf("an untouched file changed across an unrelated commit:\n%q\n%q", want, got)
		}
	}
}

// TestAnHTTPSPageIsNotAClone: the common case for an https URI is a page to read,
// and guessing wrong would clone a website.
func TestAnHTTPSPageIsNotAClone(t *testing.T) {
	t.Parallel()
	var f bundle.Fetcher
	_, err := f.Fetch(t.Context(), "https://example.org/page.html")
	if err == nil {
		t.Skip("the network answered; this test only cares which adapter was chosen")
	}
	if strings.Contains(err.Error(), "clone") {
		t.Errorf("an ordinary https page went to the git adapter: %v", err)
	}
}

func commit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "second"}} {
		//nolint:gosec // G204: args comes from the literal table above it; no
		// value from outside this file reaches the command line.
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// byURI maps each candidate's recorded URI to its bytes, so two fetches can be
// compared on the thing a record is built from.
func byURI(candidates []archive.Candidate) map[string]string {
	out := make(map[string]string, len(candidates))
	for i := range candidates {
		out[candidates[i].URI] = string(candidates[i].Bytes)
	}
	return out
}
