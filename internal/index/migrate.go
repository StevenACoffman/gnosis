package index

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/StevenACoffman/skillet/errs"
)

// ftsAnalyzer is the FTS5 tokenizer clause, defined once and shared by every
// full-text table.
//
// It is not a default worth accepting: the apostrophe and slash in the term
// character set are what make "don't" and "foo/bar" findable in technical prose.
// One constant rather than one per table because SPEC §5.5 requires it — two
// copies drift, and the corpus would then search differently depending on which
// table answered.
//
// Named "analyzer" rather than the obvious "tokenizer" because gosec's G101
// credential heuristic matches any identifier containing "token" assigned a long
// string literal. Renaming is the honest fix; the alternative is a nolint on a
// rule that is right far more often than it is wrong. "Analyzer" is the standard
// search-engine word for the same thing, so nothing is lost but the coincidence.
const ftsAnalyzer = "tokenize = \"porter unicode61 remove_diacritics 1 tokenchars '''&/'\""

// migrations returns the ordered schema steps.
//
// It is a function rather than a package variable because a mutable global is
// prohibited (rules.md §1) and a slice var is mutable — skillsaw's Dimensions()
// has the same shape for the same reason.
//
// Each element is one migration and advances PRAGMA user_version by one.
// Appending is the only permitted change **once a version has shipped**: editing
// a shipped statement would leave existing databases at a version whose schema no
// longer matches what that number means. SQLite cannot DROP COLUMN either, so
// there is no repair for it after the fact.
//
// Before the first release the statements are still edited in place, and the
// concept-to-claim rename was one such edit. The alternative would have left a
// permanent ALTER TABLE ... RENAME in the history for a table nobody ever had,
// which is a worse artifact than a clean schema. This licence expires at v1.
//
// The groups below are organisational only; they carry no numbering of their own.
func migrations() []string {
	var all []string
	all = append(all, documentSchema()...)
	all = append(all, linkSchema()...)
	all = append(all, vocabularySchema()...)
	all = append(all, sourceSchema()...)
	all = append(all, claimSummaryNullable()...)
	all = append(all, verificationSchema()...)
	return all
}

// verificationSchema creates the table OKF §5.2's `verified` list projects into.
//
// **A table rather than a column**, and §5.5 gives the reason: OKF §5.2 makes `verified`
// a list of independent *events*, specifically so a human sign-off and an automated pass
// stay distinguishable. Collapsing it to a column destroys the distinction that makes
// trust tiers meaningful (§14.1).
//
// Keyed by claim, which is where §5.5's schema puts it and which required a decision:
// OKF's `verified` is document-level, and expanding one to every claim in a document
// would assert that somebody verified each claim when they verified a page. §5.5.1
// refused exactly that inheritance for `subject` — "editing it would silently re-subject
// every claim that did not override" — so gnosis reads `verified` inside a
// `gnosis_claims` entry and does not expand a document-level list.
func verificationSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS verifications (
			claim_id TEXT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
			by       TEXT NOT NULL,
			at       TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS verifications_claim ON verifications (claim_id)`,
	}
}

// claimSummaryNullable makes a claim's summary columns nullable (SPEC §5.5.3).
//
// `title`, `description` and `lead` were `NOT NULL DEFAULT ”`, and the empty string is
// a value: it asserts that a claim *has* no lead, which is false before extraction has
// written one. §17.4's `lead` check cannot tell that from an author who wrote an empty
// one, so under the old default it would report every claim in the corpus the first time
// it ran. `claims.pos` is nullable one column over for the identical reason — zero is a
// real position, and the empty string is a real lead.
//
// **Appended rather than corrected in place, and the reasoning that said otherwise was
// wrong.** §5.5.3 first claimed a migration could be edited because `schema-shape`
// reports the mismatch; it does not. `DB.Objects` selects `name FROM sqlite_master`, so
// the check compares object *names* and is blind to a column definition — an edited
// migration would leave every existing index with the old constraint and fail at the
// first NULL insert, as a runtime error nothing had warned about.
//
// A drop is safe here and nowhere else: nothing has ever written this table, so there
// are no rows to preserve. The FTS table is recreated with it because SQLite's
// external-content tables reference their content table by name, and a dropped content
// table leaves the index pointing at nothing.
func claimSummaryNullable() []string {
	return []string{
		`DROP TABLE IF EXISTS claims_fts`,
		`DROP TABLE IF EXISTS claims`,
		`CREATE TABLE claims (
			id           TEXT PRIMARY KEY NOT NULL,
			document_id  TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			anchor_hash  TEXT NOT NULL,
			pos          INTEGER,
			type         TEXT NOT NULL,
			title        TEXT,
			description  TEXT,
			lead         TEXT,
			status       TEXT NOT NULL DEFAULT 'stable',
			stale_after  TEXT,
			generated_by TEXT NOT NULL DEFAULT '',
			generated_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX claims_document ON claims (document_id)`,
		`CREATE INDEX claims_type ON claims (type)`,
		`CREATE INDEX claims_anchor ON claims (document_id, anchor_hash)`,
		`CREATE VIRTUAL TABLE claims_fts USING fts5(
			title, description, lead,
			content = claims,
			content_rowid = rowid,
			` + ftsAnalyzer + `
		)`,
	}
}

// SchemaVersion is the version a freshly migrated index reports.
//
// Requires: nothing.
// Ensures: equals what Version returns after Open. Exported so a caller can
// tell "this index is older than the binary", which Open repairs, from "this
// index is newer than the binary", which it cannot.
func SchemaVersion() int {
	return len(migrations())
}

// documentSchema creates documents, claims, and the two search indexes.
func documentSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id            TEXT PRIMARY KEY NOT NULL,
			path          TEXT NOT NULL UNIQUE,
			title         TEXT NOT NULL DEFAULT '',
			slug          TEXT NOT NULL DEFAULT '',
			content_hash  TEXT NOT NULL DEFAULT '',
			byte_size     INTEGER NOT NULL DEFAULT 0,
			created_at    TEXT NOT NULL DEFAULT '',
			modified_at   TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS documents_hash ON documents (content_hash)`,

		// Phase 1 searches documents; claims_fts arrives with extraction in
		// Phase 2 (SPEC §19).
		//
		// Self-contained rather than external-content, which is a departure from
		// the sibling table below and from an earlier reading of SPEC §5.5.
		// External content reads column values back out of the content table, so
		// it would require storing every document body in `documents` purely to
		// satisfy FTS. Holding the text once, here, is the same storage without
		// the second copy and without rowid synchronisation across a replace.
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
			document_id UNINDEXED,
			title,
			body,
			` + ftsAnalyzer + `
		)`,

		// A document holds one or more addressable claims. The split is what
		// lets a verdict attach to the right half of a sentence carrying two
		// assertions, without splitting the file — see SPEC §5.5.
		//
		// anchor_hash is the claim's address. pos is a cached byte offset into
		// the document body and is NULL when the anchor cannot be located
		// (SPEC §5.5.2): zero is a valid position — the first byte — so it
		// cannot double as "missing".
		`CREATE TABLE IF NOT EXISTS claims (
			id           TEXT PRIMARY KEY NOT NULL,
			document_id  TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			anchor_hash  TEXT NOT NULL,
			pos          INTEGER,
			type         TEXT NOT NULL,
			title        TEXT NOT NULL DEFAULT '',
			description  TEXT NOT NULL DEFAULT '',
			lead         TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'stable',
			stale_after  TEXT,
			generated_by TEXT NOT NULL DEFAULT '',
			generated_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS claims_document ON claims (document_id)`,
		`CREATE INDEX IF NOT EXISTS claims_type ON claims (type)`,
		`CREATE INDEX IF NOT EXISTS claims_anchor ON claims (document_id, anchor_hash)`,

		// External content is right here: `claims` already carries every indexed
		// column, so FTS stores the index and nothing else.
		`CREATE VIRTUAL TABLE IF NOT EXISTS claims_fts USING fts5(
			title, description, lead,
			content = claims,
			content_rowid = rowid,
			` + ftsAnalyzer + `
		)`,
	}
}

// linkSchema creates the link graph.
//
// A link keeps what the author wrote even when it resolves to nothing: OKF §6.1
// calls an unresolved target "not-yet-written knowledge", and href is the only
// surviving record of intent once the target is gone. Deleting a document
// therefore degrades its inbound links rather than erasing them.
//
// A link has two sources and only one of them is always known. It is written *in
// a document*, which is true from Phase 1; it sits *within a claim*, which is not
// known until extraction runs (SPEC §19). So source_document_id is NOT NULL and
// source_claim_id is nullable, filled in when the claim containing the link is
// identified. Requiring a claim would have meant Phase 1 could record no links at
// all, and the link graph is most of what `graph` and `show` are for.
func linkSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS links (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			source_document_id  TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			source_claim_id     TEXT REFERENCES claims(id) ON DELETE SET NULL,
			target_document_id  TEXT REFERENCES documents(id) ON DELETE SET NULL,
			href                TEXT NOT NULL,
			title               TEXT NOT NULL DEFAULT '',
			rel                 TEXT NOT NULL DEFAULT '',
			external            INTEGER NOT NULL DEFAULT 0,
			snippet             TEXT NOT NULL DEFAULT '',
			snippet_start       INTEGER NOT NULL DEFAULT 0,
			snippet_end         INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS links_source ON links (source_document_id)`,
		`CREATE INDEX IF NOT EXISTS links_source_claim ON links (source_claim_id)`,
		`CREATE INDEX IF NOT EXISTS links_target ON links (target_document_id)`,
	}
}

// vocabularySchema indexes ontology.toml. Derived like everything else here;
// the file remains authoritative.
func vocabularySchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS subjects (
			key         TEXT PRIMARY KEY NOT NULL,
			dimension   TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			deprecated  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS subject_aliases (
			alias TEXT PRIMARY KEY NOT NULL,
			key   TEXT NOT NULL REFERENCES subjects(key) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS claim_subjects (
			claim_id    TEXT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
			subject_key TEXT NOT NULL,
			op          TEXT NOT NULL DEFAULT '',
			value_norm  REAL,
			value_raw   TEXT NOT NULL DEFAULT '',
			dimension   TEXT NOT NULL DEFAULT '',
			derived     INTEGER NOT NULL DEFAULT 1,
			pattern_id  TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (claim_id, subject_key)
		)`,
	}
}

// migrate applies every migration not yet recorded in user_version.
//
// Requires: db is open.
// Ensures: on success user_version equals len(migrations()); on failure the
// database is left at the last fully applied version, because each migration
// runs in its own transaction.
func (db *DB) migrate(ctx context.Context) error {
	const op = "index.DB.migrate"

	var version int
	if err := db.sql.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	for i, stmt := range migrations() {
		if i < version {
			continue
		}
		if err := applyMigration(ctx, db.sql, stmt, i+1); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
	}
	return nil
}

// applyMigration runs one statement and records the new version atomically.
//
// user_version is set with Sprintf because PRAGMA does not accept a bound
// parameter. The value is a loop index over statements this package owns and
// never reaches here from outside, so there is no injection surface — unlike
// every other query, which binds its arguments.
func applyMigration(ctx context.Context, db *sql.DB, stmt string, version int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration %d: begin: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return fmt.Errorf("migration %d: recording version: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %d: commit: %w", version, err)
	}
	return nil
}
