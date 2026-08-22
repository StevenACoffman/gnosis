package bundle

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/quotecheck"
)

// evidenceKey is the frontmatter field carrying a claim's quotations (§5.5).
const evidenceKey = "gnosis_evidence"

// sourcesKey is OKF's own sources list.
const sourcesKey = "sources"

// candidate assembles the diff the gate judges.
//
// Requires: after is the exact bytes that would be written.
// Ensures: Doc is after, parsed. The parse happens here rather than in the gate
// because `internal/okf` is a sibling adapter and adapters do not import each
// other (PLAN §0.1) — this shell is the one layer allowed to know both.
//
// A document that will not parse is not an error here. It is a candidate whose
// conformance signal will fail, which is the right place for it to be reported: an
// error would tell a caller the tool broke, and what actually happened is that
// their document is malformed.
func (c *Coordinator) candidate(rel string, before, after []byte) *gate.Candidate {
	cand := &gate.Candidate{Path: rel, Before: before, After: after}

	doc, err := okf.Parse(after)
	if err != nil {
		// Leave Doc zero: conformance reports the missing type, title, and body.
		// There is deliberately no error return — a malformed candidate is a
		// verdict, not a fault, and an error here would tell a caller the tool
		// broke when what happened is that their document did.
		return cand
	}
	title, _ := doc.Text("title")
	cand.Doc = gate.Document{
		Type:    doc.Type(),
		Title:   title,
		Body:    doc.Body,
		Claims:  claimsOf(doc),
		Sources: sourcesOf(doc),
	}
	return cand
}

// gateInputs gathers what the signals need to know about everything else.
//
// Requires: the writer lock is held, so the corpus cannot change underneath the
// gate between this call and the write.
// Ensures: a Corpus and Limits built from the bundle as it currently is. The
// archived text is read eagerly rather than lazily because the gate is pure and
// must not perform I/O — the cost is bounded by tier 0, which the per-corpus
// budget bounds in turn (§4.3).
func (c *Coordinator) gateInputs() (*gate.Corpus, gate.Limits, error) {
	const op = "bundle.Coordinator.gateInputs"

	text, err := archivedText(op, c.Dir)
	if err != nil {
		return nil, gate.Limits{}, err
	}
	uris, err := fetchedURIs(op, c.Dir)
	if err != nil {
		return nil, gate.Limits{}, err
	}
	titles, err := titlesByFold(c.Dir)
	if err != nil {
		return nil, gate.Limits{}, err
	}

	// Provisional until standards/promote.toml exists, which is recorded in TODO.
	// MinPassageWords comes from quotecheck so the gate and the guard cannot
	// disagree about how short a passage is too short to be evidence.
	limits := gate.Limits{
		HedgingMax:      3,
		MinPassageWords: quotecheck.MinPassageWords,
	}

	return &gate.Corpus{
		ArchivedText: text,
		FetchedURIs:  uris,
		TitlesByFold: titles,
	}, limits, nil
}

// archivedText reads every file under evidence/text into memory.
// The walk is rooted at an fs.FS rather than at an operating-system path, so a
// symlink inside evidence/ cannot lead the reader out of the bundle. That is the
// same posture bundle.Load takes and the reason it is worth the small awkwardness
// of slash-only paths.
func archivedText(op, bundleDir string) (map[string]string, error) {
	out := map[string]string{}
	fsys := os.DirFS(bundleDir)

	err := fs.WalkDir(fsys, archive.TextDir, func(name string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// A corpus that has fetched nothing yet has no evidence directory,
			// and that is not a failure to read one.
			return fs.SkipAll
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		data, rerr := fs.ReadFile(fsys, name)
		if rerr != nil {
			return &errs.Error{Op: op, Err: rerr}
		}
		out[name] = string(data)
		return nil
	})
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}

// fetchedURIs is every source tier 0 holds a record for, whatever the
// disposition. A `referenced` source is still provenance.
func fetchedURIs(op, bundleDir string) (map[string]bool, error) {
	out := map[string]bool{}
	err := walkRecords(op, os.DirFS(bundleDir), func(rec *archive.Record) {
		out[rec.URI] = true
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkRecords calls visit for every fetch record in tier 0.
//
// Requires: fsys is rooted at the bundle.
// Ensures: a bundle that has fetched nothing is not an error. **A malformed record
// is**, and is not skipped quietly: a reader that stepped over one would report a
// source as never fetched when in fact its record exists and cannot be read, which
// sends somebody to re-fetch a source that is already there rather than to the
// corruption.
//
// The walk is rooted at an fs.FS rather than an operating-system path, so a symlink
// inside evidence/ cannot lead it out of the bundle.
func walkRecords(op string, fsys fs.FS, visit func(*archive.Record)) error {
	err := fs.WalkDir(fsys, archive.FetchDir, func(name string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return fs.SkipAll
		case err != nil:
			return err
		case d.IsDir() || !strings.HasSuffix(name, ".json"):
			return nil
		}
		data, rerr := fs.ReadFile(fsys, name)
		if rerr != nil {
			return &errs.Error{Op: op, Err: rerr}
		}
		rec, rerr := archive.ParseRecord(data)
		if rerr != nil {
			return &errs.Error{Op: op, Message: op + ": " + name, Err: rerr}
		}
		visit(&rec)
		return nil
	})
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	return nil
}

// SourceRows projects every committed fetch record for the derived index.
//
// Requires: bundleDir is a bundle root.
// Ensures: one row per record, in the order the walk found them; the caller sorts
// if it cares. A bundle with no archive yields none rather than an error, which is
// the ordinary state of a corpus that has fetched nothing.
//
// This reads the **records**, not the index, because the index is what it is
// building. §4.3.1 makes the committed records authoritative and the table a
// rollup, so a rebuild that read the previous table would be copying a cache
// forward rather than re-deriving it — and the byte-identical guarantee of §4.5
// would then hold over whatever the last run happened to leave behind.
func SourceRows(bundleDir string) ([]index.SourceRow, error) {
	const op = "bundle.SourceRows"

	var out []index.SourceRow
	err := walkRecords(op, os.DirFS(bundleDir), func(rec *archive.Record) {
		hash, hErr := rec.Hash()
		if hErr != nil {
			// Unreachable: the record parsed, so it marshals. Skipping rather than
			// failing would drop a source from the projection silently, so the row
			// is written with an empty key and the primary key rejects a second one
			// — visible, rather than absent.
			hash = ""
		}
		out = append(out, index.SourceRow{
			RecordSHA256:     hash,
			URI:              rec.URI,
			SourceSHA256:     rec.SourceSHA256,
			ByteSize:         rec.ByteSize,
			MediaType:        rec.MediaType,
			Disposition:      string(rec.Disposition),
			ArchivePath:      rec.ArchivePath,
			Extractor:        rec.Extractor,
			ExtractorVersion: rec.ExtractorVersion,
			ExtractedFrom:    rec.ExtractedFrom,
			RejectReason:     string(rec.RejectReason),
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// titlesByFold maps each existing document's folded title to its path.
//
// gnosis.Surface.Fold is the normalizer, matching what the duplication signal
// uses. They must agree, and the gate's doc comment says so; this is the other
// half of that agreement.
func titlesByFold(bundleDir string) (map[string][]string, error) {
	const op = "bundle.titlesByFold"

	docs, err := Load(os.DirFS(bundleDir))
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	out := map[string][]string{}
	for i := range docs {
		d := &docs[i]
		if d.Title == "" {
			continue
		}
		key := gnosis.Surface(d.Title).Fold()
		out[key] = append(out[key], d.Path)
	}
	return out, nil
}
