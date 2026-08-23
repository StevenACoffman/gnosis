package bundle

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/skillet/errs"
)

// auditFile is the write trail, inside stateDir and therefore gitignored.
const auditFile = "audit.jsonl"

// appendRow appends one row to the bundle's write trail.
//
// Requires: bundleDir exists and is writable; row is populated.
// Ensures: the row is appended as one line. Appending rather than rewriting means
// a concurrent reader never sees a partial file, and the writer lock means two
// writers never interleave a line.
//
// **A failure here does not fail the write it describes**, and callers are
// expected to treat it that way. If a document landed and this returns an error,
// reporting that error would tell a caller their write failed when it succeeded —
// which is the more dangerous of the two wrong answers.
//
// It is unexported, and that is the point rather than an accident of layering.
// §15 requires every mutation to verify its own row, and the first guard on that
// was a test reading the call sites' source text for `bundle.Audit(` — brittle,
// and written and deleted inside one pass. Taking the unverified append off the
// package's surface makes the compiler enforce what that test was inspecting:
// AuditVerified is the only way in, so a writer added later cannot append without
// checking. Make it impossible rather than tested.
//
// See AuditVerified, and Coordinator.auditUnread for why the two failures differ.
func appendRow(bundleDir string, row *audit.Row) error {
	const op = "bundle.appendRow"

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
// Ensures: every row that parsed, in the order written, **and** the line numbers
// that did not. The returned error is for reading the file — a missing directory, a
// failing disk — and never for the file's contents, so a caller may always inspect
// the Trail it got back.
//
// That split is the point and it took two attempts. The first version returned rows
// and dropped what it could not parse, which makes a truncated trail read as a
// short one: the direction that flatters. The second reported the first malformed
// line as an error and returned **no rows at all**, so one bad byte on line 3 made
// the other 3,999 unreadable — worse than either option §15 discusses, and my own
// doing. This version is §15's: the rows and the damage, both, with Trail.Whole for
// the caller who needs "all of it or an error".
//
// A blank line is not corruption. An append interrupted between the row and its
// newline leaves one, and so does a hand-edit; neither is a lost record, and
// counting them would make the reported number mean two different things.
func AuditTrail(bundleDir string) (Trail, error) {
	const op = "bundle.AuditTrail"

	f, err := os.Open(filepath.Join(bundleDir, stateDir, auditFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Trail{Rows: []audit.Row{}}, nil
		}
		return Trail{}, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	out := Trail{Rows: []audit.Row{}}
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var row audit.Row
		if uErr := json.Unmarshal(scanner.Bytes(), &row); uErr != nil {
			out.Malformed = append(out.Malformed, line)
			continue
		}
		out.Rows = append(out.Rows, row)
	}
	if sErr := scanner.Err(); sErr != nil {
		// The scan stopped part way, so Malformed undercounts and the rows are a
		// prefix rather than the trail. Reporting a Trail here would be reporting a
		// count this function knows is wrong.
		return Trail{}, &errs.Error{Op: op, Err: sErr}
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
// **Best-effort applies only to the append.** A row the append claimed to write
// and that is not on disk is a different event, handled the opposite way: §15
// requires it to reach the caller as an error, because it is the one failure no
// other signal reveals. See Coordinator.auditUnread.
func (c *Coordinator) record(row *audit.Row) {
	err := AuditVerified(c.Dir, row)
	switch {
	case err == nil:
	case AuditLost(err):
		// §15: an append reporting success is not evidence that a record exists,
		// and this is the only place the difference is visible.
		c.auditUnread = err
	default:
		c.auditErr = err
	}
}
