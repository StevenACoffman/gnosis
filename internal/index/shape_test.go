package index_test

import (
	"slices"
	"testing"
)

// TestFreshIndexMatchesItsMigrations is the baseline: if this ever fails, the
// check is measuring something other than what it claims to.
func TestFreshIndexMatchesItsMigrations(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	shape, err := db.CheckShape(t.Context())
	if err != nil {
		t.Fatalf("CheckShape: %v", err)
	}
	if !shape.OK() {
		t.Errorf("a freshly migrated index does not match its own migrations:\n"+
			"missing:    %v\nunexpected: %v", shape.Missing, shape.Unexpected)
	}
}

// TestPartialMigrationIsDetected is the failure this check exists for and the
// one PRAGMA user_version cannot see. Each migration commits in its own
// transaction, so an interrupted run leaves a database recording a version whose
// schema is not all present — and every later command then fails on a missing
// table with SQLite's error rather than a diagnosis.
func TestPartialMigrationIsDetected(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	mustExec(t, db, `DROP TABLE claim_subjects`)

	shape, err := db.CheckShape(t.Context())
	if err != nil {
		t.Fatalf("CheckShape: %v", err)
	}
	if len(shape.Missing) != 1 || shape.Missing[0] != "claim_subjects" {
		t.Errorf("missing = %v, want exactly [claim_subjects]", shape.Missing)
	}
	if len(shape.Unexpected) != 0 {
		t.Errorf("unexpected = %v, want none", shape.Unexpected)
	}
}

// TestExtraObjectIsReportedNotRemoved: an object the migrations do not describe
// is usually a newer gnosis's work left by a downgrade, or something a person
// added on purpose. Reporting it is right; dropping it would be gnosis deleting
// what it does not understand.
func TestExtraObjectIsReportedNotRemoved(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	mustExec(t, db, `CREATE TABLE scratch (id TEXT PRIMARY KEY)`)

	shape, err := db.CheckShape(t.Context())
	if err != nil {
		t.Fatalf("CheckShape: %v", err)
	}
	if len(shape.Unexpected) != 1 || shape.Unexpected[0] != "scratch" {
		t.Errorf("unexpected = %v, want exactly [scratch]", shape.Unexpected)
	}

	after, err := db.Objects(t.Context())
	if err != nil {
		t.Fatalf("Objects: %v", err)
	}
	if !slices.Contains(after, "scratch") {
		t.Error("CheckShape removed an object; it is a diagnostic, not a repair")
	}
}
