package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
)

// fixedClock is why Coordinator.Now is a field. An audit row's whole value is the
// time on it, and a value the tests cannot pin is a value the tests do not check.
func fixedClock() func() time.Time {
	at := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	return func() time.Time { return at }
}

func TestAppendAndReadTheTrail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, op := range []audit.Op{audit.OpFetch, audit.OpAdmit, audit.OpPromote} {
		err := bundle.Audit(dir, &audit.Row{
			At: fixedClock()(), Op: op, Actor: "human:priya",
		})
		if err != nil {
			t.Fatalf("append %s: %v", op, err)
		}
	}

	got, err := bundle.AuditTrail(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	// Oldest first: a trail read in write order is the only order that lets a
	// reader follow what happened.
	if got[0].Op != audit.OpFetch || got[2].Op != audit.OpPromote {
		t.Errorf("rows are out of order: %v, %v, %v", got[0].Op, got[1].Op, got[2].Op)
	}
}

// TestAnAbsentTrailIsNotAnError, and is empty rather than nil, so a caller need
// not distinguish "no writes yet" from "no result".
func TestAnAbsentTrailIsNotAnError(t *testing.T) {
	t.Parallel()
	got, err := bundle.AuditTrail(t.TempDir())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil {
		t.Error("an absent trail returned nil")
	}
	if len(got) != 0 {
		t.Errorf("got %d rows from an absent trail", len(got))
	}
}

// TestAMalformedLineIsAnError, not a skip. A trail that quietly drops what it
// cannot read cannot be counted, and counting is most of what one is for.
func TestAMalformedLineIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := bundle.Audit(dir, &audit.Row{
		At: fixedClock()(), Op: audit.OpFetch, Actor: "human:priya",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	path := filepath.Join(dir, ".gnosis", "audit.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err = f.WriteString("{not json\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err = bundle.AuditTrail(dir); err == nil {
		t.Error("a corrupt trail read cleanly")
	}
}

// TestAnInvalidRowIsNotWritten, so a trail never contains a row nobody could be
// asked about.
func TestAnInvalidRowIsNotWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := bundle.Audit(dir, &audit.Row{Op: audit.OpFetch}); err == nil {
		t.Fatal("a row with no actor and no time was written")
	}
	got, err := bundle.AuditTrail(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("the rejected row reached the trail: %+v", got)
	}
}

// TestAPromotionIsRecordedEvenWhenRefused. "We declined to promote this eleven
// times" is a fact about the corpus that a successful-writes-only trail would not
// hold, and it is the fact most worth having when somebody asks why a document
// never landed.
func TestAPromotionIsRecordedEvenWhenRefused(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	c := bundle.Coordinator{Dir: dir, Now: fixedClock()}
	if _, err := c.Execute(t.Context(), promoteCmd(command.EffectApply)); err != nil {
		t.Fatalf("execute: %v", err)
	}

	got, err := bundle.AuditTrail(dir)
	if err != nil {
		t.Fatalf("read trail: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want the refusal", len(got))
	}
	if got[0].Op != audit.OpPromote {
		t.Errorf("op = %q", got[0].Op)
	}
	if got[0].Outcome != "blocked" {
		t.Errorf("outcome = %q, want blocked", got[0].Outcome)
	}
	if got[0].Actor != "human:priya" {
		t.Errorf("actor = %q", got[0].Actor)
	}
	// The clock is the one that was injected, exactly.
	if !got[0].At.Equal(fixedClock()()) {
		t.Errorf("at = %v, want the injected time", got[0].At)
	}
}

// TestTheTrailIsPerUserState: it lives under .gnosis/, which init gitignores, so
// two colleagues at one commit hold different trails and are both right.
func TestTheTrailIsPerUserState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := bundle.Audit(dir, &audit.Row{
		At: fixedClock()(), Op: audit.OpFetch, Actor: "human:priya",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	var outside []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".gnosis":
			return filepath.SkipDir
		case d.IsDir():
			return nil
		}
		if strings.Contains(d.Name(), "audit") {
			outside = append(outside, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(outside) > 0 {
		t.Errorf("the trail is outside .gnosis/ and would be committed: %v", outside)
	}
}
