package bundle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
)

// aRow is the row every case here verifies against.
func aRow() *audit.Row {
	return &audit.Row{At: fixedClock()(), Op: audit.OpPromote, Actor: "human:priya"}
}

// trailAt writes exactly these bytes where the trail belongs.
func trailAt(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, ".gnosis", "audit.jsonl")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if contents != "" {
		if err := os.WriteFile(full, []byte(contents), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

// canonical is the row as Audit would write it.
func canonical(t *testing.T, row *audit.Row) string {
	t.Helper()
	b, err := row.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	return string(b)
}

func TestVerifyAudit(t *testing.T) {
	t.Parallel()
	row := aRow()
	line := canonical(t, row)
	other := canonical(t, &audit.Row{
		At: fixedClock()(), Op: audit.OpFetch, Actor: "agent:test",
	})

	cases := map[string]struct {
		contents string
		wantErr  bool
		says     string
	}{
		"the row is the only line": {line, false, ""},
		"the row is the last line": {other + line, false, ""},
		// The observed failure: the append said yes and wrote nothing.
		"the trail is empty":        {"", true, "empty"},
		"the trail has no such row": {other, true, "last line"},
		// A row that landed and was then appended over means somebody wrote
		// without the lock, which is a different fault from a lost row.
		"the row is not last": {line + other, true, "last line"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := bundle.VerifyAuditForTest(trailAt(t, tc.contents), row)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got %v, want error: %v", err, tc.wantErr)
			}
			if err == nil {
				return
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the error %q omits %q", err, tc.says)
			}
			// §15 keeps corruption apart from an operational failure. A row that
			// was written and is not there is the file being wrong, not the disk.
			if !strings.Contains(err.Error(), "corruption") {
				t.Errorf("the error does not name this as corruption: %v", err)
			}
		})
	}
}

// TestVerificationIgnoresOldDamage. The tail read is bounded, and the reason is
// not only cost: parsing the whole trail would let a corrupt line from six months
// ago fail a write that is perfectly correct today.
func TestVerificationIgnoresOldDamage(t *testing.T) {
	t.Parallel()
	row := aRow()
	dir := trailAt(t, "{not json at all\n"+canonical(t, row))

	if err := bundle.VerifyAuditForTest(dir, row); err != nil {
		t.Errorf("an unrelated malformed line failed the verification: %v", err)
	}
}

// TestTheTailIsBounded. A file larger than the window still verifies, because only
// the end of it is read.
func TestTheTailIsBounded(t *testing.T) {
	t.Parallel()
	row := aRow()
	filler := strings.Repeat(canonical(t, &audit.Row{
		At: fixedClock()(), Op: audit.OpFetch, Actor: "agent:filler",
	}), 2000)
	dir := trailAt(t, filler+canonical(t, row))

	if err := bundle.VerifyAuditForTest(dir, row); err != nil {
		t.Errorf("a long trail failed verification: %v", err)
	}
}

// TestReadTailOfAnAbsentFileIsNotAnError, because for the trail that state means
// the append wrote nothing — a finding for the caller, not a read failure.
func TestReadTailOfAnAbsentFileIsNotAnError(t *testing.T) {
	t.Parallel()
	got, err := bundle.ReadTailForTest(filepath.Join(t.TempDir(), "nope"), 1024)
	if err != nil {
		t.Fatalf("an absent file was a read error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d bytes from an absent file", len(got))
	}
}

// TestAVerifiedMutationReportsNoError is the ordinary path, driven through the
// coordinator so the wiring is covered and not only the verifier.
func TestAVerifiedMutationReportsNoError(t *testing.T) {
	t.Parallel()
	dir := admissibleBundle(t)

	c := bundle.Coordinator{Dir: dir, Now: fixedClock()}
	if _, err := c.Execute(t.Context(), promoteCmd(command.EffectApply)); err != nil {
		t.Fatalf("a mutation whose row was written reported an error: %v", err)
	}
}

// TestAuditLostSeparatesTheTwoFailures is the predicate Step 0's whole resolution
// rests on. An append that reported failure is a known gap a caller may warn about
// and carry on from; a row the append claimed to write and that is not there is the
// one failure no other signal reveals, and it must not be handled the same way.
func TestAuditLostSeparatesTheTwoFailures(t *testing.T) {
	t.Parallel()

	t.Run("an append that failed is not a lost row", func(t *testing.T) {
		t.Parallel()
		// A directory where the trail belongs: the append fails outright.
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".gnosis", "audit.jsonl"), 0o750); err != nil {
			t.Fatalf("wedge: %v", err)
		}
		err := writerFor(t, dir).Audit(aRow())
		if err == nil {
			t.Skip("this platform appended to a directory")
		}
		if bundle.AuditLost(err) {
			t.Errorf("a failed append was reported as a lost row: %v", err)
		}
	})

	t.Run("a nil error is not a lost row", func(t *testing.T) {
		t.Parallel()
		if bundle.AuditLost(nil) {
			t.Error("nil was reported as a lost row")
		}
	})

	t.Run("a successful append is not a lost row", func(t *testing.T) {
		t.Parallel()
		if err := writerFor(t, t.TempDir()).Audit(aRow()); err != nil {
			t.Errorf("an ordinary append failed: %v", err)
		}
	})
}

// TestAuditVerifiedWritesTheRow. The verification must not be the only thing that
// happens: this is an append that also checks, not a check that forgot to append.
func TestAuditVerifiedWritesTheRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writerFor(t, dir).Audit(aRow()); err != nil {
		t.Fatalf("append: %v", err)
	}
	trail, err := bundle.AuditTrail(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(trail.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(trail.Rows))
	}
	if trail.Rows[0].Op != audit.OpPromote {
		t.Errorf("op = %q", trail.Rows[0].Op)
	}
}
