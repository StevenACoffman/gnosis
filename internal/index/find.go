package index

import (
	"context"
	"database/sql"
	"errors"
	"path"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// idLength is the character width of a UUIDv7 in its canonical form. A reference
// may carry a slug after it, so a prefix of exactly this length is the candidate.
const idLength = 36

// Detail is a document as `show` renders it.
type Detail struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Title string `json:"title"`
	Bytes int    `json:"bytes"`

	// ContentHash is the document as the index last saw it.
	//
	// Carried so a reader can be told when the two copies disagree. `show` reads the
	// file and `search` reads the indexed text, which are both defensible — the file
	// is the truth, the index is what was searched — and a document edited since the
	// last rebuild shows fresh text with a stale snippet. The column was already
	// stored; nothing had asked for it.
	ContentHash string `json:"content_hash,omitempty"`

	Outbound []Resolved `json:"outbound"`
	Inbound  []Resolved `json:"inbound"`
}

// Find locates a document by identifier, however the caller spelled it.
//
// Requires: db is open.
// Ensures: resolves a bare identifier, a current path, or a **stale** path whose
// slug no longer matches — the identifier is parsed out of the filename and
// matched on, which is the property that lets a link written before a retitle
// keep working with no mapping table (SPEC §5.1.1). Reports ENOTFOUND rather than
// (nil, nil) when nothing matches.
func (db *DB) Find(ctx context.Context, ref string) (*Detail, error) {
	const op = "index.DB.Find"

	id, ok := identifierIn(ref)
	if !ok {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + ref + " contains no document identifier",
		}
	}

	var d Detail
	err := db.sql.QueryRowContext(ctx,
		`SELECT id, path, title, byte_size, content_hash FROM documents WHERE id = ?`,
		id.String()).Scan(&d.ID, &d.Path, &d.Title, &d.Bytes, &d.ContentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{
			Code:    errs.ENOTFOUND,
			Message: op + ": no document with identifier " + id.String(),
		}
	}
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	if d.Outbound, err = db.Outbound(ctx, id); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if d.Inbound, err = db.Inbound(ctx, id); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return &d, nil
}

// identifierIn extracts a document identifier from a reference.
//
// A reference may be the bare identifier, a canonical path, or any path whose
// final segment begins with one — which is what makes a stale slug harmless.
func identifierIn(ref string) (gnosis.ID, bool) {
	candidate := strings.TrimSuffix(path.Base(strings.TrimSpace(ref)), ".md")
	if len(candidate) < idLength {
		return "", false
	}
	id, err := gnosis.ParseID(candidate[:idLength])
	if err != nil {
		return "", false
	}
	return id, true
}
