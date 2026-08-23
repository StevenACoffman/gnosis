package bundle

import (
	"strconv"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/skillet/errs"
)

// Trail is the parsed write trail together with what would not parse.
//
// Two fields rather than a bare slice, because §15 requires that a consumer
// **cannot accidentally treat a partial trail as whole**. A reader given
// `[]audit.Row` has no way to ask how many rows it did not get; a reader given
// this sees the question. That is the whole reason for the type.
//
// The zero value is an empty trail with nothing malformed, which is the honest
// reading of a bundle that has never been written to.
type Trail struct {
	// Rows are the rows that parsed, oldest first.
	Rows []audit.Row `json:"rows"`

	// Malformed are the 1-based line numbers that did not, ascending. A line here
	// is corruption per §15 — the file is append-only and written by one process
	// holding a lock, so a line that will not decode was truncated or edited.
	Malformed []int `json:"malformed,omitempty"`
}

// Whole reports whether the trail can be counted on, as an error.
//
// Requires: nothing; the zero Trail is whole.
// Ensures: nil when nothing was malformed, and EINVALID naming the lines
// otherwise. Pure.
//
// This is §15's distinction made callable: a malformed count is "`EINVALID`
// territory when a reader asks for the whole trail" and "a reported number when
// they ask for a range". Asking for the whole trail is calling this. A reader
// showing the last twenty rows does not, and gets the rows.
func (t *Trail) Whole() error {
	if len(t.Malformed) == 0 {
		return nil
	}
	lines := make([]string, 0, len(t.Malformed))
	for _, n := range t.Malformed {
		lines = append(lines, strconv.Itoa(n))
	}
	return &errs.Error{
		Code: errs.EINVALID,
		Message: "bundle.Trail.Whole: " + auditFile + " has " +
			strconv.Itoa(len(t.Malformed)) + " unreadable line(s) (" +
			strings.Join(lines, ", ") + "); the trail cannot be counted",
	}
}

// Newest is the timestamp of the last row, or the zero time for none.
//
// Requires: nothing.
// Ensures: the last row's At, which is the newest because the file is append-only
// and every writer holds the lock. Pure.
//
// The zero time means "no rows", and a caller comparing it against anything must
// treat that as unknown rather than as very old — reporting an empty trail as
// stale would flag every corpus nobody has written to yet.
func (t *Trail) Newest() time.Time {
	if len(t.Rows) == 0 {
		return time.Time{}
	}
	return t.Rows[len(t.Rows)-1].At
}
