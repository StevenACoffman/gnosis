package bundle

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/skillet/errs"
)

// auditFile is the write trail, inside stateDir and therefore gitignored.
const auditFile = "audit.jsonl"

// Audit appends one row to the bundle's write trail.
//
// Requires: bundleDir exists and is writable; row is populated.
// Ensures: the row is appended as one line. Appending rather than rewriting means
// a concurrent reader never sees a partial file, and the writer lock means two
// writers never interleave a line.
//
// **A failure here does not fail the write it describes**, and callers are
// expected to treat it that way. If a document landed and this returns an error,
// reporting that error would tell a caller their write failed when it succeeded —
// which is the more dangerous of the two wrong answers. See AuditOrReport.
func Audit(bundleDir string, row *audit.Row) error {
	const op = "bundle.Audit"

	line, err := row.Canonical()
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}

	dir := filepath.Join(bundleDir, stateDir)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		return &errs.Error{Op: op, Err: mkErr}
	}
	// 0o600 rather than 0o640: the trail names who did what, and it is per-user
	// state under a gitignored directory, so there is no group that should be
	// reading another user's record of their own writes.
	f, err := os.OpenFile(filepath.Join(dir, auditFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	if _, err = f.Write(line); err != nil {
		return errors.Join(&errs.Error{Op: op, Err: err}, f.Close())
	}
	if cErr := f.Close(); cErr != nil {
		// A close error on an append means the write may not have reached the
		// disk, which for a trail is the same as not having happened.
		return &errs.Error{Op: op, Err: cErr}
	}
	return nil
}

// AuditTrail reads the bundle's write trail, oldest first.
//
// Requires: nothing; a bundle with no trail is not an error.
// Ensures: rows in the order they were written. A malformed line is an error
// rather than a skip: a trail that quietly drops what it cannot read is a trail
// that cannot be counted, and counting is most of what one is for.
func AuditTrail(bundleDir string) ([]audit.Row, error) {
	const op = "bundle.AuditTrail"

	f, err := os.Open(filepath.Join(bundleDir, stateDir, auditFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []audit.Row{}, nil
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	out := []audit.Row{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var row audit.Row
		if uErr := json.Unmarshal(scanner.Bytes(), &row); uErr != nil {
			return nil, &errs.Error{Code: errs.EINVALID, Op: op, Err: uErr}
		}
		out = append(out, row)
	}
	if sErr := scanner.Err(); sErr != nil {
		return nil, &errs.Error{Op: op, Err: sErr}
	}
	return out, nil
}

// now reports the coordinator's clock.
//
// The clock is a field rather than a call to time.Now so a test can assert an
// exact timestamp instead of "recently". That is a genuine dependency and not a
// test-only seam: an audit row's whole value is the time on it, and a value the
// tests cannot pin is a value the tests do not check.
func (c *Coordinator) now() time.Time {
	if c.Now == nil {
		return time.Now().UTC()
	}
	return c.Now().UTC()
}

// record appends a row for one command's outcome, and never fails the caller.
//
// The write already happened. Reporting an audit failure as the operation's
// failure would tell a caller to retry something that succeeded, so the error is
// folded into the outcome's detail instead — visible, and not mistaken for the
// write's own result.
//
// That this is best-effort is a real weakness rather than a tidy design, and it is
// recorded in TODO as such: a corpus whose trail silently has gaps cannot answer
// the question the trail exists for.
func (c *Coordinator) record(row *audit.Row) {
	if err := Audit(c.Dir, row); err != nil {
		c.auditErr = err
	}
}
