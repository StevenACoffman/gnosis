package audit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/skillet/errs"
)

func row() audit.Row {
	return audit.Row{
		At:      time.Date(2026, 8, 21, 14, 30, 0, 0, time.UTC),
		Op:      audit.OpPromote,
		Actor:   "human:priya",
		Paths:   []string{"c/01932b7c-cache.md"},
		Outcome: "ok",
	}
}

func TestCanonicalIsOneLine(t *testing.T) {
	t.Parallel()
	r := row()
	got, err := r.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if n := strings.Count(string(got), "\n"); n != 1 {
		t.Errorf("%d newlines, want the single trailing one", n)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Error("no trailing newline; appending would join two rows")
	}
}

// TestTheZeroRowIsRejected: an audit trail whose empty rows look like events is
// worse than none, because it invites counting them.
func TestTheZeroRowIsRejected(t *testing.T) {
	t.Parallel()
	var r audit.Row

	err := r.Validate()
	if err == nil {
		t.Fatal("the zero Row validated")
	}
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Errorf("code = %q, want EINVALID", errs.ErrorCode(err))
	}
	for _, want := range []string{"op is unset", "actor is empty", "at is the zero time"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q omits %q", err, want)
		}
	}
	if _, cErr := r.Canonical(); cErr == nil {
		t.Error("an invalid row rendered a line")
	}
}

// TestAnActorlessRowIsRefused is the whole point: a write nobody can be asked
// about is the case this trail exists to prevent.
func TestAnActorlessRowIsRefused(t *testing.T) {
	t.Parallel()
	r := row()
	r.Actor = "   "
	if err := r.Validate(); err == nil {
		t.Error("a row with a blank actor validated")
	}
}

// TestARowWithNoPathsIsValid. A refused promotion touched nothing and is exactly
// the row most worth having; requiring a path would push callers into inventing
// one.
func TestARowWithNoPathsIsValid(t *testing.T) {
	t.Parallel()
	r := row()
	r.Paths = nil
	r.Outcome = "blocked"
	if err := r.Validate(); err != nil {
		t.Errorf("a refusal row was rejected: %v", err)
	}
}

// TestTheTimestampIsPresent, unlike a fetch record's. The two differ on purpose:
// a fetch record is content-addressed so a timestamp would make tier 0 grow with
// checking, and an audit row's whole job is to say when.
func TestTheTimestampIsPresent(t *testing.T) {
	t.Parallel()
	r := row()
	got, err := r.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if !strings.Contains(string(got), "2026-08-21T14:30:00Z") {
		t.Errorf("the row does not carry its time:\n%s", got)
	}
}

// TestEmptyFieldsAreOmitted, so a row for a simple operation is readable rather
// than mostly empty strings.
func TestEmptyFieldsAreOmitted(t *testing.T) {
	t.Parallel()
	r := audit.Row{At: row().At, Op: audit.OpRebuild, Actor: "human:priya"}
	got, err := r.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	for _, absent := range []string{"paths", "hash_before", "hash_after", "findings", "detail"} {
		if strings.Contains(string(got), absent) {
			t.Errorf("unset field %q was encoded:\n%s", absent, got)
		}
	}
}

// TestOpUnsetIsTheZeroValue, so a forgotten field is not silently some real
// operation.
func TestOpUnsetIsTheZeroValue(t *testing.T) {
	t.Parallel()
	var op audit.Op
	if op != audit.OpUnset {
		t.Errorf("the zero Op is %q, not unset", op)
	}
}
