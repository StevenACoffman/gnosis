package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withCases writes a retrieval-case file into a bundle.
func withCases(t *testing.T, dir, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "standards"), 0o750); err != nil {
		t.Fatalf("mkdir standards: %v", err)
	}
	path := filepath.Join(dir, "standards", "retrieval-cases.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write cases: %v", err)
	}
}

// TestAnEmptySuiteSaysItExaminedNothing is the shipped state and the one most likely
// to be mistaken for a pass.
//
// §11.0.2 says cases are authored when a real query disappoints, never invented up
// front — so a corpus with none is ordinary, and reporting that as success would be the
// silence `scan.Coverage` exists to break one subsystem over.
func TestAnEmptySuiteSaysItExaminedNothing(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	stdout, stderr, err := run(t, "--bundle", dir, "search", "--cases")
	if err != nil {
		t.Fatalf("an empty suite failed: %v\n%s", err, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("an empty suite printed results:\n%s", stdout)
	}
	if !strings.Contains(stderr, "no retrieval cases") {
		t.Errorf("the empty case does not say it examined nothing: %s", stderr)
	}
}

// TestACaseThatHoldsAndOneThatDoesNot is the instrument working, over the real index.
func TestACaseThatHoldsAndOneThatDoesNot(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	withCases(t, dir, `
[[case]]
query  = "retry"
titles = ["Retry Budget"]
why    = "the fixture corpus holds this and a search for it must find it"

[[case]]
query  = "retry"
titles = ["A Document Nobody Wrote"]
why    = "deliberately unsatisfiable, so the suite is known to be able to fail"
`)

	stdout, stderr, err := run(t, "--bundle", dir, "search", "--cases")
	if err == nil {
		t.Error("a failing case exited zero")
	}
	if !strings.Contains(stdout, "held\tretry") {
		t.Errorf("the satisfiable case did not hold:\n%s\n%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "failed\tretry") {
		t.Errorf("the unsatisfiable case did not fail:\n%s", stdout)
	}
	// A failure names what was missing and why the case exists, because a reader
	// looking at one needs to know what somebody was searching for.
	if !strings.Contains(stderr, "A Document Nobody Wrote") {
		t.Errorf("the failure does not name the missing title: %s", stderr)
	}
	if !strings.Contains(stderr, "deliberately unsatisfiable") {
		t.Errorf("the failure does not carry the case's reason: %s", stderr)
	}
	if !strings.Contains(stderr, "1 held, 1 failed") {
		t.Errorf("the summary is wrong: %s", stderr)
	}
}

// TestACaseMayRequireTheCorpusHoldNothing is the half §11.0.2 asks for by name, over
// the real index rather than from a literal.
//
// A corpus that answers every query with its best guess cannot say "we do not know",
// and this is what makes that answer assertable.
func TestACaseMayRequireTheCorpusHoldNothing(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	withCases(t, dir, `
[[case]]
query   = "kubernetes ingress annotations"
nothing = true
why     = "out of scope; the corpus deliberately holds nothing about cluster config"

[[case]]
query   = "retry"
nothing = true
why     = "deliberately wrong: the corpus does hold this, so the case must fail"
`)

	stdout, _, err := run(t, "--bundle", dir, "search", "--cases")
	if err == nil {
		t.Error("a nothing-case that matched exited zero")
	}
	if !strings.Contains(stdout, "held\tkubernetes ingress annotations") {
		t.Errorf("the out-of-scope case did not hold:\n%s", stdout)
	}
	if !strings.Contains(stdout, "failed\tretry") {
		t.Errorf("a nothing-case matched and still held:\n%s", stdout)
	}
}

// TestAMalformedCaseFileIsRefused keeps a file nobody can grade from grading as a
// pass. The contradiction is the interesting one: a case expecting titles *and*
// declaring nothing would otherwise mean whichever the implementation checked first.
func TestAMalformedCaseFileIsRefused(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	withCases(t, dir, `
[[case]]
query   = "retry"
titles  = ["Retry Budget"]
nothing = true
why     = "a contradiction"
`)

	_, stderr, err := run(t, "--bundle", dir, "search", "--cases")
	if err == nil {
		t.Fatal("a contradictory case file was graded")
	}
	if !strings.Contains(stderr, "cannot require both") {
		t.Errorf("the refusal does not say what is wrong: %s", stderr)
	}
}

// TestCasesTakesNoQuery keeps the two modes from being given at once, where the query
// would be silently ignored.
func TestCasesTakesNoQuery(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	_, stderr, err := run(t, "--bundle", dir, "search", "--cases", "retry")
	if err == nil {
		t.Fatal("a query was accepted alongside --cases")
	}
	if !strings.Contains(stderr, "takes no query") {
		t.Errorf("the usage error does not say why: %s", stderr)
	}
}
