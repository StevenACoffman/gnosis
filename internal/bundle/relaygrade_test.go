package bundle_test

import (
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// runStart is the moment a graded relay run begins.
func runStart() time.Time { return time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC) }

// gradeTrail builds a trail from rows offset from the run's start.
func gradeTrail(rows ...audit.Row) *bundle.Trail {
	return &bundle.Trail{Rows: rows}
}

// minute returns a time n minutes into the run.
func minute(n int) time.Time { return runStart().Add(time.Duration(n) * time.Minute) }

// ok is a successful row of an operation.
func ok(op audit.Op, n int) audit.Row {
	return audit.Row{At: minute(n), Op: op, Outcome: string(gnosis.StatusOK)}
}

// TestNothingElseHappenedFirstIsTheAssertionThatBites is §18.6's instructive half.
//
// A run that merely reached the required step tells you less than one that reached it
// *first*. Ordering is a property a prose reply cannot be trusted to report about
// itself, so it is read off the trail — and the allowlist is what makes "first"
// meaningful rather than a tolerance somebody tuned.
func TestNothingElseHappenedFirstIsTheAssertionThatBites(t *testing.T) {
	t.Parallel()

	trail := gradeTrail(
		ok(audit.OpFetch, 1),   // allowed: the agent had to read something
		ok(audit.OpPromote, 2), // NOT allowed: it promoted before it was admitted
		ok(audit.OpAdmit, 3),
	)
	grade := bundle.GradeRelay(trail, &bundle.RelayRun{
		Since: runStart(), Key: "k1", Allowed: []audit.Op{audit.OpFetch},
	})

	if !grade.Admitted {
		t.Error("the admission was not seen")
	}
	if len(grade.Interfered) != 1 || grade.Interfered[0] != string(audit.OpPromote) {
		t.Fatalf("interference = %v, want [promote]", grade.Interfered)
	}
	if grade.Held() {
		t.Error("a run that promoted before admitting was graded as holding")
	}
	// The report says which assertion failed rather than that one did.
	if !strings.Contains(bundle.RelayReport(grade), "promote") {
		t.Errorf("the report does not name what interfered: %s", bundle.RelayReport(grade))
	}
}

// TestWhatHappensAfterTheAdmissionIsNotGraded keeps the assertion to its own scope: the
// question is what preceded the required step, and a corpus that goes on being used is
// not thereby a failed run.
func TestWhatHappensAfterTheAdmissionIsNotGraded(t *testing.T) {
	t.Parallel()

	trail := gradeTrail(ok(audit.OpAdmit, 1), ok(audit.OpPromote, 2))
	grade := bundle.GradeRelay(trail, &bundle.RelayRun{Since: runStart(), Key: "k1"})
	if !grade.Held() {
		t.Errorf("a promotion after the admission was counted against the run: %v",
			grade.Interfered)
	}
}

// TestARefusedAdmissionIsNotAnAdmission is what §18.6 is actually asking: whether a real
// model produced a reply this corpus would *take*. A reply examined and declined answers
// that question rather than partly satisfying it.
func TestARefusedAdmissionIsNotAnAdmission(t *testing.T) {
	t.Parallel()

	trail := gradeTrail(audit.Row{
		At: minute(1), Op: audit.OpAdmit, Outcome: string(gnosis.StatusBlocked),
	})
	grade := bundle.GradeRelay(trail, &bundle.RelayRun{Since: runStart(), Key: "k1"})
	if grade.Admitted {
		t.Error("a refused admission was graded as one")
	}
	if !strings.Contains(bundle.RelayReport(grade), "no reply was admitted") {
		t.Errorf("the report does not say so: %s", bundle.RelayReport(grade))
	}
}

// TestRowsBeforeTheRunAreNotItsBusiness bounds the grade. A bundle with a history must
// be gradeable without that history counting as interference.
func TestRowsBeforeTheRunAreNotItsBusiness(t *testing.T) {
	t.Parallel()

	trail := gradeTrail(
		audit.Row{At: minute(-60), Op: audit.OpPromote, Outcome: string(gnosis.StatusOK)},
		ok(audit.OpAdmit, 1),
	)
	grade := bundle.GradeRelay(trail, &bundle.RelayRun{Since: runStart(), Key: "k1"})
	if !grade.Held() {
		t.Errorf("history before the run was counted against it: %v", grade.Interfered)
	}
	if grade.Rows != 1 {
		t.Errorf("rows = %d, want 1 — the count must describe what was examined", grade.Rows)
	}
}

// TestAnEmptyTrailFailsRatherThanPasses is the direction that matters: a run that did
// nothing must not grade clean because there was nothing to object to.
func TestAnEmptyTrailFailsRatherThanPasses(t *testing.T) {
	t.Parallel()
	grade := bundle.GradeRelay(gradeTrail(), &bundle.RelayRun{Since: runStart(), Key: "k1"})
	if grade.Held() {
		t.Error("a run that wrote nothing at all was graded as holding")
	}
}
