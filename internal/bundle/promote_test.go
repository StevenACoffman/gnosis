package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

const (
	docID    = "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"
	docPath  = "c/" + docID + "-cache-lifetime.md"
	quoteRun = "The cache is cleared on restart"
)

// admissibleBundle builds a bundle holding tier-0 evidence for one source and a
// quarantined document citing it. Everything the gate can check passes.
func admissibleBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	const text = "Vendor documentation. " + quoteRun + ", and the cache is per-process.\n"
	// StoreEvidence writes both the archived text and its fetch record, which is
	// what the provenance signal reads.
	archivePath := writeArchive(t, dir, text)

	if _, err := bundle.Quarantine(dir, docPath, []byte(document(archivePath))); err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	return dir
}

// document renders a candidate citing archivePath for its one enforced claim.
func document(archivePath string) string {
	return "---\n" +
		"type: Reference\n" +
		"title: Cache Lifetime\n" +
		"gnosis_id: " + docID + "\n" +
		"sources:\n" +
		"  - resource: https://example.org/cache.md\n" +
		"gnosis_claims:\n" +
		"  - id: claim-1\n" +
		"    anchor: " + quoteRun + "\n" +
		"    gnosis_evidence:\n" +
		"      - " + quoteRun + "\n" +
		"    archive_paths:\n" +
		"      - " + archivePath + "\n" +
		"---\n" +
		quoteRun + " and holds nothing across sessions.\n"
}

func writeArchive(t *testing.T, dir, text string) string {
	t.Helper()
	out := archive.Decide(&archive.Candidate{
		URI: "https://example.org/cache.md", Bytes: []byte(text), Extension: ".md",
	}, archive.Gates{
		Allowlist: []string{".md"}, PerFileCap: 262144, EmbeddedPayloadCap: 8192,
	})
	if out.Record.Disposition != archive.Archived {
		t.Fatalf("the fixture source was not archived: %q", out.Record.RejectReason)
	}
	if _, err := bundle.StoreEvidence(dir, &out); err != nil {
		t.Fatalf("store evidence: %v", err)
	}
	return out.Record.ArchivePath
}

func execute(t *testing.T, dir string, cmd *command.Promote) gnosis.Outcome {
	t.Helper()
	c := bundle.Coordinator{Dir: dir}
	got, err := c.Execute(t.Context(), cmd)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return got
}

func promoteCmd(eff command.Effect) *command.Promote {
	return &command.Promote{Path: docPath, Eff: eff, Approver: "human:priya"}
}

// TestAdmissibleCandidateIsStillWithheld is the honest state of this build and is
// asserted so it is not mistaken for a defect. Every signal that can run passes;
// `security` and `conflict` cannot run, and unchecked withholds approval.
func TestAdmissibleCandidateIsStillWithheld(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	got := execute(t, dir, promoteCmd(command.EffectApply))
	if got.Status != gnosis.StatusBlocked {
		t.Fatalf("status = %q (%s), want blocked", got.Status, got.Message)
	}
	if got.Reason != gnosis.ReasonGateUnavailable {
		t.Errorf("reason = %q, want the unbuilt-signal reason", got.Reason)
	}

	data, _ := got.Data.(map[string]any)
	failed, _ := data["failed"].([]gate.Signal)
	if len(failed) != 0 {
		t.Errorf("an admissible candidate failed signals: %v", failed)
	}
	// Nothing was written, and the draft is still there to retry.
	if _, err := os.Stat(filepath.Join(dir, docPath)); err == nil {
		t.Error("a withheld promotion wrote the document anyway")
	}
	if _, err := bundle.ReadQuarantined(dir, docPath); err != nil {
		t.Errorf("a withheld promotion discarded the draft: %v", err)
	}
}

// TestAFabricatedQuotationFailsRatherThanBeingUnchecked: the distinction the
// three-valued verdict exists for. A caller must be able to tell "your document
// is wrong" from "this build cannot check that".
func TestAFabricatedQuotationFailsRatherThanBeingUnchecked(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	spoiled := strings.ReplaceAll(
		string(mustRead(t, dir, docPath)), quoteRun, "A sentence nobody ever wrote")
	if _, err := bundle.Quarantine(dir, docPath, []byte(spoiled)); err != nil {
		t.Fatalf("re-quarantine: %v", err)
	}

	got := execute(t, dir, promoteCmd(command.EffectApply))
	if got.Reason != gnosis.ReasonNeedsHuman {
		t.Errorf("reason = %q, want needs_human — a real failure, not an unbuilt check",
			got.Reason)
	}
	if !strings.Contains(got.Message, "withheld") {
		t.Errorf("message = %q", got.Message)
	}
}

// TestPreviewWritesNothing. Preview and apply are one code path down to the final
// branch, so the only way they can differ is in whether the write happens.
func TestPreviewWritesNothing(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	preview := execute(t, dir, promoteCmd(command.EffectPreview))
	apply := execute(t, dir, promoteCmd(command.EffectApply))

	if preview.Status != apply.Status || preview.Reason != apply.Reason {
		t.Errorf("preview and apply disagree: %q/%q vs %q/%q",
			preview.Status, preview.Reason, apply.Status, apply.Reason)
	}
	if _, err := os.Stat(filepath.Join(dir, docPath)); err == nil {
		t.Error("something was written")
	}
}

// TestNothingQuarantinedIsBlockedNotBroken: promoting a slug nobody admitted is a
// caller mistake about the corpus's state, not a tool failure.
func TestNothingQuarantinedIsBlockedNotBroken(t *testing.T) {
	t.Parallel()
	got := execute(t, t.TempDir(), promoteCmd(command.EffectApply))

	if got.Status != gnosis.StatusBlocked {
		t.Errorf("status = %q, want blocked", got.Status)
	}
	if !strings.Contains(got.Message, "quarantined") {
		t.Errorf("message = %q, does not say the draft is missing", got.Message)
	}
}

// TestQuarantineRefusesTraversal. A quarantined path arrives from a model's reply,
// so `../../etc/whatever` is an input this receives rather than one it guards
// against on principle.
func TestQuarantineRefusesTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, bad := range []string{"../escaped.md", "../../etc/passwd", "", "/absolute.md"} {
		if _, err := bundle.Quarantine(dir, bad, []byte("x")); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped.md")); err == nil {
		t.Error("a traversal escaped the bundle")
	}
}

// TestQuarantineIsNotInTheBundle is §3083's decided constraint: unvetted text is
// text an agent will obey, and a coding agent walking the repository does not know
// about --include-quarantine.
func TestQuarantineIsNotInTheBundle(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	var strays []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".gnosis":
			return filepath.SkipDir
		case d.IsDir():
			return nil
		}
		if strings.Contains(string(mustReadFile(t, path)), "Cache Lifetime") {
			strays = append(strays, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(strays) > 0 {
		t.Errorf("quarantined content is visible outside .gnosis/: %v", strays)
	}
}

// TestQuarantinedListsWhatIsWaiting, sorted, because a review queue whose order
// changed between runs would be unusable.
func TestQuarantinedListsWhatIsWaiting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, p := range []string{"c/b.md", "c/a.md", "c/c.md"} {
		if _, err := bundle.Quarantine(dir, p, []byte("x")); err != nil {
			t.Fatalf("quarantine %s: %v", p, err)
		}
	}
	got, err := bundle.Quarantined(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"c/a.md", "c/b.md", "c/c.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestQuarantinedIsEmptyNotNil for a bundle with none, so a caller need not
// distinguish "nothing waiting" from "no result".
func TestQuarantinedIsEmptyNotNil(t *testing.T) {
	t.Parallel()
	got, err := bundle.Quarantined(t.TempDir())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got == nil {
		t.Error("an empty quarantine returned nil")
	}
}

func mustRead(t *testing.T, dir, rel string) []byte {
	t.Helper()
	data, err := bundle.ReadQuarantined(dir, rel)
	if err != nil {
		t.Fatalf("read quarantined: %v", err)
	}
	return data
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
