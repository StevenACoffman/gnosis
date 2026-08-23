package bundle

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// Stored is what a store attempt did, which is frequently nothing.
//
// Wrote is carried separately from the paths because "already present" is the
// expected outcome of a staleness sweep and is not a failure. A caller reporting
// "3 sources fetched" when three records already existed would be reporting work
// that did not happen (§9.2).
type Stored struct {
	// RecordPath is where the ledger entry lives, relative to the bundle root.
	RecordPath string

	// ContentPath is where the archived text lives, or empty for a referenced
	// source that stored none.
	ContentPath string

	// Wrote reports whether anything reached the disk.
	Wrote bool
}

// StoreEvidence writes a fetch record and its archived text, if they are not
// already there.
//
// Requires: bundleDir exists and is writable; out came from archive.Decide.
// Ensures: idempotent. A re-fetch of unchanged bytes produces the same paths,
// finds them present, and writes nothing — reported as Wrote false, which is what
// §9.2 means by a no-op. Both files are written atomically, so an interrupted
// fetch leaves no half-record for the next run to read as complete.
//
// An existing file is never overwritten, and one holding *different* bytes is an
// ECONFLICT rather than a quiet no-op. The path is the hash of the content, so
// differing bytes there is corruption rather than an update; silently replacing it
// would erase the evidence, and silently accepting it would report a corrupt corpus
// as unchanged. See writeOnce.
func StoreEvidence(bundleDir string, out *archive.Outcome) (Stored, error) {
	const op = "bundle.StoreEvidence"

	recordPath, err := out.Record.Path()
	if err != nil {
		return Stored{}, &errs.Error{Op: op, Err: err}
	}
	canonical, err := out.Record.Canonical()
	if err != nil {
		return Stored{}, &errs.Error{Op: op, Err: err}
	}

	stored := Stored{RecordPath: recordPath, ContentPath: out.Record.ArchivePath}
	// The content is written before the record, so a crash between the two leaves
	// orphaned text rather than a record pointing at text that is not there. An
	// orphan is inert; a dangling record is a claim of evidence that cannot be
	// produced.
	if out.Content != nil {
		wrote, err := writeOnce(op, bundleDir, out.Record.ArchivePath, out.Content)
		if err != nil {
			return Stored{}, err
		}
		stored.Wrote = wrote
	}
	wrote, err := writeOnce(op, bundleDir, recordPath, canonical)
	if err != nil {
		return Stored{}, err
	}
	stored.Wrote = stored.Wrote || wrote
	return stored, nil
}

// writeOnce writes data at rel unless something is already there.
//
// Requires: rel is bundle-relative; data is what belongs at rel.
// Ensures: reports true when it wrote, false when the file was already
// byte-for-byte data, and ECONFLICT when a file is there and differs. It never
// overwrites, whichever of those it found.
//
// **Differing bytes at one of these paths is corruption, not an update**, and
// that is what the third case exists to say. Every path this writes to is the
// hash of its own content — a fetch record's name is the sha256 of the record,
// and an archived text's name is the sha256 of the text — so two different
// contents cannot legitimately arrive at one path. Something rewrote the file
// without recomputing its name, or the file was damaged.
//
// Reporting that as "already present" is the failure this exists to prevent.
// §4.3.1 rests the whole append-only argument on a rewritten record landing
// somewhere else, and a writer that finds a mismatch and says nothing turns that
// argument into a property nobody checks. The comparison costs one read of a file
// bounded by the per-file cap, and only on the path that would otherwise have done
// no work at all.
func writeOnce(op, bundleDir, rel string, data []byte) (bool, error) {
	full := filepath.Join(bundleDir, filepath.FromSlash(rel))

	existing, err := os.ReadFile(full)
	switch {
	case err == nil && bytes.Equal(existing, data):
		return false, nil
	case err == nil:
		return false, &errs.Error{
			Code: errs.ECONFLICT,
			Message: op + ": " + rel + " holds different bytes than its own name" +
				"; a content-addressed path cannot be updated, so this is corruption",
		}
	case !errors.Is(err, fs.ErrNotExist):
		return false, &errs.Error{Op: op, Err: err}
	}
	if err = os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return false, &errs.Error{Op: op, Err: err}
	}
	if err = atomicfile.WriteFile(full, data, 0o640); err != nil {
		return false, &errs.Error{Op: op, Err: err}
	}
	return true, nil
}
