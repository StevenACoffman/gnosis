package indexcmd_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestRecreateRepairsASchemaShapeFailure is the remedy `schema-shape` had always advised
// and never had.
//
// That diagnostic declares `ActionAutomatic` — a claim that a machine can fix it — while
// telling a reader to delete the file by hand. Plain `rebuild` could not: it opens the
// existing database and migration skips every statement because `user_version` is already
// current, so it fails on the missing table rather than recreating it. This test is what
// makes the action code true rather than aspirational.
func TestRecreateRepairsASchemaShapeFailure(t *testing.T) {
	t.Parallel()

	dir := corpusOf(t)
	dropTable(t, dir, "links")

	if _, err := run(t, "--bundle", dir, "index", "rebuild"); err == nil {
		t.Fatal("the fixture no longer reproduces the defect: a rebuild over a " +
			"database missing a table succeeded")
	}
	if _, err := run(t, "--bundle", dir, "index", "rebuild", "--recreate"); err != nil {
		t.Fatalf("--recreate did not repair the database: %v", err)
	}
	if _, err := run(t, "--bundle", dir, "index", "rebuild"); err != nil {
		t.Fatalf("the repaired database still will not rebuild: %v", err)
	}
}

// TestRecreateSaysWhatItDestroys is the guard the `Unexpected` half of `schema-shape`
// exists for: people do put things in this database, and a repair that silently dropped
// them would be worse than the manual step it replaces.
func TestRecreateSaysWhatItDestroys(t *testing.T) {
	t.Parallel()

	dir := corpusOf(t)
	exec(t, dir, "CREATE TABLE mine (note TEXT)")

	stderr, err := run(t, "--bundle", dir, "index", "rebuild", "--recreate")
	if err != nil {
		t.Fatalf("rebuild --recreate: %v", err)
	}
	if !strings.Contains(stderr, "mine") {
		t.Errorf("the warning does not name what was destroyed:\n%s", stderr)
	}
	if !strings.Contains(stderr, "will not bring them back") {
		t.Errorf("the warning does not say the loss is permanent:\n%s", stderr)
	}
}

// TestCheckAndRecreateIsRefused keeps a read-only question from deleting anything. A flag
// combination that quietly did would be the worst possible way to learn the difference
// between the two.
func TestCheckAndRecreateIsRefused(t *testing.T) {
	t.Parallel()

	dir := corpusOf(t)
	before, err := os.Stat(filepath.Join(dir, ".gnosis", "index.db"))
	if err != nil {
		t.Fatalf("stat the index: %v", err)
	}

	stderr, err := run(t, "--bundle", dir, "index", "rebuild", "--check", "--recreate")
	if err == nil {
		t.Fatal("--check --recreate was accepted")
	}
	if !strings.Contains(stderr, "--check") {
		t.Errorf("the refusal does not name the conflicting flag:\n%s", stderr)
	}
	after, err := os.Stat(filepath.Join(dir, ".gnosis", "index.db"))
	if err != nil {
		t.Fatalf("the database was deleted by a refused invocation: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("a refused invocation still wrote to the database")
	}
}

// exec runs one statement against a bundle's index.
func exec(t *testing.T, dir, statement string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, ".gnosis", "index.db"))
	if err != nil {
		t.Fatalf("open the index: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(t.Context(), statement); err != nil {
		t.Fatalf("exec %q: %v", statement, err)
	}
}

// dropTable removes one table, which is the shape of an interrupted migration.
func dropTable(t *testing.T, dir, name string) {
	t.Helper()
	exec(t, dir, "DROP TABLE "+name)
}
