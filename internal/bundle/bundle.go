// Package bundle is the imperative shell over a knowledge base on disk.
//
// It walks the bundle, reads each file, and assembles the values the pure core
// consumes. Every decision about what those values *mean* belongs elsewhere:
// this package chooses no severity, resolves no conflict, and reports no
// finding. If a branch here starts deciding domain meaning, it is in the wrong
// package.
//
// That division is why the packages below it need no filesystem in their tests,
// and why this one is the only Phase 1 package whose tests use t.TempDir().
package bundle

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/identity"
)

// conceptDir is the single directory concepts live in, so a walk need not guess
// which markdown files are concepts and which are reserved.
const conceptDir = "c"

// idKey is the frontmatter field carrying a document's identity (SPEC §5.1).
const idKey = "gnosis_id"

// Document is one file as read from disk, before any interpretation.
type Document struct {
	Path    string
	ID      gnosis.ID
	Type    gnosis.TypeKey
	Title   string
	Hash    string
	Bytes   int
	Body    string
	Invalid error
}

// isReserved reports whether name is one of the filenames OKF §3.1 gives a
// defined meaning; those are never concepts. A function rather than a map var
// because a package-level mutable map is prohibited (rules.md §1).
func isReserved(name string) bool {
	switch name {
	case "index.md", "log.md":
		return true
	default:
		return false
	}
}

// Load reads every concept in the bundle rooted at dir.
//
// Requires: dir exists and is readable.
// Ensures: one Document per non-reserved markdown file under dir/c, in path
// order. A file that fails to parse is returned with Invalid set rather than
// omitted or erroring the whole load — a single malformed document must not make
// the rest of the corpus unreadable, and the caller decides what to report.
// Returns an error only when the walk itself fails.
func Load(fsys fs.FS) ([]Document, error) {
	const op = "bundle.Load"

	var docs []Document
	walkErr := fs.WalkDir(fsys, conceptDir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir(), !strings.HasSuffix(path, ".md"):
			return nil
		case isReserved(filepath.Base(path)):
			return nil
		}
		docs = append(docs, read(fsys, path))
		return nil
	})
	// An absent concept directory is an empty corpus, not a failure: a freshly
	// initialised bundle has one, and every command must work against it.
	if walkErr != nil && !isNotExist(walkErr) {
		return nil, &errs.Error{Op: op, Err: walkErr}
	}
	return docs, nil
}

// LoadLog reads log.md.
//
// Requires: nothing.
// Ensures: present is false when the bundle has no log, which OKF §9 permits and
// which is not a finding. Lines are returned without their terminators.
func LoadLog(fsys fs.FS) (lines []string, present bool, err error) {
	const op = "bundle.LoadLog"
	raw, readErr := fs.ReadFile(fsys, "log.md")
	switch {
	case isNotExist(readErr):
		return nil, false, nil
	case readErr != nil:
		return nil, false, &errs.Error{Op: op, Err: readErr}
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n"), true, nil
}

// Observed projects loaded documents into the shape gnosis.Reconcile consumes.
//
// Requires: docs came from Load.
// Ensures: one Observed per document, preserving an empty ID so the caller can
// tell an unidentified document from an absent one.
func Observed(docs []Document) []gnosis.Observed {
	out := make([]gnosis.Observed, 0, len(docs))
	for _, d := range docs {
		out = append(out, gnosis.Observed{Path: d.Path, ID: d.ID})
	}
	return out
}

// read loads and parses one document. A parse failure is carried on the
// Document rather than returned, so one malformed file cannot make the rest of
// the corpus unreadable.
func read(fsys fs.FS, path string) Document {
	doc := Document{Path: path}

	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		doc.Invalid = err
		return doc
	}
	doc.Bytes = len(raw)
	doc.Hash = identity.Hash(string(raw))

	parsed, err := okf.Parse(raw)
	if err != nil {
		doc.Invalid = err
		return doc
	}
	doc.Body = parsed.Body
	doc.Type = gnosis.TypeKey(parsed.Type())
	if title, ok := parsed.Text("title"); ok {
		doc.Title = title
	}

	// An absent or malformed identifier leaves ID empty, which Reconcile reads
	// as "created outside gnosis" and quarantines. Parsing it here rather than
	// trusting the frontmatter string keeps one definition of a valid identifier.
	if rawID, ok := parsed.Text(idKey); ok {
		if id, idErr := gnosis.ParseID(rawID); idErr == nil {
			doc.ID = id
		} else {
			doc.Invalid = idErr
		}
	}
	return doc
}

// isNotExist reports whether err is a missing-file error from an fs.FS.
func isNotExist(err error) bool {
	return err != nil && errors.Is(err, fs.ErrNotExist)
}

// Rows projects loaded documents into index rows.
//
// Requires: docs came from Load.
// Ensures: documents with no identifier are omitted. An unidentified document is
// quarantined rather than indexed (SPEC §5.1.2), and indexing one would give it
// an identity it never claimed.
func Rows(docs []Document) []index.DocumentRow {
	out := make([]index.DocumentRow, 0, len(docs))
	for _, d := range docs {
		if d.ID == "" {
			continue
		}
		out = append(out, index.DocumentRow{
			ID:    d.ID,
			Path:  d.Path,
			Title: d.Title,
			Slug:  string(gnosis.SlugFrom(d.Title)),
			Hash:  d.Hash,
			Body:  d.Body,
			Bytes: d.Bytes,
		})
	}
	return out
}
