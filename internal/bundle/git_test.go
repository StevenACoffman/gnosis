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
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		runGit(t, dir, args...)
	}
	return dir
}

// runGit runs one git command in dir, isolated from the developer's configuration.
//
// **The environment is the substance, and one setting is not the point.** An earlier
// version of this fixture ran `git config commit.gpgsign false`, because on a machine
// that signs commits `git commit` blocked on the GPG agent and the test hung for two
// minutes — passing or failing depending on whose laptop it was. That fixed the
// setting that happened to bite. `core.hooksPath`, a global `pre-commit`,
// `commit.template`, `gpg.format=ssh` and `init.defaultBranch` are all in the same
// position, and each would have needed its own line.
//
// Nulling both config files closes the class instead, and the identity moves into the
// environment so the fixture needs no `git config` step at all — three subprocesses
// where there were six. `GIT_TERMINAL_PROMPT=0` is the same reasoning applied to the
// one thing a config file cannot cause: a prompt that blocks forever.
//
// **This is also what TODO's "fragile under concurrent load" entry was about.** That
// entry read `fatal: failed to write commit object` under two simultaneous test runs
// as filesystem and subprocess contention in a `t.Parallel()` test. It was the same
// GPG timeout: git prints exactly that line when signing fails, and two runs meant two
// signing requests. Verified after this change — two concurrent runs of the whole
// package, green in 2.4 s — so the parallelism was never the problem and is left alone.
//
// It is duplicated in the other test package that builds a repository rather than
// shared. rules.md §10 prefers a flat self-contained test and says to abstract "only
// when it is truly universal"; two callers is not universal, and the shared form would
// be a package importing `testing` into the production build. If a third fixture
// appears, that trade changes.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	//nolint:gosec // G204: args comes from a literal table at the call site; no
	// value from outside this file reaches the command line.
	c := exec.CommandContext(t.Context(), "git", args...)
	c.Dir = dir
	c.Env = []string{
		// Neither config file is read, so nothing the reader has configured
		// globally reaches this repository.
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		// The identity, which is otherwise the only thing the config steps were for.
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.org",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.org",
		// Nothing may block waiting for input.
		"GIT_TERMINAL_PROMPT=0",
		// PATH and HOME are passed through: git needs PATH to find its own
		// helpers, and some subcommands still want HOME even with the config
		// nulled.
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
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
		runGit(t, dir, args...)
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

// TestACloneReportsItsRevision closes the half of the entry that could be closed.
//
// A reader had no way to learn which revision a git-sourced record came from, and
// nothing said so. The commit cannot go on the record — §4.3.1 — so what was missing
// was the fetch saying it at the one moment it is known.
//
// The assertion is that every candidate carries the *same* commit, which is the
// property that makes it useful: a clone is one revision of one tree, so a reader who
// finds the line beside any file has found it for all of them.
func TestACloneReportsItsRevision(t *testing.T) {
	t.Parallel()
	origin := originRepo(t, map[string]string{
		"a.md": "first\n",
		"b.md": "second\n",
	})

	var f bundle.Fetcher
	got, err := f.Fetch(t.Context(), origin+"/.git")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("the fixture cloned nothing")
	}
	first := got[0].Revision
	if len(first) != 40 {
		t.Errorf("revision %q is not a full commit hash", first)
	}
	for i := range got {
		if got[i].Revision != first {
			t.Errorf("%s reports revision %q, want %q",
				got[i].URI, got[i].Revision, first)
		}
	}
}

// TestANonGitFetchReportsNoRevision keeps the field from acquiring a meaning it does
// not have. A local file has no revision, and an empty string says so; anything
// invented here would be provenance the fetch made up.
func TestANonGitFetchReportsNoRevision(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "plain.md")
	if err := os.WriteFile(path, []byte("Local text.\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	var f bundle.Fetcher
	got, err := f.Fetch(t.Context(), path)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(got) != 1 || got[0].Revision != "" {
		t.Errorf("a local file reported revision %q", got[0].Revision)
	}
}
