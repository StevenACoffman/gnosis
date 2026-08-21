package index

import (
	"context"
	"fmt"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// DocumentRow is one indexed document.
//
// Body is carried for the full-text index and nothing else. The file on disk
// remains the truth (SPEC §4.5); this is a searchable copy in a cache that
// `index rebuild` reproduces.
type DocumentRow struct {
	ID    gnosis.ID
	Path  string
	Title string
	Slug  string
	Hash  string
	Body  string
	Bytes int
}

// Indexed lists every document row as gnosis.Reconcile consumes them.
//
// Requires: db is open.
// Ensures: returns an empty slice, never nil, for an empty index, so a caller
// need not distinguish "no rows" from "no result". Ordered by path, so two calls
// against one index are comparable.
func (db *DB) Indexed(ctx context.Context) ([]gnosis.Indexed, error) {
	const op = "index.DB.Indexed"

	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, path FROM documents ORDER BY path`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]gnosis.Indexed, 0)
	for rows.Next() {
		var r gnosis.Indexed
		if err := rows.Scan(&r.ID, &r.Path); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// Replace makes the index describe exactly the documents given.
//
// Requires: every row carries a non-empty ID and Path.
// Ensures: the index afterwards contains precisely these documents and links — the whole
// operation is one transaction, so a failure leaves the previous contents
// intact rather than a partial rebuild. Concepts cascade from their documents,
// so removing a document removes its claims.
//
// This is a replace rather than a merge because the index is a derived cache
// (SPEC §4.5): reconstructing it wholesale is the cheaper correct operation, and
// a merge would need its own reconciliation logic that could disagree with
// gnosis.Reconcile.
func (db *DB) Replace(ctx context.Context, docs []DocumentRow, links []LinkRow) error {
	const op = "index.DB.Replace"

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = tx.Rollback() }()

	// documents_fts is self-contained rather than external-content, so it does
	// not follow the cascade and has to be cleared explicitly. Forgetting this
	// would leave search answering from documents that no longer exist — a
	// failure that looks like a stale result rather than like a bug.
	for _, stmt := range []string{`DELETE FROM documents`, `DELETE FROM documents_fts`} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
	}
	for i := range docs {
		if err := insertDocument(ctx, tx, &docs[i]); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
	}
	// Links go in after every document, because a link's target is a foreign key
	// and forward references are ordinary in a corpus: A can cite B whether or
	// not B was walked first.
	for i := range links {
		if err := insertLink(ctx, tx, &links[i]); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
	}
	if err := tx.Commit(); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	return nil
}

// Count reports how many documents the index holds.
//
// Requires: db is open.
// Ensures: never negative.
func (db *DB) Count(ctx context.Context) (int, error) {
	const op = "index.DB.Count"
	var n int
	err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&n)
	if err != nil {
		return 0, &errs.Error{Op: op, Err: err}
	}
	return n, nil
}

// insertDocument writes one row. Unexported and taking a transaction handle so
// several service methods can compose it; callers outside this package never see
// a transaction (rules.md §8).
func insertDocument(ctx context.Context, tx execer, d *DocumentRow) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO documents (id, path, title, slug, content_hash, byte_size)
		VALUES (?, ?, ?, ?, ?, ?)`,
		d.ID.String(), d.Path, d.Title, d.Slug, d.Hash, d.Bytes)
	if err != nil {
		return fmt.Errorf("insert document %s: %w", d.ID, err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO documents_fts (document_id, title, body) VALUES (?, ?, ?)`,
		d.ID.String(), d.Title, d.Body)
	if err != nil {
		return fmt.Errorf("index document %s for search: %w", d.ID, err)
	}
	return nil
}
