package index

import (
	"context"
	"fmt"
)

// This file is compiled only under `go test`, so nothing here reaches a
// production build. It exists because the properties worth testing at this
// layer — that the foreign-key pragma actually took, that a cascade runs in the
// right direction, that an unresolved link keeps its href — are properties of
// the *schema*, and verifying them needs arbitrary SQL that no production caller
// should ever be handed.
//
// rules.md §9 prohibits adding test-only seams to production code. An
// export_test.go is not one: it adds no exported surface to the package as
// consumers see it.

// ExecForTest runs a statement against the index.
func (db *DB) ExecForTest(ctx context.Context, query string) error {
	if _, err := db.sql.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ExecForTest: %w", err)
	}
	return nil
}

// CountForTest runs a query expected to yield a single integer.
func (db *DB) CountForTest(ctx context.Context, query string) (int, error) {
	var n int
	if err := db.sql.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, fmt.Errorf("CountForTest: %w", err)
	}
	return n, nil
}

// SchemaOfForTest returns the CREATE statement SQLite recorded for a table.
//
// Reading the schema back out of sqlite_master rather than inspecting the Go
// source is deliberate: what matters is what the database was actually built
// with, and those two can differ.
func (db *DB) SchemaOfForTest(ctx context.Context, table string) (string, error) {
	var stmt string
	err := db.sql.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE name = ?`, table).Scan(&stmt)
	if err != nil {
		return "", fmt.Errorf("SchemaOfForTest %s: %w", table, err)
	}
	return stmt, nil
}
