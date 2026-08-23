package bundle

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/skillet/errs"
)

// stateDir holds everything derived and gitignored (SPEC §4).
const stateDir = ".gnosis"

// indexFile is the derived database, inside stateDir.
const indexFile = "index.db"

// IndexState is what a read-only caller needs to know about the index.
//
// Present is carried separately from Rows because an absent index and an empty
// one call for different reporting: the first means "not built yet", the second
// means "built, and describes nothing".
type IndexState struct {
	Rows    []gnosis.Indexed
	Present bool
}

// IndexPath is where the index lives beneath a bundle.
//
// It takes an operating-system path rather than an fs.FS because SQLite opens a
// file by name; that is the one place in this package where a bundle cannot be
// addressed as a filesystem.
func IndexPath(bundleDir string) string {
	return filepath.Join(bundleDir, stateDir, indexFile)
}

// OpenIndex opens the index beneath a bundle for writing, creating the state
// directory if it is absent.
//
// Requires: bundleDir exists and is writable.
// Ensures: the returned database is migrated to the current schema. The state
// directory is created here rather than by `init`, so that a bundle cloned
// without one — it is gitignored, so no clone has one — still works on first
// use. The caller closes the database.
func OpenIndex(ctx context.Context, bundleDir string) (*index.DB, error) {
	const op = "bundle.OpenIndex"

	dir := filepath.Join(bundleDir, stateDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	db, err := index.Open(ctx, filepath.Join(dir, indexFile))
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return db, nil
}

// LoadIndex reads the index rows without creating an index.
//
// Requires: nothing; bundleDir need not exist.
// Ensures: Present is false, with no error and no file created, when the bundle
// has no index. That is the distinction this function exists for — a command
// that only reads the corpus must not leave a database behind as a side effect
// of having looked.
//
// Opening an index that does exist applies any pending migrations, which is a
// write. That is deliberate and safe: the index is a derived cache (SPEC §4.5),
// so bringing its schema forward loses nothing, and the alternative — every read
// command failing until someone runs `rebuild` — would make an upgrade feel like
// a breakage.
func LoadIndex(ctx context.Context, bundleDir string) (IndexState, error) {
	const op = "bundle.LoadIndex"

	if _, err := os.Stat(IndexPath(bundleDir)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return IndexState{}, nil
		}
		return IndexState{}, &errs.Error{Op: op, Err: err}
	}

	db, err := index.Open(ctx, IndexPath(bundleDir))
	if err != nil {
		return IndexState{}, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Indexed(ctx)
	if err != nil {
		return IndexState{}, &errs.Error{Op: op, Err: err}
	}
	return IndexState{Rows: rows, Present: true}, nil
}

// OpenIndexForRead opens an existing index without creating one.
//
// Requires: nothing; bundleDir need not exist.
// Ensures: ENOTFOUND when the bundle has no index, and **no directory or database
// is created**. SPEC §4.5 requires that nothing read-only create state, and §4.6
// requires that readers not need the writer; OpenIndex satisfies neither, because
// it makes the state directory and would have a reader racing a writer over it.
//
// Refusing is the point. A reader that created an empty index would answer
// `search` with zero hits, which a caller cannot tell from "no matches" — the
// corpus would appear to contain nothing rather than to be unbuilt. The error
// names the repair instead.
//
// An index that *does* exist is still migrated on open, and that asymmetry is
// deliberate: the index is a derived cache (§4.5), so bringing its schema forward
// loses nothing, and failing every read until someone runs `rebuild` would make an
// upgrade feel like a breakage.
func OpenIndexForRead(ctx context.Context, bundleDir string) (*index.DB, error) {
	const op = "bundle.OpenIndexForRead"

	path := IndexPath(bundleDir)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &errs.Error{
				Code:    errs.ENOTFOUND,
				Message: op + ": no index at " + path + "; run `gnosis index rebuild`",
			}
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	db, err := index.Open(ctx, path)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return db, nil
}
