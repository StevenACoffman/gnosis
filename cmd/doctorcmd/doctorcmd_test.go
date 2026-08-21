package doctorcmd_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/cmd/doctorcmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
)

// record is the envelope with doctor's payload decoded into place. Spelled out
// rather than embedding root.Outcome so every field carries its own tag.
type record struct {
	Status  root.Status      `json:"status"`
	Code    int              `json:"code"`
	Reason  string           `json:"reason"`
	Message string           `json:"message"`
	Data    doctorcmd.Result `json:"data"`
}

// run invokes the dispatcher directly with injected I/O, which is the seam
// rules.md §7 describes for exercising a command without a subprocess.
func run(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(t.Context(), args, strings.NewReader(""), &out, &errOut)
	return out.String(), err
}

// doctorOn runs doctor under --jsonl and decodes the envelope and payload.
func doctorOn(t *testing.T, dir string) record {
	t.Helper()
	stdout, _ := run(t, "--bundle", dir, "--jsonl", "doctor")

	var got record
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON line: %v\n%s", err, stdout)
	}
	return got
}

// TestInitialisedBundleIsHealthy is the pairing that matters: whatever `init`
// writes, `doctor` must approve. If these two ever disagree, one of them is
// lying about what a correct bundle looks like.
func TestInitialisedBundleIsHealthy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	got := doctorOn(t, dir)
	if got.Status != root.StatusOK {
		t.Errorf("status = %q, want %q; findings: %+v",
			got.Status, root.StatusOK, got.Data.Diagnostics)
	}
	if len(got.Data.Diagnostics) != 0 {
		t.Errorf("a freshly initialised bundle has %d finding(s): %+v",
			len(got.Data.Diagnostics), got.Data.Diagnostics)
	}
	if got.Data.Environment.Types == 0 {
		t.Error("doctor reports no types after init seeded five")
	}
}

// TestUninitialisedBundleBlocksOnTheVocabulary: an empty directory is not a
// knowledge base, and the finding must say which command fixes it.
func TestUninitialisedBundleBlocksOnTheVocabulary(t *testing.T) {
	t.Parallel()

	got := doctorOn(t, t.TempDir())
	if got.Status != root.StatusFindings {
		t.Errorf("status = %q, want %q", got.Status, root.StatusFindings)
	}
	if got.Reason != root.ReasonVocabularyInvalid {
		t.Errorf("reason = %q, want %q", got.Reason, root.ReasonVocabularyInvalid)
	}

	var mentionsInit bool
	for _, d := range got.Data.Diagnostics {
		if strings.Contains(d.Message, "gnosis init") {
			mentionsInit = true
		}
	}
	if !mentionsInit {
		t.Errorf("no finding names the command that fixes this: %+v", got.Data.Diagnostics)
	}
}

// TestBrokenVocabularyIsDiagnosedNotFatal: doctor exists for exactly the state
// where other commands cannot work. Exiting with a tool error here would leave a
// caller with no diagnosis at the moment it needs one.
func TestBrokenVocabularyIsDiagnosedNotFatal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	broken := "version = 1\n\n[[types]]\nkey = \"Rule\"\nnormatve = true\n"
	if err := os.WriteFile(filepath.Join(dir, "ontology.toml"), []byte(broken), 0o600); err != nil {
		t.Fatalf("write a broken vocabulary: %v", err)
	}

	got := doctorOn(t, dir)
	if got.Status != root.StatusFindings {
		t.Fatalf("status = %q, want %q — a diagnosable problem is a finding",
			got.Status, root.StatusFindings)
	}

	// The typo itself must appear: naming it is the difference between a report
	// and a shrug.
	var namesTheTypo bool
	for _, d := range got.Data.Diagnostics {
		if strings.Contains(d.Message, "normatve") {
			namesTheTypo = true
		}
	}
	if !namesTheTypo {
		t.Errorf("no finding names the mistyped key: %+v", got.Data.Diagnostics)
	}
}

// TestFindingsExitThreeNotOne keeps the codes distinguishable for a CI job: a
// misconfigured bundle is not a broken tool.
func TestFindingsExitThreeNotOne(t *testing.T) {
	t.Parallel()

	_, err := run(t, "--bundle", t.TempDir(), "doctor")

	var exitErr root.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a root.ExitError", err)
	}
	if root.Code(exitErr) != root.CodeFindings {
		t.Errorf("exit code = %d, want %d", exitErr, root.CodeFindings)
	}
}

// TestStaleIndexIsReportedAfterADocumentAppears: the drift warning is the one a
// reader will see most often, so it has to name both numbers.
func TestStaleIndexIsReportedAfterADocumentAppears(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	const id = "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"
	doc := "---\ntype: Reference\ngnosis_id: " + id + "\ntitle: Alpha\n---\nbody\n"
	if err := os.WriteFile(
		filepath.Join(dir, "c", id+"-alpha.md"),
		[]byte(doc),
		0o600,
	); err != nil {
		t.Fatalf("write a concept: %v", err)
	}

	got := doctorOn(t, dir)
	var reported bool
	for _, d := range got.Data.Diagnostics {
		if d.Category == "index" && strings.Contains(d.Message, "index rebuild") {
			reported = true
		}
	}
	if !reported {
		t.Errorf("a document outside the index was not reported: %+v", got.Data.Diagnostics)
	}

	// And rebuilding must clear it, which is what makes the advice worth giving.
	if _, err := run(t, "--bundle", dir, "index", "rebuild"); err != nil {
		t.Fatalf("index rebuild: %v", err)
	}
	after := doctorOn(t, dir)
	if after.Status != root.StatusOK {
		t.Errorf("after rebuild status = %q, want %q; findings: %+v",
			after.Status, root.StatusOK, after.Data.Diagnostics)
	}
}
