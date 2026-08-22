package standardscmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/internal/standards"
)

func run(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(t.Context(), args, strings.NewReader(""), &out, &errOut)
	return out.String(), err
}

// committedBundle writes the seed standards into a real git repository and
// commits them, so there is a revision to compare against.
//
// It shells out to git rather than building the repository with go-git: the code
// under test reads what git produced, and a fixture built by the same library
// would only prove go-git agrees with itself.
func committedBundle(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "standards"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write(t, dir, standards.ArchiveFileName, standards.DefaultArchive())
	write(t, dir, standards.PromoteFileName, standards.DefaultPromote())

	for _, args := range [][]string{
		{"init", "--initial-branch", "main"},
		{"config", "user.email", "test@example.org"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "seed standards"},
	} {
		//nolint:gosec // G204: args comes from the literal table above it; no
		// value from outside this file reaches the command line.
		c := exec.CommandContext(t.Context(), "git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func write(t *testing.T, dir, rel string, body []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), body, 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// loosen rewrites one value in a standards file in the worktree.
func loosen(t *testing.T, dir, rel, old, replacement string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	updated := strings.Replace(string(body), old, replacement, 1)
	if updated == string(body) {
		t.Fatalf("the fixture no longer matches: %q not in %s", old, rel)
	}
	write(t, dir, rel, []byte(updated))
}

// TestNothingLoosenedIsSilent, or the report fires on every clean run and gets
// ignored within a week.
func TestNothingLoosenedIsSilent(t *testing.T) {
	t.Parallel()
	dir := committedBundle(t)

	stdout, err := run(t, "--bundle", dir, "standards", "check")
	if err != nil {
		t.Fatalf("a clean bundle reported: %v", err)
	}
	if !strings.Contains(stdout, "nothing loosened") {
		t.Errorf("stdout = %q", stdout)
	}
}

// TestALooseningIsAFinding: the tool worked and the corpus is in a state a person
// should look at, which is what §17's findings status is for and what a CI job
// branches on.
func TestALooseningIsAFinding(t *testing.T) {
	t.Parallel()
	dir := committedBundle(t)
	loosen(t, dir, standards.PromoteFileName, "value = 3", "value = 9")

	stdout, err := run(t, "--bundle", dir, "--jsonl", "standards", "check")
	if err == nil {
		t.Fatal("a loosening exited zero")
	}

	var env struct {
		Status string `json:"status"`
		Data   struct {
			Loosenings []struct {
				Key       string `json:"key"`
				From      string `json:"from"`
				To        string `json:"to"`
				Countable bool   `json:"countable"`
				Why       string `json:"why"`
			} `json:"loosenings"`
		} `json:"data"`
	}
	if uErr := json.Unmarshal([]byte(stdout), &env); uErr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", uErr, stdout)
	}
	if env.Status != "findings" {
		t.Errorf("status = %q, want findings", env.Status)
	}
	if len(env.Data.Loosenings) != 1 {
		t.Fatalf("got %d loosenings: %+v", len(env.Data.Loosenings), env.Data.Loosenings)
	}
	l := env.Data.Loosenings[0]
	if l.Key != "hedging_max" || l.From != "3" || l.To != "9" {
		t.Errorf("got %+v", l)
	}
	// The gate thresholds produce no finding, and the report says so rather than
	// printing a zero delta that would read as "it cost nothing".
	if l.Countable {
		t.Error("hedging_max reported a finding count; nothing counts it")
	}
	if !strings.Contains(l.Why, "promote gate") {
		t.Errorf("why = %q, does not explain the absence", l.Why)
	}
}

// TestTheBudgetDeltaIsCounted. corpus_budget is the one threshold that currently
// feeds a lint finding, so raising it can silence a real one and the delta is
// exact — the archive is measured once and diagnosed twice.
func TestTheBudgetDeltaIsCounted(t *testing.T) {
	t.Parallel()
	dir := committedBundle(t)
	// An archive that is over a tiny budget, then a budget that forgives it.
	if err := os.MkdirAll(filepath.Join(dir, "evidence", "text", "aa"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write(t, dir, "evidence/text/aa/big.md", bytes.Repeat([]byte("x"), 5000))
	loosen(t, dir, standards.ArchiveFileName, "value = 268435456", "value = 4000")
	loosen(t, dir, standards.ArchiveFileName, "value = 262144", "value = 1000")

	// Commit that state, then loosen the budget past the archive's size.
	commit(t, dir)
	loosen(t, dir, standards.ArchiveFileName, "value = 4000", "value = 900000")

	stdout, err := run(t, "--bundle", dir, "--jsonl", "standards", "check")
	if err == nil {
		t.Fatal("a budget loosening exited zero")
	}

	var env struct {
		Data struct {
			Loosenings []struct {
				Key            string `json:"key"`
				Countable      bool   `json:"countable"`
				FindingsBefore int    `json:"findings_before"`
				FindingsAfter  int    `json:"findings_after"`
			} `json:"loosenings"`
		} `json:"data"`
	}
	if uErr := json.Unmarshal([]byte(stdout), &env); uErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", uErr, stdout)
	}
	if len(env.Data.Loosenings) != 1 {
		t.Fatalf("got %+v", env.Data.Loosenings)
	}
	l := env.Data.Loosenings[0]
	if l.Key != "corpus_budget" {
		t.Fatalf("key = %q", l.Key)
	}
	if !l.Countable {
		t.Fatal("the budget delta was not counted; it is the one that can be")
	}
	if l.FindingsBefore != 1 || l.FindingsAfter != 0 {
		t.Errorf("findings %d to %d, want 1 to 0 — the loosening silenced one",
			l.FindingsBefore, l.FindingsAfter)
	}
}

// TestLogFilesTheEntry, which is the committed record §6.2 asks for.
func TestLogFilesTheEntry(t *testing.T) {
	t.Parallel()
	dir := committedBundle(t)
	loosen(t, dir, standards.PromoteFileName, "value = 3", "value = 9")

	if _, err := run(t, "--bundle", dir, "standards", "check", "--log"); err == nil {
		t.Fatal("expected findings")
	}

	body, err := os.ReadFile(filepath.Join(dir, "log.md"))
	if err != nil {
		t.Fatalf("no log.md was written: %v", err)
	}
	for _, want := range []string{"hedging_max", "3 to 9", "promote.toml"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("log.md omits %q:\n%s", want, body)
		}
	}
}

// TestAnUnknownRevisionIsRefused rather than silently compared against nothing.
func TestAnUnknownRevisionIsRefused(t *testing.T) {
	t.Parallel()
	dir := committedBundle(t)

	if _, err := run(t, "--bundle", dir, "standards", "check",
		"--since", "no-such-ref"); err == nil {
		t.Fatal("an unknown revision compared cleanly")
	}
}

func commit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "second"}} {
		//nolint:gosec // G204: args comes from the literal table above it.
		c := exec.CommandContext(t.Context(), "git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
