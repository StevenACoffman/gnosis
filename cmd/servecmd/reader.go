package servecmd

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/web"
	"github.com/StevenACoffman/skillet/errs"
)

// reader serves the viewer, and holds no writer.
//
// §4.6: "readers stay independent of [the coordinator] by requirement". This opens the
// index for reading on each request rather than holding a handle, which is the
// conservative choice for a long-lived process: an `index rebuild` replaces the file, and
// a server holding the old one would serve a corpus that no longer exists until it was
// restarted.
type reader struct {
	dir string
}

// Concept renders one document by identifier or path.
//
// Requires: id is an identifier, a current path, or a stale path the index can resolve.
// Ensures: ENOTFOUND when the corpus holds none, which the handler turns into a 404 —
// a missing page is an answer rather than a failure.
func (r *reader) Concept(ctx context.Context, id string) (*web.Page, error) {
	const op = "servecmd.reader.Concept"

	db, err := bundle.OpenIndexForRead(ctx, r.dir)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = db.Close() }()

	detail, err := db.Find(ctx, id)
	if err != nil {
		// Wrapped and not reclassified: Find reports ENOTFOUND for a reference the
		// corpus does not hold, and the wrap preserves the code — which is what lets
		// one translation function turn it into a 404 rather than a 500.
		return nil, &errs.Error{Op: op, Err: err}
	}
	page := &web.Page{
		ID: detail.ID, Path: detail.Path, Title: detail.Title,
		Links: resolvedLinks(detail.Outbound),
	}
	r.describe(page, detail)
	return page, nil
}

// Search answers at document grain, as §13's "same ladder as the CLI" requires.
func (r *reader) Search(ctx context.Context, query string, limit int) ([]web.Hit, error) {
	const op = "servecmd.reader.Search"

	db, err := bundle.OpenIndexForRead(ctx, r.dir)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = db.Close() }()

	hits, err := db.Search(ctx, query, limit)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	out := make([]web.Hit, 0, len(hits))
	for _, hit := range hits {
		out = append(out, web.Hit{ID: hit.ID, Path: hit.Path, Title: hit.Title})
	}
	return out, nil
}

// describe adds the signals §13 requires beside a concept.
//
// **A failure to compute one is not a failure to render the page.** The trust tier and
// the freshness are derived, and a corpus whose index is mid-rebuild can still show its
// prose — the zero values read as `unverified` and `unknown`, which are the honest
// answers when the lookup did not happen, and both types were built so that their zero
// value claims nothing.
func (r *reader) describe(page *web.Page, detail *index.Detail) {
	body, doc, err := r.document(detail.Path)
	if err == nil {
		page.Body = body
		page.Type = doc
	}
	if trust, tErr := bundle.TrustFor(r.dir, detail.Path); tErr == nil {
		page.Trust = trust.State
	}
	if fresh, fErr := bundle.FreshnessFor(r.dir, detail.Path, time.Now().UTC()); fErr == nil {
		page.Freshness = fresh.State.String()
	}
	// The vocabulary the page's terms are defined from. A corpus with no ontology
	// yields none, and the panel is then absent rather than empty — "a glossary nobody
	// opens is not an ontology", and an empty one is worse than none.
	page.Terms = bundle.DeclaredTerms(os.DirFS(r.dir))
}

// document reads a concept's body and its declared type.
func (r *reader) document(rel string) (body, docType string, err error) {
	const op = "servecmd.reader.document"

	docs, err := bundle.Load(os.DirFS(r.dir))
	if err != nil {
		return "", "", &errs.Error{Op: op, Err: err}
	}
	for i := range docs {
		if docs[i].Path == rel {
			return docs[i].Body, string(docs[i].Type), nil
		}
	}
	return "", "", &errs.Error{
		Code: errs.ENOTFOUND, Message: op + ": " + rel + " is indexed and not on disk",
	}
}

// resolvedLinks presents the outbound links §8.3 requires rendered inline.
//
// **External links are dropped and internal ones carry a title and an identifier.** The
// requirement is that a reader can follow a link without "emerging from the system and
// re-entering on a new path"; a bare external href does not participate in that, and one
// pointing at a target the corpus cannot resolve would be a link to nothing wearing a
// title.
func resolvedLinks(outbound []index.Resolved) []web.Link {
	var out []web.Link
	for _, link := range outbound {
		if link.External || link.TargetID == "" {
			continue
		}
		out = append(out, web.Link{
			ID: link.TargetID, Title: link.Title, Path: filepath.ToSlash(link.Href),
		})
	}
	return out
}
