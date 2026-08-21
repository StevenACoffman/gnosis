package index

import (
	"context"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// snippetEllipsis is what FTS5 puts where it trimmed the surrounding text.
const snippetEllipsis = "…"

// snippetTokens is how much context a snippet carries. FTS5 caps this at 64.
const snippetTokens = 24

// Hit is one search result, with everything a reader needs to decide whether to
// open it and to follow it without asking again (SPEC §8.3).
type Hit struct {
	ID       string     `json:"id"`
	Path     string     `json:"path"`
	Title    string     `json:"title"`
	Snippet  string     `json:"snippet"`
	Outbound []Resolved `json:"outbound,omitempty"`
}

// Search returns documents matching an FTS5 query, best first.
//
// Requires: db is open; limit is positive.
// Ensures: returns an empty slice, never nil. Results are ordered by bm25 rank
// with the title weighted above the body, because a document *about* a term is
// more often what a reader wants than one that mentions it in passing. Each hit
// carries its resolved outbound links, so following one costs no second query.
//
// A malformed FTS5 query is a usage error, not a tool failure: the syntax is the
// caller's, and reporting it as a crash would send them looking in the wrong
// place. Callers get errs.EINVALID and should render it as such.
func (db *DB) Search(ctx context.Context, query string, limit int) ([]Hit, error) {
	const op = "index.DB.Search"

	if strings.TrimSpace(query) == "" {
		return nil, &errs.Error{Code: errs.EINVALID, Message: op + ": empty query"}
	}

	rows, err := db.sql.QueryContext(ctx, `
		SELECT f.document_id,
		       COALESCE(d.path, ''),
		       COALESCE(d.title, ''),
		       snippet(documents_fts, 2, '', '', ?, ?)
		FROM documents_fts f
		LEFT JOIN documents d ON d.id = f.document_id
		WHERE documents_fts MATCH ?
		ORDER BY bm25(documents_fts, 0.0, 10.0, 1.0)
		LIMIT ?`,
		snippetEllipsis, snippetTokens, query, limit)
	if err != nil {
		// FTS5 reports a syntax error through the driver, and it is the caller's
		// syntax. Classifying it as EINVALID is what lets the command exit 2.
		return nil, &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]Hit, 0)
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.ID, &h.Path, &h.Title, &h.Snippet); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	// Resolved links are fetched per hit rather than joined, because a join would
	// multiply rows by links and the result set is already bounded by limit.
	for i := range out {
		id, err := gnosis.ParseID(out[i].ID)
		if err != nil {
			continue
		}
		links, err := db.Outbound(ctx, id)
		if err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out[i].Outbound = links
	}
	return out, nil
}
