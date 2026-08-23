package indexcmd_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
)

const validID = "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"

// corpusSize is six because it is the smallest count where "one deleted" is
// clearly under a half floor and "all deleted" is clearly over it — the two cases
// these tests turn on.
const corpusSize = 6

func run(t *testing.T, args ...string) (stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(t.Context(), args, strings.NewReader(""), &out, &errOut)
	return errOut.String(), err
}

// corpusOf writes corpusSize documents and builds an index over them.
func corpusOf(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for i := range corpusSize {
		id := validID[:len(validID)-1] + string(rune('a'+i))
		body := "---\ntype: Reference\ntitle: Doc " + string(rune('A'+i)) +
			"\ngnosis_id: " + id + "\n---\nbody\n"
		name := filepath.Join(dir, "c", id+"-doc.md")
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if _, err := run(t, "--bundle", dir, "index", "rebuild"); err != nil {
		t.Fatalf("initial rebuild: %v", err)
	}
	return dir
}

// TestARebuildThatLosesTheCorpusRefuses is the whole point. Being a cache is what
// makes the index safe to destroy and also what makes destroying it unnoticeable.
func TestARebuildThatLosesTheCorpusRefuses(t *testing.T) {
	t.Parallel()
	dir := corpusOf(t)

	// The accident: c/ is gone, as it would be from a wrong --bundle or a clone
	// that did not include it.
	if err := os.RemoveAll(filepath.Join(dir, "c")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	stderr, err := run(t, "--bundle", dir, "index", "rebuild")
	if err == nil {
		t.Fatal("a rebuild that lost the whole corpus succeeded")
	}

	var exitErr root.ExitError
	if !errors.As(err, &exitErr) || root.Code(exitErr) != root.CodeBlocked {
		t.Errorf("exit = %v, want blocked", err)
	}
	// Both counts, because the number that matters is the one the reader did not
	// expect.
	for _, want := range []string{strconv.Itoa(corpusSize) + " documents", "to 0", "--force"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q omits %q", stderr, want)
		}
	}
}

// TestTheRefusalLeavesTheIndexIntact is the property, as distinct from the report.
// A refusal that had already written would be a message about something that
// already happened.
func TestTheRefusalLeavesTheIndexIntact(t *testing.T) {
	t.Parallel()
	dir := corpusOf(t)

	before, err := os.ReadFile(filepath.Join(dir, ".gnosis", "index.db"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if err = os.RemoveAll(filepath.Join(dir, "c")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err = run(t, "--bundle", dir, "index", "rebuild"); err == nil {
		t.Fatal("expected a refusal")
	}

	after, err := os.ReadFile(filepath.Join(dir, ".gnosis", "index.db"))
	if err != nil {
		t.Fatalf("read index after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the refusal changed the index it was refusing to overwrite")
	}
}

// TestForceProceeds, because a real deletion is legitimate and the floor cannot
// tell one from an accident. What it buys is that the accident takes a second
// command rather than none.
func TestForceProceeds(t *testing.T) {
	t.Parallel()
	dir := corpusOf(t)
	if err := os.RemoveAll(filepath.Join(dir, "c")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if stderr, err := run(t, "--bundle", dir, "index", "rebuild", "--force"); err != nil {
		t.Fatalf("--force was refused: %v\n%s", err, stderr)
	}
}

// TestAFirstRebuildIsNotAFall. A fresh bundle has no prior count, and a floor that
// fired here would break the ordinary path to protect the rare one.
func TestAFirstRebuildIsNotAFall(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "--bundle", t.TempDir(), "index", "rebuild"); err != nil {
		t.Fatalf("the first rebuild of an empty bundle was refused: %v", err)
	}
}

// TestOrdinaryEditingIsNotRefused: deleting one document of six is under the
// floor and must pass, or the check is a nuisance rather than a guard.
func TestOrdinaryEditingIsNotRefused(t *testing.T) {
	t.Parallel()
	dir := corpusOf(t)

	entries, err := os.ReadDir(filepath.Join(dir, "c"))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if err = os.Remove(filepath.Join(dir, "c", entries[0].Name())); err != nil {
		t.Fatalf("remove one: %v", err)
	}

	if stderr, rerr := run(t, "--bundle", dir, "index", "rebuild"); rerr != nil {
		t.Fatalf("deleting one document of six was refused: %v\n%s", rerr, stderr)
	}
}

// TestCheckIsNotBlocked. --check writes nothing, so there is nothing for the floor
// to protect and refusing would stop a CI job from reporting the very drift it was
// run to find.
func TestCheckIsNotBlocked(t *testing.T) {
	t.Parallel()
	dir := corpusOf(t)
	if err := os.RemoveAll(filepath.Join(dir, "c")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, err := run(t, "--bundle", dir, "index", "rebuild", "--check")
	var exitErr root.ExitError
	if errors.As(err, &exitErr) && root.Code(exitErr) == root.CodeBlocked {
		t.Error("--check was blocked by the rebuild floor")
	}
}

// TestARebuildIsAudited. §15 audits every mutation, and a rebuild is one: it
// replaces the derived tables wholesale.
func TestARebuildIsAudited(t *testing.T) {
	t.Parallel()
	dir := corpusOf(t)

	body, err := os.ReadFile(filepath.Join(dir, ".gnosis", "audit.jsonl"))
	if err != nil {
		t.Fatalf("no audit trail after a rebuild: %v", err)
	}
	trail := string(body)

	if !strings.Contains(trail, `"op":"rebuild"`) {
		t.Errorf("the trail omits the rebuild:\n%s", trail)
	}
	// The actor is a check rather than a person: the tool caused the write, and
	// §5.5 holds that a check name is as much an answer to "who says so" as an
	// actor is.
	if !strings.Contains(trail, `"actor":"check:index-rebuild"`) {
		t.Errorf("the trail omits the actor:\n%s", trail)
	}
	if !strings.Contains(trail, strconv.Itoa(corpusSize)+" documents") {
		t.Errorf("the rebuild row does not say what it indexed:\n%s", trail)
	}
}

// TestARepeatedInitIsStillAudited. "Somebody ran init here and it was already
// initialised" is a fact about this machine, and a trail holding only successful
// creations would make a repeated init look like it never happened.
func TestARepeatedInitIsStillAudited(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for range 2 {
		if _, err := run(t, "--bundle", dir, "init"); err != nil {
			t.Fatalf("init: %v", err)
		}
	}

	body, err := os.ReadFile(filepath.Join(dir, ".gnosis", "audit.jsonl"))
	if err != nil {
		t.Fatalf("read trail: %v", err)
	}
	if n := strings.Count(string(body), `"op":"init"`); n != 2 {
		t.Errorf("got %d init rows after two inits:\n%s", n, body)
	}
	if !strings.Contains(string(body), "already present") {
		t.Errorf("the second row does not say nothing was created:\n%s", body)
	}
}
