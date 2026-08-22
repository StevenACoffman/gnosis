package index

import (
	"context"
	"database/sql"

	"github.com/StevenACoffman/skillet/errs"
)

// SourceRow is one fetch record, as the index holds it.
//
// It is a **projection of tier 0 and nothing more** (§4.3.1): every column comes
// from a committed record, the key is that record's own hash, and a rebuild
// reproduces the whole table from `evidence/fetch/`. Nothing writes here except a
// rebuild, and nothing in it is a fact the bundle does not already state.
//
// What it buys is that tier 0 becomes queryable. Every question about the archive
// currently walks a directory — which archived text backs this URI, how many
// sources are `referenced`, which extractor produced this file — and each of those
// is a scan of the filesystem in code that had to be written to do it.
type SourceRow struct {
	RecordSHA256     string
	URI              string
	SourceSHA256     string
	ByteSize         int64
	MediaType        string
	Disposition      string
	ArchivePath      string
	Extractor        string
	ExtractorVersion string
	ExtractedFrom    string
	RejectReason     string
}

// sourceSchema is the tier-0 projection.
//
// `record_sha256` is the primary key rather than the URI because a source fetched
// twice has two records — one per version — and both belong here. Keying on the
// URI would silently keep whichever was written last, which is the history §4.1
// went to some trouble to preserve.
func sourceSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS sources_fetched (
			record_sha256     TEXT PRIMARY KEY NOT NULL,
			uri               TEXT NOT NULL,
			source_sha256     TEXT NOT NULL,
			byte_size         INTEGER NOT NULL,
			media_type        TEXT NOT NULL DEFAULT '',
			disposition       TEXT NOT NULL,
			archive_path      TEXT NOT NULL DEFAULT '',
			extractor         TEXT NOT NULL DEFAULT '',
			extractor_version TEXT NOT NULL DEFAULT '',
			extracted_from    TEXT NOT NULL DEFAULT '',
			reject_reason     TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS sources_fetched_uri ON sources_fetched(uri)`,
	}
}

// ReplaceSources rewrites the tier-0 projection.
//
// Requires: rows came from the committed records; the caller holds the writer lock.
// Ensures: the table afterwards holds exactly rows, in one transaction, so a
// reader never sees a partially rebuilt projection. Replace rather than merge,
// because the table is derived: a record deleted from tier 0 must disappear here,
// and a merge would leave it behind as the only surviving trace of something the
// corpus no longer holds.
func (db *DB) ReplaceSources(ctx context.Context, rows []SourceRow) error {
	const op = "index.DB.ReplaceSources"

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `DELETE FROM sources_fetched`); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if err = insertSources(ctx, tx, rows); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if err = tx.Commit(); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	return nil
}

// insertSources writes every row through one prepared statement.
func insertSources(ctx context.Context, tx *sql.Tx, rows []SourceRow) error {
	const op = "index.insertSources"

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO sources_fetched (
			record_sha256, uri, source_sha256, byte_size, media_type, disposition,
			archive_path, extractor, extractor_version, extracted_from, reject_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = stmt.Close() }()

	for i := range rows {
		r := &rows[i]
		if _, err = stmt.ExecContext(ctx,
			r.RecordSHA256, r.URI, r.SourceSHA256, r.ByteSize, r.MediaType,
			r.Disposition, r.ArchivePath, r.Extractor, r.ExtractorVersion,
			r.ExtractedFrom, r.RejectReason,
		); err != nil {
			return &errs.Error{Op: op, Message: op + ": " + r.URI, Err: err}
		}
	}
	return nil
}

// Sources reads the tier-0 projection, ordered by URI then record hash.
//
// Requires: db is open.
// Ensures: a stable order, so two reads over one state are comparable. Empty
// rather than nil, so a caller need not distinguish "nothing fetched" from "no
// result" — and a corpus that has fetched nothing is the ordinary state of a fresh
// bundle rather than a condition to handle.
func (db *DB) Sources(ctx context.Context) ([]SourceRow, error) {
	const op = "index.DB.Sources"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT record_sha256, uri, source_sha256, byte_size, media_type,
		       disposition, archive_path, extractor, extractor_version,
		       extracted_from, reject_reason
		FROM sources_fetched
		ORDER BY uri, record_sha256`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]SourceRow, 0)
	for rows.Next() {
		var r SourceRow
		if err = rows.Scan(&r.RecordSHA256, &r.URI, &r.SourceSHA256, &r.ByteSize,
			&r.MediaType, &r.Disposition, &r.ArchivePath, &r.Extractor,
			&r.ExtractorVersion, &r.ExtractedFrom, &r.RejectReason); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, r)
	}
	if err = rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}
