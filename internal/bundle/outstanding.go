package bundle

import (
	"sort"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Abandoned is one decision somebody was asked for and never made.
//
// It is per path rather than per attempt. Somebody who tried three times and walked
// away three times has one outstanding decision, and reporting three would make the
// list longer where the corpus got no more uncertain.
type Abandoned struct {
	// Path is the quarantined document, relative to the bundle root.
	Path string `json:"path"`

	// Asked is when the tool last put the decision in front of somebody. The
	// *last* time, because that is when the clock a reader cares about starts:
	// "nobody has decided this in three weeks" is the fact, and the first attempt
	// would make an abandoned document look older every time somebody looked at it
	// again.
	Asked time.Time `json:"asked"`

	// Attempts is how many times it was asked for, which is the difference between
	// a document nobody has looked at and one three people have declined to sign.
	Attempts int `json:"attempts"`

	// Reason is the last thing the tool said was needed, so the list says what to
	// do rather than only what is undone.
	Reason string `json:"reason,omitempty"`
}

// Undecided is every required decision nobody made.
//
// §15 asks for this report and observes that the states are already recorded: a
// promotion that reached `needs_human` is in the trail, and the draft it was about is
// in quarantine. What was missing is the subtraction.
type Undecided struct {
	// Abandoned is the outstanding decisions, sorted by path.
	Abandoned []Abandoned `json:"abandoned"`

	// Drafts is how many documents are waiting in quarantine at all. It is the
	// denominator, for the reason Debt.Rows is one: three abandoned decisions mean
	// something different against four drafts than against four hundred.
	Drafts int `json:"drafts"`

	// Unreadable is how many trail lines would not parse. A count from a damaged
	// trail is a floor, and Complete is what lets a caller say so.
	Unreadable int `json:"unreadable_lines,omitempty"`
}

// Complete reports whether the count is a total rather than a floor.
//
// Requires: nothing; the zero Undecided is complete, describing a corpus nobody has
// been asked about.
// Ensures: false exactly when the trail had lines that would not parse. The caller
// must render the difference: an incomplete count invites the reader to conclude that
// fewer decisions are outstanding than there are, which is the direction that
// flatters.
func (u *Undecided) Complete() bool { return u.Unreadable == 0 }

// Outstanding is every decision the tool asked for and nobody made.
//
// Requires: trail came from AuditTrail; drafts came from Quarantined.
// Ensures: one entry per abandoned path, sorted; a corpus where every escalation was
// answered yields none rather than an error. Pure — both inputs are read by the
// caller (§4.6).
//
// # The definition is a subtraction, and each term removes a different mistake
//
// A path is outstanding when **the tool asked** — a promote row that ended blocked —
// and none of the following happened afterwards:
//
//   - **a successful promotion of that path.** The decision was made; that it took
//     two attempts is not an outstanding anything.
//   - **a discard.** The decision was also made, in the other direction: somebody
//     looked at the draft and dropped it, which §9.5 treats as an answer.
//   - **the draft leaving quarantine by any other route.** A path with no draft has
//     nothing to decide about, whatever the trail says — a manual delete leaves no row
//     and this is what keeps the report from citing a file that is gone.
//
// Drop any one term and the report fills with decisions already taken, which is the
// report nobody reads twice.
//
// # What it deliberately cannot see
//
// A person who was never asked. The trail records what the tool did, so a draft that
// nobody has ever run `promote` against has no row and is not abandoned — it is
// unexamined, which `quarantine` already lists. Folding the two together would report
// a fresh corpus as a pile of neglected decisions on its first day, which is §12's
// argument about a warning true of everything.
func Outstanding(trail *Trail, drafts []string) *Undecided {
	waiting := map[string]bool{}
	for _, d := range drafts {
		waiting[d] = true
	}

	out := &Undecided{
		Abandoned:  []Abandoned{},
		Drafts:     len(drafts),
		Unreadable: len(trail.Malformed),
	}

	asked := map[string]*Abandoned{}
	for i := range trail.Rows {
		row := &trail.Rows[i]
		path := primaryPath(row)
		if path == "" {
			continue
		}
		switch {
		case row.Op == audit.OpDiscard, decided(row):
			// Answered. Anything earlier is settled, and a later ask starts again.
			delete(asked, path)
		case row.Op == audit.OpPromote && row.Outcome == string(gnosis.StatusBlocked):
			record(asked, path, row)
		}
	}

	for path, a := range asked {
		if waiting[path] {
			out.Abandoned = append(out.Abandoned, *a)
		}
	}
	sort.Slice(out.Abandoned, func(i, j int) bool {
		return out.Abandoned[i].Path < out.Abandoned[j].Path
	})
	return out
}

// decided reports whether a row is a promotion that landed.
func decided(row *audit.Row) bool {
	return row.Op == audit.OpPromote && row.Outcome == string(gnosis.StatusOK)
}

// record adds or updates the ask for one path.
//
// The newest ask wins for Asked and Reason and the attempts accumulate, which is the
// combination a reader needs: when the clock started, and how many people have
// already put this down.
func record(asked map[string]*Abandoned, path string, row *audit.Row) {
	a, seen := asked[path]
	if !seen {
		asked[path] = &Abandoned{Path: path, Asked: row.At, Attempts: 1, Reason: row.Detail}
		return
	}
	a.Attempts++
	if !row.At.Before(a.Asked) {
		a.Asked = row.At
		a.Reason = row.Detail
	}
}
