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
	"time"

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

// versionKey is the frontmatter field recording which corpus conventions a
// document was written under (SPEC §5.5.1.1).
const versionKey = "gnosis_schema_version"

// Document is one file as read from disk, before any interpretation.
//
// SchemaVersion is a pointer because absent and zero are different states: a
// document written before versioning existed carries no key, and one written
// under version 0 does not exist. §5.5.1.1 depends on telling those apart, since
// the documents with no version are exactly the ones the check is looking for.
type Document struct {
	Path          string
	ID            gnosis.ID
	Type          gnosis.TypeKey
	Title         string
	Hash          string
	Bytes         int
	Body          string
	SchemaVersion *int
	Invalid       error

	// Claims are the document's declared claims and where they say their
	// evidence lives (§5.5.1). Empty for a document declaring none.
	Claims []DocClaim

	// StaleAfter is the OKF date the author asked for this to be revisited by,
	// or the zero time when they declared none.
	StaleAfter time.Time

	// SourceKeys identify the source versions this document rests on, keyed as
	// checked.jsonl keys them.
	SourceKeys []string

	// Limitations is what this concept declares it does **not** cover (§17.2).
	// Empty for a document declaring none, which is a finding only on a normative
	// type.
	Limitations []string
}

// DocClaim is a claim's identity and its evidence addresses, as the document
// states them.
//
// It is deliberately narrower than gate.Claim: the checks that read this need to
// know which claim named which archived file and nothing else, and a wider type
// would invite a check to start judging evidence, which is the gate's job.
type DocClaim struct {
	ID string

	// Anchor is the span of the body this claim addresses (§5.5.1). The
	// claim-anchor check needs it to say whether the address still resolves, which
	// is a question about the document rather than about the evidence.
	Anchor string

	// Quotes are the passages this claim offers as evidence, as frontmatter declares
	// them. §17.3.1's sufficiency check weighs them against the claim's wording.
	Quotes []string

	// Lead is the claim's conclusion, stated first (§17.4). Empty until extraction
	// writes one; §5.5.3 makes that a NULL column rather than an empty string.
	Lead string

	// Verified are OKF §5.2's verification events for this claim.
	//
	// **Per claim, and a document-level `verified` is not expanded into it.** OKF puts
	// the key at document level; §5.5 keys the table by claim; and inheriting one to
	// the other would assert that somebody verified each claim when they verified a
	// page. §5.5.1 refused exactly that for `subject` — editing an inherited default
	// silently re-subjects every claim that did not override — and §5.5's own reason
	// for making this a table rather than a column is that a human sign-off and an
	// automated pass must stay distinguishable. Expanding a page-level list one level
	// down destroys that distinction wholesale.
	Verified []Verification

	// Subject is the surface phrase naming what this claim is about (§5.5.1), as
	// the author wrote it — an alias is resolved by the checks, not here. Empty for
	// a claim declaring none, which is a review signal on some types and silence on
	// others (§5.8.3).
	//
	// **It is deliberately not on gate.Claim.** The backlog entry asked for both
	// readers, and that would hand the promote gate a field §5.8.3 forbids it to act
	// on: a claim with no subject is reported for review and never blocked, because
	// blocking would make the corpus refuse ordinary knowledge. The comment on
	// docClaims already gives the general form of this — sharing a type would give a
	// check the fields to start judging.
	Subject string

	ArchivePaths []string
}

// Verification is one OKF §5.2 verification event.
//
// A struct rather than a pair of parallel slices because the two belong together: an
// actor with no time and a time with no actor are both meaningless, and a shape that can
// hold either is a shape a reader has to check.
type Verification struct {
	// By is the actor, as OKF §7's grammar writes it. Kept as the raw string rather
	// than parsed into gnosis.Actor: §14.1.1 makes these two populations, and the
	// tier fold reads the raw form deliberately.
	By string

	// At is the event's timestamp, as declared. A string rather than a time.Time
	// because it is projected into the index verbatim and never compared here — and
	// parsing it would make a malformed date drop a verification silently.
	At string
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
	for i := range docs {
		out = append(out, gnosis.Observed{Path: docs[i].Path, ID: docs[i].ID})
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
	if v, ok := parsed.Int(versionKey); ok {
		doc.SchemaVersion = &v
	}

	doc.Claims = docClaims(parsed)
	doc.Limitations = stringsOf(parsed.Fields, limitationsKey)
	doc.StaleAfter = staleAfter(parsed)
	doc.SourceKeys = sourceKeys(doc.Claims)

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
	for i := range docs {
		d := &docs[i]
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
