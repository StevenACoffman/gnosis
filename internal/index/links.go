package index

import (
	"context"
	"fmt"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// LinkRow is one link as found in a document body.
//
// TargetID is empty when the href resolves to no document in the bundle, which
// OKF §6.1 calls not-yet-written knowledge and this design treats as a gap rather
// than an error. Href is retained either way, and matters most in exactly that
// case: it is then the only surviving record of what the author meant.
type LinkRow struct {
	SourceID gnosis.ID
	TargetID gnosis.ID
	Href     string
	External bool
}

// Resolved is one link with its target's title, for rendering inline.
type Resolved struct {
	Href     string `json:"href"`
	TargetID string `json:"target_id,omitempty"`
	Title    string `json:"title,omitempty"`
	External bool   `json:"external"`
}

// String renders a link for a person, saying plainly when it leads nowhere.
//
// It lives here rather than in a command because two commands render links and a
// third will: the family's rule is that a thing moves on its second consumer, and
// two spellings of "this link goes nowhere" is precisely the drift that rule
// exists to prevent.
func (r Resolved) String() string {
	switch {
	case r.External:
		return r.Href + " (external)"
	case r.Title != "":
		return r.Href + " — " + r.Title
	default:
		return r.Href + " (no such document yet)"
	}
}

// Outbound lists the links leaving a document, with each target resolved.
//
// Requires: db is open.
// Ensures: returns an empty slice, never nil. A link whose target is absent is
// returned with an empty TargetID and Title rather than omitted — SPEC §8.3
// requires a reader to be able to follow links without re-querying, and a gap is
// something a reader needs to see, not something to hide.
func (db *DB) Outbound(ctx context.Context, id gnosis.ID) ([]Resolved, error) {
	const op = "index.DB.Outbound"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT l.href, COALESCE(l.target_document_id, ''), COALESCE(d.title, ''), l.external
		FROM links l
		LEFT JOIN documents d ON d.id = l.target_document_id
		WHERE l.source_document_id = ?
		ORDER BY l.href`, id.String())
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]Resolved, 0)
	for rows.Next() {
		var r Resolved
		if err := rows.Scan(&r.Href, &r.TargetID, &r.Title, &r.External); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// Inbound lists the documents linking to id, for orphan and hub reporting.
//
// Requires: db is open.
// Ensures: returns an empty slice, never nil; ordered by path so two readings
// agree.
//
// Each result describes the **source** document — its path and title — not the
// link's href. The href on an inbound link points back at the document being
// shown, which the reader is already looking at; what they need is where the
// link came from.
func (db *DB) Inbound(ctx context.Context, id gnosis.ID) ([]Resolved, error) {
	const op = "index.DB.Inbound"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT d.path, d.id, d.title, l.external
		FROM links l
		JOIN documents d ON d.id = l.source_document_id
		WHERE l.target_document_id = ?
		ORDER BY d.path`, id.String())
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]Resolved, 0)
	for rows.Next() {
		var r Resolved
		if err := rows.Scan(&r.Href, &r.TargetID, &r.Title, &r.External); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// insertLink writes one link row.
func insertLink(ctx context.Context, tx execer, l *LinkRow) error {
	var target any
	if l.TargetID != "" {
		target = l.TargetID.String()
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO links (source_document_id, target_document_id, href, external)
		VALUES (?, ?, ?, ?)`,
		l.SourceID.String(), target, l.Href, l.External)
	if err != nil {
		return fmt.Errorf("insert link from %s: %w", l.SourceID, err)
	}
	return nil
}
