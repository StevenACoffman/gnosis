package bundle

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
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

// WriterLock is exclusive ownership of a bundle for writing.
//
// The requirement it implements is easy to state too narrowly, so §4.6 states it
// in full: **the writer owns the bundle, not merely the database.** Serialising
// SQLite writes and leaving markdown writes unserialised would coordinate the
// cache and not the corpus — two agents promoting a claim concurrently is a bundle
// problem, and SQLite's locking has nothing to say about it. So the lock is taken
// over the bundle directory and every write path takes it, including the ones that
// touch no database.
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
type WriterLock struct {
	lock *flock.Flock
}

// AcquireWriterLock takes the bundle's writer lock, waiting until it is free or
// ctx is done.
//
// Requires: bundleDir exists and is writable.
// Ensures: on success the caller holds the bundle exclusively and MUST call
// Release. On a context that expires while waiting, returns an error carrying
// ECONFLICT and gnosis.ReasonWriterBusy's sense — another writer holds it, which
// is a state that resolves on its own, not a fault in the caller's request.
//
// The state directory is created here rather than assumed, because it is
// gitignored and so no fresh clone has one.
func AcquireWriterLock(ctx context.Context, bundleDir string) (*WriterLock, error) {
	const op = "bundle.AcquireWriterLock"

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
	return &WriterLock{lock: lock}, nil
}

// Release gives up the lock.
//
// Requires: nothing; releasing a nil or already-released lock is a no-op, so a
// deferred Release needs no guard.
// Ensures: the lock is available to the next writer. The lock *file* is left in
// place deliberately — removing it races with another process that has just
// opened it, and an empty file under a gitignored directory costs nothing.
func (w *WriterLock) Release() {
	if w == nil || w.lock == nil {
		return
	}
	_ = w.lock.Unlock()
	w.lock = nil
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

// writerBusyOutcome is the envelope for contention, so every write path reports
// it identically.
func writerBusyOutcome(bundleDir string) gnosis.Outcome {
	return gnosis.Blocked(gnosis.ReasonWriterBusy,
		"another writer holds "+bundleDir+"; retry when it finishes", nil)
}
