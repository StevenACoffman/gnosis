package bundle

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// lockFile is the advisory writer lock, inside stateDir.
//
// It lives under `.gnosis/`, which is gitignored and derived (SPEC §4). That is
// the right place and not merely a convenient one: a lock is per-user transient
// state, and committing one would export one machine's momentary condition to
// everybody who pulls.
const lockFile = "writer.lock"

// lockPoll is how often TryLockContext retries.
//
// Short enough that a person does not notice the wait behind a fast write, long
// enough that a command blocked behind a long rebuild is not spinning. It is not
// in standards/ because it is not a policy anybody should argue with: it changes
// how quickly a queued writer notices, and nothing about what the tool decides.
const lockPoll = 25 * time.Millisecond

// Writer is exclusive permission to write one bundle, and the only way to do it.
//
// The requirement it implements is easy to state too narrowly, so §4.6 states it
// in full: **the writer owns the bundle, not merely the database.** Serialising
// SQLite writes and leaving markdown writes unserialised would coordinate the
// cache and not the corpus — two agents promoting a claim concurrently is a bundle
// problem, and SQLite's locking has nothing to say about it. So the lock is taken
// over the bundle directory and every write path goes through this type, including
// the ones that touch no database.
//
// # Why this is a type and not a comment
//
// Every write used to be a package function taking a bundle directory, with
// `Requires: the writer lock is held` in its doc comment and nothing behind that
// sentence. Seven functions said it and one caller did not do it: `gnosis fetch`
// wrote tier 0 and rewrote `.gnosis/checked.jsonl` — a read-modify-write over the
// whole file — with no lock at all, so two concurrent fetches could lose one
// user's observations entirely. Nothing reported it, because a prose precondition
// has no failure mode; it just stops being true.
//
// A Writer can only be obtained by taking the lock, and every write is a method on
// it. That does not make the guarantee stronger for the callers that already
// complied — it makes the compiler refuse the ones that do not, which is the
// difference between a documented intent and a checked one. It is the same
// correction §15 records for the audit trail: make it impossible rather than
// tested.
//
// Readers never take it and MUST NOT need it. `lint`, `search`, `show`, and
// `graph` open the index directly, which is why busy_timeout is set (§5.5) and why
// nothing read-only creates state (§4.5): a corpus has to be inspectable when no
// writer is running.
//
// **The ceiling of this mechanism is known and is the reason it is a first step
// rather than the design.** A lock carries no command. It can say that somebody
// else is writing and it cannot say what they are writing, whether their diff was
// approved, or whether it conflicts with this one. §4.6.2's command bus is what
// answers those, and this serialises writes until there is a process to route them
// through.
//
// One write path stays outside the type and the reason is layering, not oversight:
// `index.DB.ReplaceSources` is in a parser package, and PLAN §0.4 forbids a parser
// importing this shell. Its precondition is still prose. Relaxing the depguard rule
// to enforce it here would trade a checked architectural claim for a checked
// precondition, which is a worse bargain than the one comment.
type Writer struct {
	// dir is the bundle this Writer may write. It is derived from the caller's
	// bundle directory at acquisition, so it cannot disagree with the lock's
	// location — the two are the same design decision and are set once, together.
	dir string

	lock *flock.Flock
}

// AcquireWriter takes the bundle's writer lock, waiting until it is free or ctx is
// done.
//
// Requires: bundleDir exists and is writable.
// Ensures: on success the caller holds the bundle exclusively and MUST call
// Release. On a context that expires while waiting, returns an error carrying
// ECONFLICT and gnosis.ReasonWriterBusy's sense — another writer holds it, which
// is a state that resolves on its own, not a fault in the caller's request.
//
// The state directory is created here rather than assumed, because it is
// gitignored and so no fresh clone has one.
func AcquireWriter(ctx context.Context, bundleDir string) (*Writer, error) {
	const op = "bundle.AcquireWriter"

	dir := filepath.Join(bundleDir, stateDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	lock := flock.New(filepath.Join(dir, lockFile))
	held, err := lock.TryLockContext(ctx, lockPoll)
	if err != nil {
		return nil, &errs.Error{Op: op, Code: errs.ECONFLICT, Err: err}
	}
	if !held {
		return nil, &errs.Error{
			Code:    errs.ECONFLICT,
			Message: op + ": another writer holds " + bundleDir,
		}
	}
	return &Writer{dir: bundleDir, lock: lock}, nil
}

// WriterBusy reports whether err is another writer holding the bundle.
//
// Requires: nothing.
// Ensures: it distinguishes contention from failure, which callers need because
// the two call for opposite responses — retry later, versus stop and report. A
// command uses it to choose gnosis.ReasonWriterBusy over a generic error.
func WriterBusy(err error) bool {
	return err != nil && errs.ErrorCode(err) == errs.ECONFLICT
}

// Dir is the bundle this Writer may write.
//
// Requires: nothing; a released or nil Writer reports "".
// Ensures: the same path AcquireWriter was given. It exists because a few write
// paths compose a bundle-relative location and a caller should read it from the
// permission rather than carry a second copy alongside.
func (w *Writer) Dir() string {
	if w == nil {
		return ""
	}
	return w.dir
}

// Release gives up the lock.
//
// Requires: nothing; releasing a nil or already-released Writer is a no-op, so a
// deferred Release needs no guard.
// Ensures: the lock is available to the next writer, and every write method on
// this value refuses from here on. The lock *file* is left in place deliberately —
// removing it races with another process that has just opened it, and an empty file
// under a gitignored directory costs nothing.
func (w *Writer) Release() {
	if w == nil || w.lock == nil {
		return
	}
	_ = w.lock.Unlock()
	w.lock = nil
}

// WriteConcept replaces a concept document's bytes.
//
// Requires: the writer holds the lock; rel is bundle-relative and under the concept
// directory; content is a rendered OKF document.
// Ensures: written atomically, so an interrupted write never leaves half a document
// where a whole one was.
//
// **It refuses a path outside `c/`.** Every caller today passes one from a prompt's
// metadata, which is written by gnosis — but a write method that would follow whatever
// path it was handed is one traversal away from overwriting `ontology.toml`, and the
// guard costs a comparison.
func (w *Writer) WriteConcept(rel string, content []byte) error {
	const op = "bundle.Writer.WriteConcept"

	if err := w.held(op); err != nil {
		return err
	}
	clean := path.Clean(rel)
	if !strings.HasPrefix(clean, conceptDir+"/") || strings.Contains(clean, "..") {
		return &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": " + rel + " is not a concept path",
		}
	}
	full := filepath.Join(w.dir, filepath.FromSlash(clean))
	if err := atomicfile.WriteFile(full, content, 0o640); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	return nil
}

// held reports whether this Writer still holds the lock.
//
// Requires: op names the calling write, for the message.
// Ensures: nil while the lock is held; EINTERNAL naming op otherwise. Every write
// method calls it first.
//
// This is the one hole the type cannot close by construction. A caller may keep a
// Writer past its `defer Release()` and call a method on it, and no signature can
// prevent that; what a signature *can* do is guarantee the value was obtained
// legitimately, which is the failure that actually happened. EINTERNAL rather than
// ECONFLICT because this is not contention — it is this process's own bug, and
// WriterBusy must not report it as something that resolves on its own.
func (w *Writer) held(op string) error {
	if w != nil && w.lock != nil {
		return nil
	}
	return &errs.Error{
		Code:    errs.EINTERNAL,
		Message: op + ": the bundle's writer lock is not held; the Writer was released",
	}
}

// writerBusyOutcome is the envelope for contention, so every write path reports
// it identically.
func writerBusyOutcome(bundleDir string) gnosis.Outcome {
	return gnosis.Blocked(gnosis.ReasonWriterBusy,
		"another writer holds "+bundleDir+"; retry when it finishes", nil)
}
