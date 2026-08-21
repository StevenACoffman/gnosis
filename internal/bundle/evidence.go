package bundle

import (
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
// An existing record is never overwritten, even when its bytes differ. The path
// is the hash of the content, so differing bytes at one path is corruption or
// tampering rather than an update, and silently replacing it would erase the only
// evidence that it happened. Verify reports it instead.
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

// writeOnce writes data at rel unless a file is already there, and reports
// whether it wrote.
func writeOnce(op, bundleDir, rel string, data []byte) (bool, error) {
	full := filepath.Join(bundleDir, filepath.FromSlash(rel))

	switch _, err := os.Stat(full); {
	case err == nil:
		return false, nil
	case !errors.Is(err, fs.ErrNotExist):
		return false, &errs.Error{Op: op, Err: err}
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return false, &errs.Error{Op: op, Err: err}
	}
	if err := atomicfile.WriteFile(full, data, 0o640); err != nil {
		return false, &errs.Error{Op: op, Err: err}
	}
	return true, nil
}
