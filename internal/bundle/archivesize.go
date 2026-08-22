package bundle

import (
	"errors"
	"io/fs"
	"os"

	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/errs"
)

// evidenceDir is the whole of tier 0, both the records and the archived text.
const evidenceDir = "evidence"

// MeasureArchive reports what the evidence store costs and what the largest
// files in it are.
//
// Requires: bundleDir is a bundle root, which need not have an archive.
// Ensures: a zero total and no files for a corpus that has fetched nothing.
// Largest is sorted biggest first and bounded, because §9.2 requires a warning to
// name what is large — a caller told the archive is big and not told what is big
// in it has to go and look, which is the work the report was supposed to save.
//
// It walks rather than querying `sources_fetched`, and the reason is not
// laziness. That table records each source's **fetched** byte size, and for an
// `extracted` record the archived file is the extraction rather than the source —
// often a tenth the size, sometimes larger. Summing the column would report a
// number that is not what the repository holds. The walk is a stat per file on a
// `doctor` path, which is not hot.
func MeasureArchive(
	bundleDir string,
	budget int64,
	warnFraction float64,
) (lint.ArchiveSize, error) {
	const op = "bundle.MeasureArchive"

	out := lint.ArchiveSize{Budget: budget, WarnFraction: warnFraction}
	fsys := os.DirFS(bundleDir)

	err := fs.WalkDir(fsys, evidenceDir, func(name string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return fs.SkipAll
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			return &errs.Error{Op: op, Message: op + ": " + name, Err: iErr}
		}
		out.Bytes += info.Size()
		out.Largest = append(out.Largest, lint.ArchiveFile{Path: name, Bytes: info.Size()})
		return nil
	})
	if err != nil {
		return lint.ArchiveSize{}, &errs.Error{Op: op, Err: err}
	}

	lint.SortArchiveFiles(out.Largest)
	// Keep only what a message will use. Holding every path would make the
	// envelope of a large corpus mostly a directory listing.
	const keep = 5
	if len(out.Largest) > keep {
		out.Largest = out.Largest[:keep]
	}
	return out, nil
}
