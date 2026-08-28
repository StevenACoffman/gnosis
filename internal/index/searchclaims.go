package index

import (
	"context"
	"strings"

	"github.com/StevenACoffman/skillet/errs"
)

// ClaimHit is one claim with what a reader needs to identify it.
//
// The lead is the whole payload: §17.4 makes it "the unit of retrieval in §11", so a
// claim is legible from its lead without opening anything. That is also why there is no
// title or description here — nothing writes those columns, and rendering an empty one
// would advertise a field the extractor never fills.
//
// **One type for the two readers that ask this question**: `SearchClaims`, where it is
// what matched, and `AllClaims`, where it is what exists. A second identical triple would
// be two places for the answer to "how do I identify a claim" to diverge.
type ClaimHit struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Lead string `json:"lead"`
}

// ClaimResults is what a claim query answered *and* what it could not reach.
//
// **Unextracted is part of the answer, not a statistic beside it**, which is why it
// travels in the same struct rather than as a second return value a caller can drop.
// `claims_fts` holds only claims that carry a lead (§5.5.3), so a corpus mid-extraction
// has claims this query cannot match at any ranking. A result set that stayed silent
// about them would present a partial answer as a whole one — the defect `index rebuild`
// shipped with and the link report was designed against.
type ClaimResults struct {
	Hits        []ClaimHit `json:"hits"`
	Unextracted int        `json:"unextracted"`
}

// SearchClaims returns claims whose lead matches an FTS5 query, best first.
//
// Requires: db is open; limit is positive.
// Ensures: Hits is empty, never nil. Unextracted counts the claims held in this index
// that carry no lead, whether or not anything matched.
//
// **This is the reader `claims_fts` did not have.** The table has been populated since
// extraction landed and nothing selected from it — stored state whose correctness nobody
// could observe, which is this project's most-repeated defect. Building the query is what
// makes the writing checkable; deleting the writer would have obeyed the rule without
// serving its reason.
//
// A malformed FTS5 query is the caller's syntax, so it comes back as errs.EINVALID
// exactly as Search's does. The two must not disagree about that: one query language,
// one classification, or a reader learns that a typo is a crash in one place and a usage
// error in the other.
func (db *DB) SearchClaims(ctx context.Context, query string, limit int) (*ClaimResults, error) {
	const op = "index.DB.SearchClaims"

	if strings.TrimSpace(query) == "" {
		return nil, &errs.Error{Code: errs.EINVALID, Message: op + ": empty query"}
	}

	rows, err := db.sql.QueryContext(ctx, `
		SELECT c.id,
		       COALESCE(d.path, ''),
		       COALESCE(c.lead, '')
		FROM claims_fts f
		JOIN claims c ON c.rowid = f.rowid
		LEFT JOIN documents d ON d.id = c.document_id
		WHERE claims_fts MATCH ?
		ORDER BY bm25(claims_fts)
		LIMIT ?`,
		query, limit)
	if err != nil {
		return nil, &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := &ClaimResults{Hits: make([]ClaimHit, 0)}
	for rows.Next() {
		var h ClaimHit
		if err := rows.Scan(&h.ID, &h.Path, &h.Lead); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out.Hits = append(out.Hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	// Counted after the query and not derived from it: the shortfall is a property of
	// the corpus, so a query matching nothing must still report it. A count computed
	// only when something matched would go quiet in the case a reader most needs it.
	row := db.sql.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM claims WHERE lead IS NULL OR lead = ''`)
	if err := row.Scan(&out.Unextracted); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// AllClaims returns every claim in the index, with its document and lead.
//
// Requires: db is open.
// Ensures: an empty slice, never nil, ordered by document path then claim id so two runs
// produce the same list. A claim whose lead is NULL comes back with an empty one rather
// than being dropped — §12.2's reach report needs to *see* that it cannot be retrieved,
// and a query that quietly excluded it would make the corpus look smaller than it is.
func (db *DB) AllClaims(ctx context.Context) ([]ClaimHit, error) {
	const op = "index.DB.AllClaims"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT c.id, COALESCE(d.path, ''), COALESCE(c.lead, '')
		FROM claims c
		LEFT JOIN documents d ON d.id = c.document_id
		ORDER BY d.path, c.id`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]ClaimHit, 0)
	for rows.Next() {
		var h ClaimHit
		if sErr := rows.Scan(&h.ID, &h.Path, &h.Lead); sErr != nil {
			return nil, &errs.Error{Op: op, Err: sErr}
		}
		out = append(out, h)
	}
	if rErr := rows.Err(); rErr != nil {
		return nil, &errs.Error{Op: op, Err: rErr}
	}
	return out, nil
}
