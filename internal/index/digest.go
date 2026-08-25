package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"

	"github.com/StevenACoffman/skillet/errs"
)

// digestedTable is one table Digest covers, and the query that reads it.
//
// The query is a whole literal rather than columns assembled at read time, for two
// reasons. It is visible — a reader checking what the digest covers sees the
// statement rather than reconstructing it — and there is no concatenation at the
// call site, so there is nowhere for a caller-supplied string to arrive even by
// accident.
//
// There is deliberately no column count here. An earlier version carried one so a
// row could be scanned without asking the driver, and that made the struct a second
// description of the query beside it — the class of defect `schema-shape` exists to
// catch, reintroduced one file over. `rows.Columns` already knows.
type digestedTable struct {
	name  string
	query string
}

// digestedTables are the tables Digest covers.
//
// A function rather than a package variable, for the reason `sourceSchema` is one:
// package-level mutable state is prohibited, and a slice is mutable however much a
// reader treats it as constant.
//
// The list is a literal rather than a walk over sqlite_master, and the reason is the
// property being measured. A digest derived from whatever tables happen to exist
// would change when the *schema* changed, so two rebuilds by binaries at different
// migration levels would differ for a reason that has nothing to do with the
// corpus — and `schema-shape` is the check that owns that question. This one is
// about content.
//
// Every query carries an explicit ORDER BY. SQLite makes no promise about row order
// without one, and the whole point here is that two builds agree; §18.3 lists
// iteration order as its own source of non-determinism, and this is the same hazard
// one layer down.
//
// FTS5's shadow tables are deliberately absent. They hold the index of the content
// rather than the content, their internal representation is SQLite's business, and
// including them would make the digest depend on a library version.
func digestedTables() []digestedTable {
	return []digestedTable{
		{
			name: "documents",
			query: `SELECT id, path, title, slug, content_hash, byte_size
				FROM documents ORDER BY id`,
		},
		{
			name: "links",
			query: `SELECT source_document_id, source_claim_id, target_document_id,
				href, external FROM links
				ORDER BY source_document_id, href, target_document_id`,
		},
		{
			name: "claims",
			query: `SELECT id, document_id, anchor_hash, pos, type, title, lead
				FROM claims ORDER BY id`,
		},
		{
			name: "sources_fetched",
			query: `SELECT record_sha256, uri, source_sha256, byte_size, media_type,
				disposition, archive_path, extractor, extractor_version,
				extracted_from, reject_reason
				FROM sources_fetched ORDER BY record_sha256`,
		},
		{
			name:  "subjects",
			query: `SELECT key, dimension, description, deprecated FROM subjects ORDER BY key`,
		},
		{
			name:  "subject_aliases",
			query: `SELECT alias, key FROM subject_aliases ORDER BY alias`,
		},
		{
			name: "claim_subjects",
			query: `SELECT claim_id, subject_key, op, value_norm, value_raw, dimension,
				derived, pattern_id FROM claim_subjects
				ORDER BY claim_id, subject_key`,
		},
	}
}

// Digest is a content hash of every row the index holds.
//
// Requires: db is open.
// Ensures: the same digest for two indexes holding the same rows, whatever order
// they were written in and whatever the files look like on disk. A different digest
// for any difference in any covered column.
//
// # Why this exists rather than a file comparison
//
// SPEC §18.3 asks that `index rebuild` twice from one bundle produce "byte-identical
// `index.db` content hashes", and a SQLite file is not byte-stable: page allocation,
// the freelist, and the write order all leave traces that differ between two builds
// of identical content. Comparing the files would fail on a database that is
// correct, which is the worse of the two errors — a determinism test nobody trusts
// gets turned off, and then the property is unmeasured.
//
// So the property is stated over content. That is weaker than byte-identity in one
// respect, and it is the respect that does not matter: what §4.6 leans on is that
// two colleagues at one commit hold indexes that **answer the same questions**, and
// answering the same questions is what this compares.
//
// # Why it is exported
//
// It is what `index rebuild` reports, so two people can compare a string rather
// than argue about whether their disagreement is about the corpus or about their
// caches. A digest whose only caller was a test would be the test-only seam the
// guidelines prohibit, and §4.6's per-user-index claim is the reason a user has to
// check.
func (db *DB) Digest(ctx context.Context) (string, error) {
	const op = "index.DB.Digest"

	sum := sha256.New()
	for _, table := range digestedTables() {
		if err := db.digestTable(ctx, op, sum, &table); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// DigestedTables names the tables Digest covers, sorted.
//
// Requires: nothing.
// Ensures: the table names, so a test can assert that every content table the
// migrations create is covered. Pure.
//
// It exists because the failure this design permits is silent: a table added by a
// later migration and not added here would leave its rows out of the digest, and
// two rebuilds differing only in that table would report identical digests. The
// test that compares this against Objects is what makes the omission loud.
func DigestedTables() []string {
	tables := digestedTables()
	out := make([]string, 0, len(tables))
	for _, t := range tables {
		out = append(out, t.name)
	}
	sort.Strings(out)
	return out
}

// digestTable folds one table's rows into sum.
//
// The table name goes into the hash before its rows, so moving a row between two
// tables with the same columns changes the digest. Without it the digest would be a
// hash of a bag of values with no account of where they were.
//
// Write errors are not checked, and that is a documented property rather than an
// omission: hash.Hash's Write "never returns an error". Checking it would put four
// unreachable branches in the middle of the loop, which is what the complexity
// linter objected to on the first version of this function — correctly.
func (db *DB) digestTable(
	ctx context.Context, op string, sum hash.Hash, table *digestedTable,
) error {
	rows, err := db.sql.QueryContext(ctx, table.query)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	_, _ = fmt.Fprintf(sum, "table\x00%s\x00", table.name)

	columns, err := rows.Columns()
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	cells := make([]any, len(columns))
	scan := make([]any, len(columns))
	for i := range cells {
		scan[i] = &cells[i]
	}
	line := make([]byte, 0, 256)
	for rows.Next() {
		if err = rows.Scan(scan...); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
		_, _ = sum.Write(appendRow(line[:0], cells))
	}
	if err = rows.Err(); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	return nil
}

// appendRow renders one row's cells into dst and returns the extended slice.
//
// Requires: nothing.
// Ensures: two rows differing in any cell render differently. Pure, and it allocates
// nothing when dst has room — the caller reuses one buffer across a whole table.
//
// Each cell is delimited rather than concatenated, because concatenation makes
// ("ab", "c") and ("a", "bc") indistinguishable. That is the same collision the
// relay's cache key had to be fixed for, and the failure it produces here is two
// different indexes reporting one digest.
//
// A NULL renders as its own token rather than as an empty string, because
// `claims.pos` is nullable and 0 is a real position (§5.5.2) — collapsing them would
// make "not located" and "the first byte of the body" hash alike, which is the exact
// distinction that column exists to keep.
func appendRow(dst []byte, cells []any) []byte {
	for _, cell := range cells {
		dst = append(dst, 0x01)
		if cell == nil {
			dst = append(dst, "NULL"...)
		} else {
			dst = fmt.Appendf(dst, "%v", cell)
		}
		dst = append(dst, 0x00)
	}
	return append(dst, 0x02)
}
