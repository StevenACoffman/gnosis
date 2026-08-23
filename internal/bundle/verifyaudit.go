package bundle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/skillet/errs"
)

// tailWindow is how far back verifyAudit reads to find the last line.
//
// A row is one JSON object with bounded fields — an actor, a few paths, two
// hashes — so 64 KiB is far more than one needs. It is deliberately generous
// rather than tight: the failure mode of a window too small is reporting a
// mismatch that is not there, and accusing the trail of being wrong when the
// reader was too short is the more expensive error.
const tailWindow = 64 << 10

// lostRowError marks a row that was appended and cannot be read back.
//
// A type rather than a sentinel value so there is no package-level variable, and
// so the underlying corruption error keeps its own message and code — a caller
// asking errs.ErrorCode still gets EINVALID.
type lostRowError struct{ err error }

func (l *lostRowError) Error() string { return l.err.Error() }

func (l *lostRowError) Unwrap() error { return l.err }

// AuditVerified appends a row and confirms it landed.
//
// Requires: bundleDir is writable; the writer lock is held if one is in use.
// Ensures: nil when the row is on disk. Otherwise an error, and **the two failures
// are distinguishable by AuditLost**, because they call for opposite handling: an
// append that reported failure is a known gap the caller may warn about and carry
// on from, and a row the append claimed to write and that is not there is the one
// failure no other signal reveals.
//
// This exists because `init` and `index rebuild` write rows without going through
// the coordinator — they predate it — so verifying only inside Execute would make
// "a mutation verifies its own row" true of two mutations out of four. A claim that
// holds for half its subjects is the kind of half-truth §15 is about.
func AuditVerified(bundleDir string, row *audit.Row) error {
	if err := appendRow(bundleDir, row); err != nil {
		return err
	}
	if err := verifyAudit(bundleDir, row); err != nil {
		return &lostRowError{err: err}
	}
	return nil
}

// AuditLost reports whether err is a row that was appended and cannot be read
// back.
//
// Requires: nothing; a nil error is not a lost row.
// Ensures: true only for the verification failure, never for an append failure.
// A caller distinguishing them is distinguishing "we know the record failed" from
// "the trail told us it succeeded and it did not", and only the second is
// undetectable by any other means.
func AuditLost(err error) bool {
	var l *lostRowError
	return errors.As(err, &l)
}

// verifyAudit confirms that a row just appended is actually on disk.
//
// Requires: want was just written by Audit and the writer lock is still held —
// without the lock another writer could append between the two, and the
// comparison would fail for a trail that is perfectly correct.
// Ensures: nil when the trail's last line is exactly want's canonical bytes. An
// error naming what was found otherwise, and an error when the tail cannot be read
// at all. Reads a bounded window from the end, so the cost does not grow with the
// trail and old damage elsewhere in the file cannot fail a new write.
//
// **This is the only place a silently-lost row can be detected.** §15's argument
// is from an observed failure rather than a hypothetical one: a surveyed project's
// ledger-append step returned success for five consecutive nights while writing
// nothing, and every other stage of the same routine succeeded, so no other signal
// existed. An append that reports success is not evidence that a record exists;
// reading it back is.
//
// The comparison is on canonical bytes rather than on parsed fields. `audit.Row`
// has no equality and gaining one would drift from what `Audit` writes; comparing
// what the same serialiser produces means there is one definition of a row's
// identity rather than two.
func verifyAudit(bundleDir string, want *audit.Row) error {
	const op = "bundle.verifyAudit"

	line, err := want.Canonical()
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}

	tail, err := readTail(op, filepath.Join(bundleDir, stateDir, auditFile), tailWindow)
	if err != nil {
		return err
	}
	if bytes.HasSuffix(tail, line) {
		return nil
	}

	// Distinguish "nothing is there" from "something else is there". The first is
	// the observed failure; the second means a writer appended without the lock,
	// which is a different fault with a different fix.
	if len(tail) == 0 {
		return corrupt(op, auditFile, "the row was appended and the trail is empty")
	}
	return corrupt(op, auditFile,
		"the row was appended and is not the trail's last line")
}

// readTail reads up to n bytes from the end of a file.
//
// Requires: n is positive.
// Ensures: the final min(n, size) bytes, or an empty slice for a file that does
// not exist — which for the trail means the append wrote nothing at all, and is a
// finding rather than an error.
func readTail(op, path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	size := info.Size()
	if size < n {
		n = size
	}
	buf := make([]byte, n)
	if _, err = f.ReadAt(buf, size-n); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return buf, nil
}
