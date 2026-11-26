package splitcmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toerr "github.com/StevenACoffman/toerr/errors"
)

// What becomes of a split source's original under --originals.
const (
	originalMoved    = "moved"
	originalAlready  = "already"  // the same bytes were already there; the source copy is removed
	originalConflict = "conflict" // other bytes are there; the original is kept where it is
	originalKept     = "kept"     // a part is in conflict, so the parts on disk are not this split's
)

// placementRoots are --sources and --originals made absolute; originals is "" when
// --originals is not set.
type placementRoots struct {
	sources, originals string
}

// placement is where one source's parts are written and where its original goes.
type placement struct {
	// partsDir is the directory the parts are written to.
	partsDir string

	// moveTo is where the original moves once it is split, or "" to leave it where
	// it is: without --originals, or for an original already under it.
	moveTo string
}

// locate places a source given --sources and --originals.
//
// Requires: path, sources and originals are absolute and clean; originals is ""
// when --originals is not set.
// Ensures: without originals, parts go beside path and nothing moves. A path under
// sources has its parts beside it and moves to the same relative path under
// originals. A path under originals is an original moved by an earlier split: its
// parts go to the same relative directory under sources, and it stays. Any other
// path is refused, with the reason. Pure.
//
// originals is tried first, so it may sit inside sources, as it does when sources
// is the working directory.
//
// A refusal is a finding on the source rather than an error for the run, as the
// planner's is: the other sources are unaffected.
func locate(path, sources, originals string) (placement, string) {
	if originals == "" {
		return placement{partsDir: filepath.Dir(path)}, ""
	}
	if rel, ok := within(originals, path); ok {
		return placement{partsDir: filepath.Join(sources, filepath.Dir(rel))}, ""
	}
	if rel, ok := within(sources, path); ok {
		return placement{
			partsDir: filepath.Dir(path),
			moveTo:   filepath.Join(originals, rel),
		}, ""
	}
	return placement{}, fmt.Sprintf("under neither --sources %s nor --originals %s, "+
		"so its original has no place to move to; nothing was written", sources, originals)
}

// within returns path relative to dir when path lies inside dir.
func within(dir, path string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "." || rel == ".." ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// decideMove picks what happens to a split source's original.
//
// Requires: exists reports whether a file is at the destination, and existing is
// its content when it does; conflicts counts the parts in conflict.
// Ensures: kept while any part is in conflict; moved when nothing is at the
// destination; already when the destination holds the same bytes; conflict
// otherwise. Pure.
//
// An original moves only once every part on disk is this split's, because while a
// part conflicts the original is still the only faithful copy of that stretch. A
// file already at the destination is never overwritten, --replace or not: it is
// not a part split wrote, so nothing records that it may be replaced.
func decideMove(original, existing []byte, exists bool, conflicts int) string {
	switch {
	case conflicts > 0:
		return originalKept
	case !exists:
		return originalMoved
	case bytes.Equal(existing, original):
		return originalAlready
	default:
		return originalConflict
	}
}

// moveOriginal carries out decideMove's choice for the original at path, creating
// the destination's directories as needed. A dry run decides without moving.
func (c *Config) moveOriginal(path, to string, original []byte, conflicts int) (Original, error) {
	existing, err := os.ReadFile(to)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Original{}, toerr.WrapWithMessage(err, "split: read "+to)
	}
	o := Original{To: to, State: decideMove(original, existing, exists, conflicts)}
	if c.DryRun {
		return o, nil
	}
	switch o.State {
	case originalMoved:
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return Original{}, toerr.WrapWithMessage(err, "split: create "+filepath.Dir(to))
		}
		if err := os.Rename(path, to); err != nil {
			return Original{}, toerr.WrapWithMessage(err, "split: move "+path)
		}
	case originalAlready:
		if err := os.Remove(path); err != nil {
			return Original{}, toerr.WrapWithMessage(err, "split: remove "+path)
		}
	}
	return o, nil
}
