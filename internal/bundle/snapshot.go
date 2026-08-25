package bundle

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/markdown"
)

// conceptPrefix is the bundle-absolute prefix OKF §6.1 recommends for links
// between concepts. A link that does not start with it is treated as external.
const conceptPrefix = "/" + conceptDir + "/"

// Snapshot assembles everything the checks need.
//
// This is the composition the shell exists for: read the bundle, project it into
// the shapes the pure core consumes, and hand over. It decides no severity and
// reports no finding — those belong to internal/lint, which is why this function
// returns a Snapshot rather than a Report.
//
// Requires: fsys is rooted at the bundle; idx describes the same bundle, and its
// zero value states that the bundle has no index.
// Ensures: a bundle with no concepts yields a valid empty Snapshot rather than an
// error, because every command must work against a freshly initialised bundle.
func Snapshot(fsys fs.FS, idx IndexState, fresh FreshnessState) (*lint.Snapshot, error) {
	const op = "bundle.Snapshot"

	docs, err := Load(fsys)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	logLines, hasLog, err := LoadLog(fsys)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	archived, err := archivedPaths(fsys)
	if err != nil {
		return nil, err
	}
	recorded, err := recordedPaths(fsys)
	if err != nil {
		return nil, err
	}

	return &lint.Snapshot{
		Documents:     documents(docs),
		ArchivedText:  archived,
		RecordedText:  recorded,
		SourceChecks:  fresh.Checks,
		StalenessDays: fresh.StalenessDays,
		Links:         links(docs),
		Vocabulary:    vocabulary(fsys),
		Resolutions:   gnosis.Reconcile(Observed(docs), idx.Rows),
		SchemaVersion: gnosis.SchemaVersion,
		LogLines:      logLines,
		HasLog:        hasLog,
		HasIndex:      idx.Present,
	}, nil
}

// documents projects loaded files into the check-facing shape.
func documents(docs []Document) []lint.Document {
	out := make([]lint.Document, 0, len(docs))
	for i := range docs {
		d := &docs[i]
		out = append(out, lint.Document{
			ID: d.ID, Path: d.Path, Type: d.Type, Title: d.Title,
			Body: d.Body, SchemaVersion: d.SchemaVersion,
			Claims:     claimRefs(d.Claims),
			StaleAfter: d.StaleAfter, SourceKeys: d.SourceKeys,
		})
	}
	return out
}

// claimRefs projects a document's claims into the check-facing shape.
func claimRefs(claims []DocClaim) []lint.Claim {
	if len(claims) == 0 {
		return nil
	}
	out := make([]lint.Claim, 0, len(claims))
	for i := range claims {
		out = append(out, lint.Claim{
			ID:           claims[i].ID,
			Anchor:       claims[i].Anchor,
			Subject:      claims[i].Subject,
			ArchivePaths: claims[i].ArchivePaths,
		})
	}
	return out
}

// archivedPaths is the set of archived text files present, for the archive-path
// check to resolve claim addresses against.
//
// Walked through the same fs.FS as the documents, so a check never touches a disk
// and a caller testing with an fstest.MapFS gets the same answers as one reading a
// bundle. An absent archive is an empty set rather than an error: a corpus that has
// fetched nothing has no dangling paths, only claims that will report as dangling
// the moment they name one.
func archivedPaths(fsys fs.FS) (map[string]bool, error) {
	const op = "bundle.archivedPaths"

	out := map[string]bool{}
	err := fs.WalkDir(fsys, archive.TextDir, func(name string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return fs.SkipAll
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		out[name] = true
		return nil
	})
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// recordedPaths is the set of archive paths tier 0's fetch records name.
//
// Requires: fsys is rooted at the bundle.
// Ensures: one entry per distinct archive path any record names, and an empty set
// for a bundle with no records rather than an error — a corpus that has fetched
// nothing has nothing to account for.
//
// A `referenced` record names no archive path and contributes nothing, which is
// correct: it stored no text, so there is no file for it to account for. That is why
// this reads the records rather than counting them.
//
// Walked through the same fs.FS as the documents, so the closure check never touches
// a disk and a caller testing with an fstest.MapFS gets the same answers as one
// reading a bundle.
func recordedPaths(fsys fs.FS) (map[string]bool, error) {
	const op = "bundle.recordedPaths"

	out := map[string]bool{}
	err := walkRecords(op, fsys, func(rec *archive.Record) {
		if rec.ArchivePath != "" {
			out[rec.ArchivePath] = true
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// links extracts every link from every body and resolves each against the
// documents present.
//
// Resolution is by identifier parsed out of the href's filename, not by path
// lookup: a link written before a retitle still points at the old slug, and the
// immutable prefix is what lets it keep working (SPEC §5.1.1). A href naming an
// identifier no document carries resolves to nothing, which is legal.
func links(docs []Document) []lint.Link {
	present := make(map[gnosis.ID]bool, len(docs))
	for i := range docs {
		if docs[i].ID != "" {
			present[docs[i].ID] = true
		}
	}

	out := make([]lint.Link, 0)
	for i := range docs {
		d := &docs[i]
		if d.ID == "" {
			continue
		}
		for _, href := range markdown.Parse(d.Body).Links {
			link := lint.Link{FromID: d.ID, Href: href}
			if target, ok := targetID(href); ok {
				link.External = false
				if present[target] {
					link.ToID = target
				}
			} else {
				link.External = true
			}
			out = append(out, link)
		}
	}
	return out
}

// targetID recovers the identifier a concept link points at.
//
// Requires: nothing.
// Ensures: reports false for any href that is not a bundle-absolute concept
// path, which includes every external URL and every relative link. A concept
// filename is "<uuid>-<slug>.md", so the identifier is the leading segment.
func targetID(href string) (gnosis.ID, bool) {
	if !strings.HasPrefix(href, conceptPrefix) {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(href, conceptPrefix), ".md")
	// The identifier is 36 characters; a slug follows it after a hyphen.
	if len(name) < 36 {
		return "", false
	}
	id, err := gnosis.ParseID(name[:36])
	if err != nil {
		return "", false
	}
	return id, true
}

// LinkRows projects the resolved link graph into index rows.
//
// Requires: docs came from Load.
// Ensures: one row per link found in an identified document's body, with the
// target filled in only where it resolves. Links out of an unidentified document
// are omitted for the same reason its document row is (§5.1.2): it is
// quarantined, not indexed, and indexing its links would give it a presence in
// the graph it has not earned.
func LinkRows(docs []Document) []index.LinkRow {
	out := make([]index.LinkRow, 0)
	for _, l := range links(docs) {
		out = append(out, index.LinkRow{
			SourceID: l.FromID,
			TargetID: l.ToID,
			Href:     l.Href,
			External: l.External,
		})
	}
	return out
}

// Reconciled compares loaded documents against index rows.
//
// Requires: docs came from Load.
// Ensures: the outcome of gnosis.Reconcile over the projection, so a caller need
// not know how an observation is shaped.
func Reconciled(docs []Document, indexed []gnosis.Indexed) []gnosis.Resolution {
	return gnosis.Reconcile(Observed(docs), indexed)
}
