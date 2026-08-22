package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// runWithStdin is run with something to read, for the confirmation prompt. The
// shared helper supplies an empty reader, which is the right default everywhere
// else and is exactly the "declined" case here.
func runWithStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(t.Context(), args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), err
}

// waiting drives fetch → ingest → admit and returns the bundle and the one
// quarantined path. The whole pipeline rather than a hand-built fixture, because
// what is being tested is that the cycle completes.
func waiting(t *testing.T) (bundleDir, path string) {
	t.Helper()
	bundleDir, uri := relayBundle(t)
	if _, _, err := run(t, "--bundle", bundleDir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	key, _ := firstPrompt(t, ingest(t, bundleDir, uri))

	_, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"admit", "--key", key, "--submitter", "agent:test",
		answer(t, "The cache is cleared on restart and holds nothing"))
	if err != nil {
		t.Fatalf("admit: %v\n%s", err, stderr)
	}

	paths, err := bundle.Quarantined(bundleDir)
	if err != nil || len(paths) != 1 {
		t.Fatalf("want one quarantined document, got %v (%v)", paths, err)
	}
	return bundleDir, paths[0]
}

// TestQuarantineShowsWhyEachIsStuck. A list of paths says what is waiting and not
// why any of it is stuck, which is the question a reader actually has.
func TestQuarantineShowsWhyEachIsStuck(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	stdout, _, err := run(t, "--bundle", bundleDir, "quarantine")
	if err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	if !strings.Contains(stdout, path) {
		t.Errorf("the queue omits the waiting document:\n%s", stdout)
	}
	if !strings.Contains(stdout, "needs_human") {
		t.Errorf("the queue does not report the decision:\n%s", stdout)
	}
	if !strings.Contains(stdout, "conflict") {
		t.Errorf("the queue does not name the unrun signal:\n%s", stdout)
	}
}

// TestAnEmptyQueueIsNotAFinding. Nothing waiting is the state a healthy corpus is
// in most of the time.
func TestAnEmptyQueueIsNotAFinding(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := run(t, "--bundle", dir, "quarantine"); err != nil {
		t.Errorf("an empty queue reported an error: %v", err)
	}
}

// TestPreviewIsTheDefault. A command that writes unless told not to is the shape
// §9.4 argues against, and the two mistakes cost differently: a surprising preview
// wastes a second and a surprising write enters the corpus.
func TestPreviewIsTheDefault(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	_, _, _ = run(t, "--bundle", bundleDir, "promote", path)
	if _, err := os.Stat(filepath.Join(bundleDir, filepath.FromSlash(path))); err == nil {
		t.Error("a promote with no --apply wrote the document")
	}
}

// TestAPreviewDoesNotAccuseYou. Found by running the command: with no --approver,
// the preview reported that the promotion "cannot be self-granted by an agent" —
// an accusation about an action the caller had not taken. A preview asks what
// would happen; its authorisation is not yet missing.
func TestAPreviewDoesNotAccuseYou(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	_, stderr, _ := run(t, "--bundle", bundleDir, "promote", path)
	if strings.Contains(stderr, "self-granted") {
		t.Errorf("the preview accused the caller of something they did not do:\n%s", stderr)
	}
	if !strings.Contains(stderr, "would need a person") {
		t.Errorf("the preview does not say what applying would require:\n%s", stderr)
	}
}

// TestAPreviewWritesNoAuditRow. A preview is a read, and a mutation log that also
// holds reads is a log somebody stops reading.
func TestAPreviewWritesNoAuditRow(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	before, err := bundle.AuditTrail(bundleDir)
	if err != nil {
		t.Fatalf("read the trail: %v", err)
	}
	_, _, _ = run(t, "--bundle", bundleDir, "promote", path)

	after, err := bundle.AuditTrail(bundleDir)
	if err != nil {
		t.Fatalf("read the trail: %v", err)
	}
	if len(after.Rows) != len(before.Rows) {
		t.Errorf("a preview added %d audit row(s)", len(after.Rows)-len(before.Rows))
	}
}

// TestJSONLNeverPrompts is the case most likely to be got wrong. A machine caller
// cannot type a phrase, and a prompt on a pipe hangs — so the requirement goes in
// the envelope instead. The empty stdin here is what a pipe with nothing on it
// looks like, and the test would hang rather than fail if this regressed.
func TestJSONLNeverPrompts(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	stdout, _, err := runWithStdin(t, "", "--bundle", bundleDir, "--jsonl",
		"promote", "--apply", "--approver", "human:priya",
		"--rationale", "checked by hand", path)
	if err == nil {
		t.Fatal("a promotion needing a human reported success with no confirmation")
	}

	var env struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if uErr := json.Unmarshal([]byte(stdout), &env); uErr != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", uErr, stdout)
	}
	if env.Status != "blocked" || env.Reason != "needs_human" {
		t.Errorf("envelope = %+v, want blocked/needs_human", env)
	}
	if _, sErr := os.Stat(filepath.Join(bundleDir, filepath.FromSlash(path))); sErr == nil {
		t.Error("the document was written with no confirmation")
	}
}

// TestConfirmingWithThePathPromotes is the cycle completing: fetch, ingest, admit,
// promote. Nothing had ever reached the corpus before the gate could distinguish
// "could not check" from "checked and failed".
func TestConfirmingWithThePathPromotes(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	_, stderr, err := runWithStdin(t, path+"\n", "--bundle", bundleDir,
		"promote", "--apply", "--approver", "human:priya",
		"--rationale", "read the source myself", path)
	if err != nil {
		t.Fatalf("promote: %v\n%s", err, stderr)
	}
	if _, sErr := os.Stat(filepath.Join(bundleDir, filepath.FromSlash(path))); sErr != nil {
		t.Fatalf("the document was not written: %v\n%s", sErr, stderr)
	}
	// The person who authorised it is told what they took on, not only the file.
	if !strings.Contains(stderr, "conflict") {
		t.Errorf("the approver was not told which signals they carried:\n%s", stderr)
	}
}

// TestConfirmingWithAnythingElseDoesNot. "yes" is muscle memory; naming the file
// is what makes somebody look at which one it is.
func TestConfirmingWithAnythingElseDoesNot(t *testing.T) {
	t.Parallel()
	for _, typed := range []string{"yes", "y", "", "c/wrong.md"} {
		t.Run("typed "+typed, func(t *testing.T) {
			t.Parallel()
			bundleDir, path := waiting(t)

			_, _, _ = runWithStdin(t, typed+"\n", "--bundle", bundleDir,
				"promote", "--apply", "--approver", "human:priya",
				"--rationale", "checked", path)

			if _, err := os.Stat(filepath.Join(bundleDir, filepath.FromSlash(path))); err == nil {
				t.Errorf("typing %q promoted the document", typed)
			}
		})
	}
}

// TestAnAgentCannotGrantItsOwnPromotion. §9.5: no self-granted approval — and an
// agent is what produced the candidate.
func TestAnAgentCannotGrantItsOwnPromotion(t *testing.T) {
	t.Parallel()
	bundleDir, path := waiting(t)

	_, stderr, _ := runWithStdin(t, path+"\n", "--bundle", bundleDir,
		"promote", "--apply", "--approver", "agent:claude",
		"--rationale", "I checked my own work", path)

	if _, err := os.Stat(filepath.Join(bundleDir, filepath.FromSlash(path))); err == nil {
		t.Fatal("an agent promoted a document by confirming it")
	}
	if !strings.Contains(stderr, "must be a person") {
		t.Errorf("the refusal does not say why:\n%s", stderr)
	}
}
