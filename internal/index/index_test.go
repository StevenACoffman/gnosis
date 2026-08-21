package index_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/index"
)

// openTemp opens an index in a per-test directory. A real SQLite file is used
// rather than a mock: the behaviour under test is the schema and the migration
// mechanism, and a mock of those would only test the mock.
func openTemp(t *testing.T) *index.DB {
	t.Helper()
	db, err := index.Open(t.Context(), filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return db
}

func TestOpenMigratesToCurrentVersion(t *testing.T) {
	t.Parallel()
	db := openTemp(t)
	v, err := db.Version(t.Context())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v == 0 {
		t.Error("Version = 0; Open should have applied the schema")
	}
}

// TestReopenIsIdempotent is the property that makes `index rebuild` safe to run
// at any time: a second Open must not re-apply migrations or fail on tables that
// already exist.
func TestReopenIsIdempotent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "index.db")

	first, err := index.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	v1, err := first.Version(t.Context())
	if err != nil {
		t.Fatalf("first Version: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	second, err := index.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() {
		if err := second.Close(); err != nil {
			t.Errorf("second Close: %v", err)
		}
	}()
	v2, err := second.Version(t.Context())
	if err != nil {
		t.Fatalf("second Version: %v", err)
	}
	if v1 != v2 {
		t.Errorf("version moved on reopen: %d then %d", v1, v2)
	}
}

// TestForeignKeysAreEnforced checks the pragma actually took. SQLite disables
// foreign keys by default per connection, so every ON DELETE CASCADE in the
// schema is inert unless the DSN turns them on — a silent failure that would
// leave orphaned claims behind every deleted document.
func TestForeignKeysAreEnforced(t *testing.T) {
	t.Parallel()
	db := openTemp(t)
	err := db.ExecForTest(t.Context(),
		`INSERT INTO claims (id, document_id, type) VALUES ('c1', 'nonexistent', 'Reference')`)
	if err == nil {
		t.Error("inserted a claim referencing no document; foreign keys are not enforced")
	}
}

// TestCascadeDeletesClaims pairs with the pragma check: enforcement is only
// useful if the cascade direction is right.
func TestCascadeDeletesClaims(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	mustExec(t, db, `INSERT INTO documents (id, path) VALUES ('d1', 'c/a.md')`)
	mustExec(
		t,
		db,
		`INSERT INTO claims (id, document_id, anchor_hash, type) VALUES ('c1', 'd1', 'h1', 'Reference')`,
	)

	if n := count(t, db, `SELECT COUNT(*) FROM claims`); n != 1 {
		t.Fatalf("setup: claims = %d, want 1", n)
	}
	mustExec(t, db, `DELETE FROM documents WHERE id = 'd1'`)
	if n := count(t, db, `SELECT COUNT(*) FROM claims`); n != 0 {
		t.Errorf("claims = %d after deleting the document, want 0", n)
	}
}

// TestUnresolvedLinkSurvivesTargetDeletion is the OKF §6.1 property: a link to a
// document that no longer exists is legal, and the href must remain. Losing it
// would discard the only record of what the author meant.
func TestUnresolvedLinkSurvivesTargetDeletion(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	mustExec(t, db, `INSERT INTO documents (id, path) VALUES ('src', 'c/a.md')`)
	mustExec(t, db, `INSERT INTO documents (id, path) VALUES ('dst', 'c/b.md')`)
	mustExec(
		t,
		db,
		`INSERT INTO claims (id, document_id, anchor_hash, type) VALUES ('c1', 'src', 'h1', 'Reference')`,
	)
	mustExec(t, db, `INSERT INTO links (source_document_id, target_document_id, href)
		VALUES ('src', 'dst', '/c/b.md')`)

	mustExec(t, db, `DELETE FROM documents WHERE id = 'dst'`)

	if n := count(t, db, `SELECT COUNT(*) FROM links WHERE href = '/c/b.md'`); n != 1 {
		t.Fatalf("link rows = %d after target deletion, want the link retained", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM links WHERE target_document_id IS NULL`); n != 1 {
		t.Error("target was not set NULL; the link should degrade, not vanish or dangle")
	}
}

// TestFullTextSearchFindsTechnicalTokens exercises the custom tokenizer. The
// apostrophe and slash in tokenchars are the reason it is configured at all.
func TestFullTextSearchFindsTechnicalTokens(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	mustExec(t, db, `INSERT INTO documents (id, path) VALUES ('d1', 'c/a.md')`)
	mustExec(t, db, `INSERT INTO claims (id, document_id, anchor_hash, type, title, lead)
		VALUES ('c1', 'd1', 'h1', 'Reference', 'Cache behaviour', 'the value does not persist')`)
	mustExec(t, db, `INSERT INTO claims_fts (rowid, title, description, lead)
		SELECT rowid, title, description, lead FROM claims WHERE id = 'c1'`)

	for _, q := range []string{"cache", "persist", "persists"} {
		if n := count(
			t,
			db,
			`SELECT COUNT(*) FROM claims_fts WHERE claims_fts MATCH '`+q+`'`,
		); n != 1 {
			t.Errorf("MATCH %q returned %d rows, want 1", q, n)
		}
	}
}

func mustExec(t *testing.T, db *index.DB, query string) {
	t.Helper()
	if err := db.ExecForTest(t.Context(), query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func count(t *testing.T, db *index.DB, query string) int {
	t.Helper()
	n, err := db.CountForTest(t.Context(), query)
	if err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

// TestBothSearchTablesShareOneAnalyzer is the point of hoisting the tokenizer
// clause into a constant, and it is asserted against the database rather than
// against the Go source because that is where a divergence would actually bite:
// a corpus that tokenizes differently depending on which table answered would
// return different results for one query with no error anywhere.
func TestBothSearchTablesShareOneAnalyzer(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	const clause = `tokenize = "porter unicode61 remove_diacritics 1 tokenchars '''&/'"`
	for _, table := range []string{"documents_fts", "claims_fts"} {
		got, err := db.SchemaOfForTest(t.Context(), table)
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}
		if !strings.Contains(got, clause) {
			t.Errorf("%s does not carry the shared analyzer clause:\n%s", table, got)
		}
	}
}

// TestDocumentSearchIsSelfContained pins the departure from claims_fts.
// documents_fts holds its own copy of the text, so it does not follow the
// document cascade and a replace has to clear it explicitly. Forgetting that
// would leave search answering from documents that no longer exist — which reads
// as a stale result rather than as a bug, and so would survive a long time.
func TestDocumentSearchIsSelfContained(t *testing.T) {
	t.Parallel()
	db := openTemp(t)

	mustExec(
		t,
		db,
		`INSERT INTO documents (id, path, title) VALUES ('d1', 'c/a.md', 'Retry budget')`,
	)
	mustExec(t, db, `INSERT INTO documents_fts (document_id, title, body)
		VALUES ('d1', 'Retry budget', 'the service retries three times')`)

	if n := count(
		t,
		db,
		`SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH 'retries'`,
	); n != 1 {
		t.Fatalf("setup: body is not searchable, got %d rows", n)
	}

	mustExec(t, db, `DELETE FROM documents WHERE id = 'd1'`)
	if n := count(
		t,
		db,
		`SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH 'retries'`,
	); n != 1 {
		t.Fatalf("documents_fts followed the delete; the test's premise is wrong, got %d", n)
	}
}
