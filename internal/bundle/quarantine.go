package bundle

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// quarantineDir is tier 1, inside stateDir and therefore gitignored.
//
// SPEC §3083 calls this a decided constraint rather than a default, and the
// reason is worth keeping next to the constant: **unvetted text is text an agent
// will obey.** A coding agent browsing the repository does not know about
// `--include-quarantine`, so putting unvetted content beside vetted content in
// the working tree would undercut the whole of §9.3. Tier 1 is reachable only
// through gnosis commands and never by a filesystem walk of the bundle — which is
// why it lives under `.gnosis/` and not under a sibling directory that merely
// happens to be ignored.
const quarantineDir = "quarantine"

// Quarantine writes a candidate document to tier 1.
//
// Requires: rel is a bundle-relative path with no parent traversal.
// Ensures: the document lands at .gnosis/quarantine/<rel>, mirroring the bundle
// layout so a promotion is a move rather than a translation. Written atomically,
// so an interrupted admit leaves no half-document for the gate to judge.
//
// Unlike StoreEvidence this **does** overwrite. A quarantined document is a draft
// and re-admitting a corrected reply is the ordinary case; tier 0's
// never-overwrite rule exists because a record's path is the hash of its content,
// and this path is not.
func (w *Writer) Quarantine(rel string, content []byte) (string, error) {
	const op = "bundle.Writer.Quarantine"

	if err := w.held(op); err != nil {
		return "", err
	}
	full, err := quarantinePath(op, w.dir, rel)
	if err != nil {
		return "", err
	}
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return "", &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, content, 0o640); wErr != nil {
		return "", &errs.Error{Op: op, Err: wErr}
	}
	return full, nil
}

// ReadQuarantined reads one quarantined document.
//
// Requires: rel is a bundle-relative path.
// Ensures: ENOTFOUND when nothing is quarantined at rel, distinguishable from a
// read failure — a caller promoting a slug that was never admitted has made a
// different mistake from one whose disk is failing.
func ReadQuarantined(bundleDir, rel string) ([]byte, error) {
	const op = "bundle.ReadQuarantined"

	full, err := quarantinePath(op, bundleDir, rel)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &errs.Error{
				Code:    errs.ENOTFOUND,
				Message: op + ": nothing quarantined at " + rel,
			}
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	return data, nil
}

// Quarantined lists what is waiting in tier 1, as bundle-relative paths.
//
// Requires: nothing; a bundle with no quarantine is not an error.
// Ensures: sorted, and empty rather than nil, so a caller need not distinguish
// "nothing quarantined" from "no result". Sorted because a review queue whose
// order changed between runs would be unusable.
func Quarantined(bundleDir string) ([]string, error) {
	const op = "bundle.Quarantined"

	root := filepath.Join(bundleDir, stateDir, quarantineDir)
	out := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return &errs.Error{Op: op, Err: rerr}
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, &errs.Error{Op: op, Err: err}
	}
	sort.Strings(out)
	return out, nil
}

// Discard removes a quarantined document.
//
// Requires: rel is a bundle-relative path.
// Ensures: removing something absent is not an error — a promotion that already
// cleared the draft and a caller retrying both arrive here, and neither is wrong.
func (w *Writer) Discard(rel string) error {
	const op = "bundle.Writer.Discard"

	if err := w.held(op); err != nil {
		return err
	}
	full, err := quarantinePath(op, w.dir, rel)
	if err != nil {
		return err
	}
	if rErr := os.Remove(full); rErr != nil && !errors.Is(rErr, fs.ErrNotExist) {
		return &errs.Error{Op: op, Err: rErr}
	}
	return nil
}

// quarantineTargetPath is where a quarantined document would land if promoted.
//
// Separate from quarantinePath, which resolves the draft's location inside tier 1.
// Two functions rather than one because confusing them would read the draft when
// the corpus copy was meant, which is how a revision would be mistaken for a
// creation.
func quarantineTargetPath(bundleDir, rel string) string {
	return filepath.Join(bundleDir, filepath.FromSlash(rel))
}

// quarantinePath resolves a bundle-relative path inside tier 1, refusing anything
// that would escape it.
//
// The traversal check is not defensive habit. A quarantined document's path comes
// from a model's reply (§9.4's ingest relay), so `../../etc/whatever` is an input
// this function will actually receive — and tier 1 exists precisely to keep
// untrusted content from reaching the working tree.
func quarantinePath(op, bundleDir, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if rel == "" || filepath.IsAbs(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + rel + " is not a path inside the bundle",
		}
	}
	return filepath.Join(bundleDir, stateDir, quarantineDir, clean), nil
}
