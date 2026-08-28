package bundle

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/skillet/errs"
)

// SpendPrompt removes an answered prompt and its metadata.
//
// Requires: the writer holds the lock; key named a prompt this bundle emitted.
// Ensures: neither file is on disk afterwards, and an already-absent file is not an
// error — a prompt removed by an earlier run, or one that was never written because
// its reply was already cached, is the same finished state this produces.
//
// # Why the removal order is the reverse of the write order
//
// `renderPending` writes the metadata first and the prompt second, so a crash between
// them leaves an inert meta describing a prompt that is not there. Removal takes the
// prompt first, so a crash between *these* leaves the **same** inert state rather than
// its opposite: a prompt an agent can read and answer whose meta `admit` would refuse.
// The two orders are chosen together, and reversing either one produces the state
// neither wants.
//
// # Best-effort, and what that does not extend to
//
// The caller ignores the error, because the reply is cached and the document is
// filed: reporting a cleanup failure as the operation's would tell a caller to retry
// something that succeeded, which is the rule `Coordinator.record` already follows for
// the audit row. The error is returned rather than swallowed here so the caller can
// say so on stderr — silence about a failure is different from deciding it is not
// fatal.
func (w *Writer) SpendPrompt(key string) error {
	const op = "bundle.Writer.SpendPrompt"

	if err := w.held(op); err != nil {
		return err
	}
	for _, rel := range []string{promptPath(key), promptMetaPath(key)} {
		full := filepath.Join(w.dir, filepath.FromSlash(rel))
		if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return &errs.Error{Op: op, Err: err}
		}
	}
	return nil
}

// promptPath is where a key's prompt lives.
//
// It exists so the writer and the remover cannot disagree about where a prompt is:
// the path was built inline where it was written, which is fine until a second place
// needs it and then is exactly the duplication that goes stale.
func promptPath(key string) string {
	return filepath.ToSlash(filepath.Join(stateDir, promptDir, key+".md"))
}
