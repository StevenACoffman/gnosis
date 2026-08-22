package bundle_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/audit"
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

// TestAdmissibleCandidateAsksForAHuman is the honest state of this build. Every
// signal that can run passes; `conflict` cannot run at all and `security` ran one
// §9.3 stage of four. That withholds *automatic* approval and no longer withholds
// promotion outright — the difference between the two is the whole of §9.5's human
// path, and an earlier version of this test asserted the deadlock as correct.
func TestAdmissibleCandidateAsksForAHuman(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	got := execute(t, dir, promoteCmd(command.EffectApply))
	if got.Status != gnosis.StatusBlocked {
		t.Fatalf("status = %q (%s), want blocked", got.Status, got.Message)
	}
	if got.Reason != gnosis.ReasonNeedsHuman {
		t.Errorf("reason = %q, want needs_human", got.Reason)
	}

	data, _ := got.Data.(map[string]any)
	unchecked, _ := data["unchecked"].([]gate.Signal)
	if len(unchecked) == 0 {
		t.Error("the escalation does not name what could not be checked")
	}
	// The message must say what to supply. An escalation a caller cannot act on
	// is a deadlock with better wording.
	for _, want := range []string{"confirm", docPath, "why"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("the message omits %q: %s", want, got.Message)
		}
	}
	// Nothing was written, and the draft is still there to retry.
	if _, err := os.Stat(filepath.Join(dir, docPath)); err == nil {
		t.Error("a withheld promotion wrote the document anyway")
	}
	if _, err := bundle.ReadQuarantined(dir, docPath); err != nil {
		t.Errorf("a withheld promotion discarded the draft: %v", err)
	}
}

// TestAHumanCanCarryTheUnrunSignals is the first promotion that succeeds. Until
// the decision existed, no candidate could reach the corpus at all.
func TestAHumanCanCarryTheUnrunSignals(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	cmd := promoteCmd(command.EffectApply)
	cmd.Confirmation = docPath
	cmd.Rationale = "reviewed the source by hand; §10 is unbuilt and this cites one page"

	got := execute(t, dir, cmd)
	if got.Status != gnosis.StatusOK {
		t.Fatalf("status = %q (%s), want ok", got.Status, got.Message)
	}
	if _, err := os.Stat(filepath.Join(dir, docPath)); err != nil {
		t.Errorf("the document was not written: %v", err)
	}
	if _, err := bundle.ReadQuarantined(dir, docPath); err == nil {
		t.Error("the draft survived a successful promotion")
	}
}

// TestTheDebtIsRecorded is what separates this design from a `--force`. A trail
// saying only "a human approved it" cannot answer the question that matters when
// §10 lands: which claims were admitted with no conflict check. One naming the
// signals can, and every such document is then one query away.
func TestTheDebtIsRecorded(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	cmd := promoteCmd(command.EffectApply)
	cmd.Confirmation = docPath
	cmd.Rationale = "checked by hand"
	if got := execute(t, dir, cmd); got.Status != gnosis.StatusOK {
		t.Fatalf("promotion did not succeed: %s", got.Message)
	}

	rows, err := bundle.AuditTrail(dir)
	if err != nil {
		t.Fatalf("read the trail: %v", err)
	}
	var found bool
	for i := range rows {
		row := &rows[i]
		if row.Op != audit.OpPromote || row.Outcome != string(gnosis.StatusOK) {
			continue
		}
		found = true
		if len(row.Signals) == 0 {
			t.Error("the row does not name the signals the promotion was carried over")
		}
		if !slices.Contains(row.Signals, string(gate.SignalConflict)) {
			t.Errorf("Signals = %v, want conflict among them", row.Signals)
		}
		if !strings.Contains(row.Detail, cmd.Rationale) {
			t.Errorf("the row does not carry the rationale: %q", row.Detail)
		}
	}
	if !found {
		t.Error("no successful promotion row in the trail")
	}
}

// TestWhatAHumanMayNotDo. Each case is a way the escalation could be turned back
// into the bypass §15 forbids, and the refusal must name what is missing rather
// than reporting a generic block.
func TestWhatAHumanMayNotDo(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edit func(*command.Promote)
		says string
	}{
		"an agent granting its own promotion": {
			func(c *command.Promote) { c.Approver = "agent:claude" }, "must be a person",
		},
		"confirming with anything but the path": {
			func(c *command.Promote) { c.Confirmation = "yes" }, "typing the document's path",
		},
		"confirming nothing at all": {
			func(c *command.Promote) { c.Confirmation = "" }, "typing the document's path",
		},
		"carrying no reason": {
			func(c *command.Promote) { c.Rationale = "" }, "state why",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := admissibleBundle(t)
			cmd := promoteCmd(command.EffectApply)
			cmd.Confirmation, cmd.Rationale = docPath, "checked by hand"
			tc.edit(cmd)

			got := execute(t, dir, cmd)
			if got.Status != gnosis.StatusBlocked {
				t.Fatalf("status = %q, want blocked (%s)", got.Status, got.Message)
			}
			if !strings.Contains(got.Message, tc.says) {
				t.Errorf("the refusal omits %q: %s", tc.says, got.Message)
			}
			if _, err := os.Stat(filepath.Join(dir, docPath)); err == nil {
				t.Error("the document was written anyway")
			}
		})
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

// TestARefusedCandidateResistsEveryConfirmation is the assertion the whole design
// rests on. The human path is defensible only because it opens for what could not
// be checked and stays shut for what was checked and failed; if a signature could
// carry a failed signal, this would be the `--yes` bypass §15 forbids with a
// longer prompt.
//
// The candidate here duplicates a title already in the corpus, so `duplication`
// fails while everything else passes — the closest a document gets to promotable
// while still being refused.
func TestARefusedCandidateResistsEveryConfirmation(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	// Put a document with the same title in the corpus, so the candidate is a
	// duplicate of something already there.
	other := filepath.Join(dir, "c", "other.md")
	if err := os.MkdirAll(filepath.Dir(other), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := "---\ntype: Reference\ntitle: Cache Lifetime\n" +
		"gnosis_id: 01932b7c-0000-7000-8000-000000000001\n---\nOther body.\n"
	if err := os.WriteFile(other, []byte(existing), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cmd := promoteCmd(command.EffectApply)
	cmd.Confirmation = docPath // correctly typed
	cmd.Rationale = "I am certain, and I have thought about it carefully"
	cmd.Approver = "human:priya" // a real person

	got := execute(t, dir, cmd)
	if got.Status != gnosis.StatusBlocked {
		t.Fatalf("status = %q, want blocked (%s)", got.Status, got.Message)
	}
	if _, err := os.Stat(filepath.Join(dir, docPath)); err == nil {
		t.Fatal("a correctly confirmed human promoted a candidate that FAILED a signal")
	}

	data, _ := got.Data.(map[string]any)
	failed, _ := data["failed"].([]gate.Signal)
	if !slices.Contains(failed, gate.SignalDuplication) {
		t.Errorf("failed = %v, want duplication among them", failed)
	}
	// The refusal must not read like the escalation. Telling somebody to confirm
	// harder when no confirmation exists is worse than a bare refusal.
	if strings.Contains(got.Message, "confirm by typing") {
		t.Errorf("a refusal offered a confirmation that cannot work: %q", got.Message)
	}
}
