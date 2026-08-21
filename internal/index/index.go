// Package index is the derived SQLite index over a knowledge base.
//
// Everything here is reconstructible. The bundle and the evidence archive are
// the source of truth; this database is a cache that `gnosis index rebuild`
// recreates from them. Anything that exists only here is a bug — see SPEC §4.5.
// That property is what lets the file be gitignored, and it is bought by every
// document carrying its own identifier in frontmatter (SPEC §5.1).
//
// The driver is modernc.org/sqlite: pure Go, no CGo, so the binary stays single
// and cross-compilable. FTS5 is available in it and is the baseline search.
package index

import (
	"context"
	"database/sql"
	"errors"

	// Registers the pure-Go "sqlite" driver.
	_ "modernc.org/sqlite"

	"github.com/StevenACoffman/skillet/errs"
)

// busyTimeout keeps a concurrent reader from failing outright while a write is
// in flight. Single-writer is the assumed model (SPEC §20), so this covers the
// overlap between a command and a long-running viewer rather than contention.
const busyTimeout = "5000"

// DB is an open index.
type DB struct {
	sql *sql.DB
}

// execer is the subset of *sql.Tx the unexported helpers need. Declaring it
// keeps a transaction out of every helper signature while still letting several
// service methods compose one — rules.md §8 forbids a transaction appearing in
// anything a caller outside this package can see.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Open opens or creates the index at path.
//
// Requires: path is writable, or is ":memory:".
// Ensures: the schema is migrated to the current version before returning. On
// any error the database is closed, so a caller never receives a half-open
// handle it would have to remember to clean up.
func Open(ctx context.Context, path string) (*DB, error) {
	const op = "index.Open"

	// Foreign keys are off by default in SQLite and must be enabled per
	// connection; without this the ON DELETE CASCADE clauses are inert.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(" + busyTimeout + ")"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	db := &DB{sql: sqlDB}
	if err := db.sql.PingContext(ctx); err != nil {
		return nil, errors.Join(&errs.Error{Op: op, Err: err}, db.Close())
	}
	if err := db.migrate(ctx); err != nil {
		return nil, errors.Join(&errs.Error{Op: op, Err: err}, db.Close())
	}
	return db, nil
}

// Close releases the database.
func (db *DB) Close() error {
	if db.sql == nil {
		return nil
	}
	if err := db.sql.Close(); err != nil {
		return &errs.Error{Op: "index.DB.Close", Err: err}
	}
	return nil
}

// Version reports the applied schema version.
//
// Requires: db is open.
// Ensures: equals len(migrations) after a successful Open.
func (db *DB) Version(ctx context.Context) (int, error) {
	const op = "index.DB.Version"
	var v int
	if err := db.sql.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return 0, &errs.Error{Op: op, Err: err}
	}
	return v, nil
}
