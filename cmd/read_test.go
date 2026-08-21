package cmd_test

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

// The read commands — search, show, graph — are tested together and at the
// dispatcher rather than one package at a time. They share a corpus, a set of
// helpers, and one integration point, and three copies of the same fixture is how
// three copies of the same subtle bug survive.

const (
	alpha = "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"
	beta  = "01932b7c-2a03-7b11-8e44-9f10c2d3e4f5"
	// absent is well-formed and belongs to nothing, which is a different failure
	// from a reference that is not an identifier at all.
	absent = "01932b7c-0000-7000-8000-000000000000"
)

// run invokes the dispatcher with injected I/O, the seam rules.md §7 describes.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(t.Context(), args, strings.NewReader(""), &out, &errOut)
	return out.String(), errOut.String(), err
}

// corpus builds a two-document bundle where alpha links to beta, and indexes it.
// The shape is the smallest one that exercises both link directions.
func corpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	write := func(id, title, body string) {
		t.Helper()
		doc := "---\ntype: Reference\ngnosis_id: " + id + "\ntitle: " + title + "\n---\n" + body
		name := filepath.Join(
			dir,
			"c",
			id+"-"+strings.ToLower(strings.ReplaceAll(title, " ", "-"))+".md",
		)
		if err := os.WriteFile(name, []byte(doc), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write(alpha, "Retry budget",
		"The service retries three times. See [Timeout](/c/"+beta+"-timeout-policy.md).\n")
	write(beta, "Timeout policy", "Every outbound request has a deadline.\n")

	if _, _, err := run(t, "--bundle", dir, "index", "rebuild"); err != nil {
		t.Fatalf("index rebuild: %v", err)
	}
	return dir
}

// decode reads the envelope's data payload into v.
func decode(t *testing.T, stdout string, v any) {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", err, stdout)
	}
	if env.Status != root.StatusOK {
		t.Fatalf("status = %q, want %q", env.Status, root.StatusOK)
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		t.Fatalf("decode payload: %v\n%s", err, env.Data)
	}
}

// exitCode extracts the code from an ExitError, failing if the error is not one.
func exitCode(t *testing.T, err error) int {
	t.Helper()
	var exitErr root.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a root.ExitError", err)
	}
	return int(exitErr)
}

// TestSearchCarriesResolvedLinks is SPEC §8.3's requirement, and the reason it is
// a requirement: a result that forces a second query to follow a link reproduces
// the defect associative indexing was invented to fix.
func TestSearchCarriesResolvedLinks(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	stdout, _, err := run(t, "--bundle", dir, "--jsonl", "search", "retries")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var got struct {
		Hits []struct {
			Title    string `json:"title"`
			Outbound []struct {
				TargetID string `json:"target_id"`
				Title    string `json:"title"`
			} `json:"outbound"`
		} `json:"hits"`
	}
	decode(t, stdout, &got)

	if len(got.Hits) != 1 {
		t.Fatalf("got %d hits, want 1", len(got.Hits))
	}
	if len(got.Hits[0].Outbound) != 1 {
		t.Fatalf("hit carries %d outbound links, want 1", len(got.Hits[0].Outbound))
	}
	if title := got.Hits[0].Outbound[0].Title; title != "Timeout policy" {
		t.Errorf("outbound link title = %q; the target was not resolved", title)
	}
}

// TestMalformedQueryIsUsageNotFailure: FTS5 syntax belongs to the caller, so
// getting it wrong is "call me differently" (2) and not "something is broken" (1).
func TestMalformedQueryIsUsageNotFailure(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	_, stderr, err := run(t, "--bundle", dir, "search", "retries AND (")
	if code := exitCode(t, err); code != root.CodeUsage {
		t.Errorf("exit code = %d, want %d", code, root.CodeUsage)
	}
	if !strings.Contains(stderr, "syntax") {
		t.Errorf("stderr does not mention the syntax problem:\n%s", stderr)
	}
}

// TestStalePathStillResolves is the property the whole identity design exists to
// buy (SPEC §5.1.1). A link written before a retitle keeps working because the
// identifier is parsed out of the filename and matched on — no mapping table, no
// redirect, no lookup by path.
func TestStalePathStillResolves(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	for _, ref := range []string{
		"c/" + alpha + "-retry-budget.md",                   // current
		"/c/" + alpha + "-whatever-it-was-called-before.md", // stale slug
		alpha, // bare identifier
	} {
		stdout, _, err := run(t, "--bundle", dir, "--jsonl", "show", ref)
		if err != nil {
			t.Errorf("show %q: %v", ref, err)
			continue
		}
		var got struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		decode(t, stdout, &got)
		if got.Title != "Retry budget" {
			t.Errorf("show %q resolved to %q, want Retry budget", ref, got.Title)
		}
	}
}

// TestAbsentAndMalformedReferencesDiffer keeps two failures distinguishable: one
// says the corpus does not hold it, the other says that is not a reference. A
// caller retries the second with different input and never the first.
func TestAbsentAndMalformedReferencesDiffer(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	_, _, err := run(t, "--bundle", dir, "show", absent)
	if code := exitCode(t, err); code != root.CodeError {
		t.Errorf("absent document exit code = %d, want %d", code, root.CodeError)
	}

	_, _, err = run(t, "--bundle", dir, "show", "not-an-identifier")
	if code := exitCode(t, err); code != root.CodeUsage {
		t.Errorf("malformed reference exit code = %d, want %d", code, root.CodeUsage)
	}
}

// TestShowDoesNotPrintUsageOnAFailure guards a regression that was live: the
// dispatcher prints command help for any error it does not recognise as an
// ExitError, so a missing document once answered with the full flag list and
// buried the one sentence that explained it.
func TestShowDoesNotPrintUsageOnAFailure(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	_, stderr, _ := run(t, "--bundle", dir, "show", absent)
	if strings.Contains(stderr, "FLAGS") || strings.Contains(stderr, "USAGE") {
		t.Errorf("a missing document produced usage help:\n%s", stderr)
	}
	if !strings.Contains(stderr, absent) {
		t.Errorf("stderr does not name the identifier that was not found:\n%s", stderr)
	}
}

// TestInboundNamesTheSource: on an inbound link the href points back at the
// document being shown, which the reader is already looking at. What they need is
// where it came from.
func TestInboundNamesTheSource(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	stdout, _, err := run(t, "--bundle", dir, "show", beta)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "Retry budget") {
		t.Errorf("inbound links do not name the source document:\n%s", stdout)
	}
}

// TestGraphIsDeterministic pins SPEC §18.3 for this surface. A graph that
// reordered itself between runs could not be diffed, and diffing is most of what
// looking at it twice is for.
func TestGraphIsDeterministic(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	first, _, err := run(t, "--bundle", dir, "graph", "--dot")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	for range 5 {
		again, _, err := run(t, "--bundle", dir, "graph", "--dot")
		if err != nil {
			t.Fatalf("graph: %v", err)
		}
		if again != first {
			t.Fatalf("output varies between runs:\n%s\n---\n%s", first, again)
		}
	}
}

// TestOrphansAreTheUnreachable: alpha links to beta, so beta is reachable and
// alpha is not. An orphan is not malformed — it is unreachable, which is why this
// reports rather than fails.
func TestOrphansAreTheUnreachable(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	stdout, _, err := run(t, "--bundle", dir, "--jsonl", "graph", "--orphans")
	if err != nil {
		t.Fatalf("graph --orphans: %v", err)
	}
	var got struct {
		Orphans []struct {
			ID string `json:"id"`
		} `json:"orphans"`
	}
	decode(t, stdout, &got)

	if len(got.Orphans) != 1 || got.Orphans[0].ID != alpha {
		t.Errorf("orphans = %+v, want exactly the document nothing links to", got.Orphans)
	}
}
