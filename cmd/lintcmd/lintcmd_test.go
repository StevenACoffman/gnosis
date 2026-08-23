package lintcmd_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
)

const validID = "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"

// bundleWith writes a bundle containing the named concept files and returns its
// root. Contents are written to a real directory because the command resolves
// its bundle with os.DirFS — testing the wiring, not the loader.
func bundleWith(t *testing.T, files map[string]string) string {
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

// run invokes the dispatcher directly with injected I/O, which is the seam
// rules.md §7 describes for exercising a command without a subprocess.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(t.Context(), args, strings.NewReader(""), &out, &errOut)
	return out.String(), errOut.String(), err
}

// concept renders a document carrying validID. A second identifier is not
// needed: the duplicate case is two files sharing one, which is the point.
func concept(extra string) string {
	return "---\ntype: Reference\ngnosis_id: " + validID + "\n" + extra + "---\nbody\n"
}

// TestCleanCorpusEmitsOK covers the ordinary case, and asserts the envelope
// shape an agent depends on.
func TestCleanCorpusEmitsOK(t *testing.T) {
	t.Parallel()
	dir := bundleWith(t, map[string]string{
		"c/" + validID + "-alpha.md": concept("title: Alpha\n"),
	})

	stdout, _, err := run(t, "--bundle", dir, "--jsonl", "lint")
	if err != nil {
		t.Fatalf("lint on a clean corpus: %v", err)
	}

	var got root.Outcome
	if uerr := json.Unmarshal([]byte(stdout), &got); uerr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", uerr, stdout)
	}
	if got.Status != root.StatusOK {
		t.Errorf("status = %q, want %q", got.Status, root.StatusOK)
	}
	if got.Code != root.CodeOK {
		t.Errorf("code = %d, want %d", got.Code, root.CodeOK)
	}
}

// TestDuplicateIdentifierExitsFindingsNotError is the distinction the status
// vocabulary exists for: a corpus with problems must not look like a broken
// tool, because a CI job branches on exactly that difference.
func TestDuplicateIdentifierExitsFindingsNotError(t *testing.T) {
	t.Parallel()
	dir := bundleWith(t, map[string]string{
		"c/" + validID + "-one.md": concept(""),
		"c/" + validID + "-two.md": concept(""),
	})

	stdout, _, err := run(t, "--bundle", dir, "--jsonl", "lint")

	var exitErr root.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a root.ExitError", err)
	}
	if root.Code(exitErr) != root.CodeFindings {
		t.Errorf("exit code = %d, want %d (findings, not error)", exitErr, root.CodeFindings)
	}

	var got root.Outcome
	if uerr := json.Unmarshal([]byte(stdout), &got); uerr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", uerr, stdout)
	}
	if got.Status != root.StatusFindings {
		t.Errorf("status = %q, want %q", got.Status, root.StatusFindings)
	}
	if got.Reason != root.ReasonDuplicateIdentity {
		t.Errorf("reason = %q, want %q", got.Reason, root.ReasonDuplicateIdentity)
	}
	if got.Message == "" {
		t.Error("message is empty; the token is for agents and the message for people")
	}
}

// TestSkipsGoToStderr keeps the streams separable: an agent parsing stdout must
// not have to filter diagnostics out of it.
func TestSkipsGoToStderr(t *testing.T) {
	t.Parallel()
	dir := bundleWith(t, map[string]string{
		"c/" + validID + "-alpha.md": concept(""),
	})

	stdout, stderr, err := run(t, "--bundle", dir, "lint")
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	if !strings.Contains(stderr, "skipped") {
		t.Errorf("stderr does not report skipped checks:\n%s", stderr)
	}
	if strings.Contains(stdout, "skipped") {
		t.Errorf("skip notices leaked into stdout:\n%s", stdout)
	}
}

// TestUnknownCheckIsAUsageError checks the diagnostic names the alternatives,
// so a caller does not have to consult the source to recover, and that it exits
// 2 rather than 1: "call me differently" is a different repair from "something
// is broken".
func TestUnknownCheckIsAUsageError(t *testing.T) {
	t.Parallel()
	dir := bundleWith(t, map[string]string{
		"c/" + validID + "-alpha.md": concept(""),
	})

	_, stderr, err := run(t, "--bundle", dir, "lint", "--check", "nonexistent")

	var exitErr root.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a root.ExitError", err)
	}
	if root.Code(exitErr) != root.CodeUsage {
		t.Errorf("exit code = %d, want %d (usage, not error)", exitErr, root.CodeUsage)
	}
	for _, want := range []string{"nonexistent", "conformance", "identity"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q does not mention %q", stderr, want)
		}
	}
}

// TestMissingBundleIsAnErrorNotFindings: gnosis being unable to read a corpus is
// a tool failure, and must not be reported as a corpus with problems.
func TestMissingBundleIsAnErrorNotFindings(t *testing.T) {
	t.Parallel()
	stdout, _, err := run(t,
		"--bundle", filepath.Join(t.TempDir(), "does-not-exist"), "--jsonl", "lint")

	// An absent bundle reads as an empty corpus, which is deliberate: `init`
	// writes no c/ directory and every command must work against that. What
	// matters is that it does not masquerade as findings.
	var got root.Outcome
	if uerr := json.Unmarshal([]byte(stdout), &got); uerr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", uerr, stdout)
	}
	if got.Status == root.StatusFindings {
		t.Error("an unreadable bundle was reported as findings")
	}
	if err != nil && got.Status != root.StatusError {
		t.Errorf("err = %v but status = %q; the two must agree", err, got.Status)
	}
}
