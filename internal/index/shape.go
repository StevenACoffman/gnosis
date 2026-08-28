package index

import (
	"context"
	"errors"
	"sort"

	"github.com/StevenACoffman/skillet/errs"
)

// Shape is the difference between the schema a database has and the schema the
// migrations describe.
//
// Both directions matter and they mean different things. Missing objects are the
// dangerous case — a migration that did not finish. Unexpected ones usually mean
// a hand-edited database or a newer binary's work left behind by a downgrade, and
// gnosis never removes them: this is a diagnostic, not a repair.
type Shape struct {
	Missing    []string
	Unexpected []string
}

// OK reports whether the database matches the migrations exactly.
func (s Shape) OK() bool { return len(s.Missing) == 0 && len(s.Unexpected) == 0 }

// Objects lists every table, index, and virtual table the database holds.
//
// Requires: db is open.
// Ensures: sorted, so two readings are comparable.
//
// The shadow tables FTS5 creates beneath a virtual table — `claims_fts_data`,
// `documents_fts_idx`, and their siblings — are **included**, even though no
// migration names them. That is deliberate: they are where the index actually
// lives, a missing one is real corruption, and because the expectation is derived
// the same way (see CheckShape) they cannot become noise. Only SQLite's own
// `sqlite_`-prefixed bookkeeping is excluded.
func (db *DB) Objects(ctx context.Context) ([]string, error) {
	const op = "index.DB.Objects"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%'
		  AND (tbl_name = name OR type = 'index')
		ORDER BY name`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	sort.Strings(out)
	return out, nil
}

// CheckShape compares a database against what the migrations would build.
//
// Requires: db is open.
// Ensures: the expectation is derived by migrating a scratch in-memory database
// rather than read from a hand-maintained list. That matters more than it looks:
// a list would be a second description of the schema, and the failure mode of a
// second description is that it stops matching the first without anyone noticing
// — which is the exact class of defect this check exists to catch.
func (db *DB) CheckShape(ctx context.Context) (Shape, error) {
	const op = "index.DB.CheckShape"

	want, err := expectedObjects(ctx)
	if err != nil {
		return Shape{}, &errs.Error{Op: op, Err: err}
	}
	got, err := db.Objects(ctx)
	if err != nil {
		return Shape{}, &errs.Error{Op: op, Err: err}
	}
	return Shape{
		Missing:    missingFrom(want, got),
		Unexpected: missingFrom(got, want),
	}, nil
}

// Tables reports the content tables the index holds, excluding indexes and the shadow
// tables an FTS5 virtual table maintains.
//
// Requires: db is open.
// Ensures: sorted; never nil. Every name is a table somebody could put rows in.
//
// **Derived from `sqlite_master.type` rather than from a list of name prefixes.** The
// digest-coverage test used to exclude indexes by prefix — `claims_`, `links_`,
// `sources_fetched_` — which meant every new index needed an entry in a list somewhere
// else, and forgetting one produced a failure that read as a missing digest rather than
// as a missing exclusion. SQLite already knows which objects are tables.
func (db *DB) Tables(ctx context.Context) ([]string, error) {
	const op = "index.DB.Tables"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// expectedObjects builds a throwaway index and reports what it contains.
func expectedObjects(ctx context.Context) ([]string, error) {
	scratch, err := Open(ctx, ":memory:")
	if err != nil {
		return nil, err
	}
	objects, err := scratch.Objects(ctx)
	if err != nil {
		return nil, errors.Join(err, scratch.Close())
	}
	return objects, scratch.Close()
}

// missingFrom returns the members of want that do not appear in have. Both
// inputs are sorted, so the result is too.
func missingFrom(want, have []string) []string {
	present := make(map[string]bool, len(have))
	for _, name := range have {
		present[name] = true
	}
	out := make([]string, 0)
	for _, name := range want {
		if !present[name] {
			out = append(out, name)
		}
	}
	return out
}
