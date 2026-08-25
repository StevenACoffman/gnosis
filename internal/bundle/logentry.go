package bundle

import (
	"os"
	"path/filepath"
	"time"

	"github.com/StevenACoffman/gnosis/internal/okflog"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// LogFile is the corpus history: at the bundle root, and committed.
//
// Exported because two commands name it in their output and one of them used to
// declare its own copy. Three spellings of one filename is the second-place-to-
// remember smell at its cheapest to fix.
const LogFile = "log.md"

// Log files notes in log.md under a date.
//
// Requires: the writer holds the lock; each note is one line of prose.
// Ensures: the file exists afterwards and holds every note under at's date, in the
// order given. An absent log is created, which OKF §9 permits it not to be. No notes
// is a no-op rather than an empty write.
//
// The lock is what this needs from the Writer and the reason it is a method: log.md
// is committed and at the bundle root, so two processes appending at once would
// interleave — §4.6's rule that the writer owns the bundle rather than merely the
// database, applied to the one file every command might want to write.
//
// A read-modify-write rather than an append, because `okflog.Add` files the note
// under the right date heading rather than at the end. That is the same shape
// `checked.jsonl` has and it is safe for the same reason: the lock spans both halves.
// Variadic because `standards check --log` files one entry per loosened threshold and
// the alternative was a loop of read-modify-writes: correct under the lock, and n
// reads and n writes to say one thing.
func (w *Writer) Log(at time.Time, notes ...string) error {
	const op = "bundle.Writer.Log"

	if err := w.held(op); err != nil {
		return err
	}
	if len(notes) == 0 {
		return nil
	}
	full := filepath.Join(w.dir, LogFile)
	src, err := os.ReadFile(full)
	if err != nil && !os.IsNotExist(err) {
		return &errs.Error{Op: op, Err: err}
	}
	day := at.Format(time.DateOnly)
	updated := string(src)
	for _, note := range notes {
		updated = okflog.Add(updated, day, note)
	}
	if wErr := atomicfile.WriteFile(full, []byte(updated), 0o640); wErr != nil {
		return &errs.Error{Op: op, Err: wErr}
	}
	return nil
}
