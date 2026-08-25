package bundle

import (
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Gains is what the corpus acquired over a window.
//
// # Why this report exists at all
//
// Hamming's rating dynamic: "if everyone starts out at 95% there is little a person can
// do to raise their rating but much which will lower it; hence the obvious strategy is
// to play things safe." A corpus whose only visible signal is *problems found* rewards
// contributing less and claiming less — every other report here counts something wrong,
// and a reader who only ever sees those learns that the safe move is a smaller page.
//
// So this counts what was gained. It is the same trail the other three reports read,
// asked the opposite question.
//
// # A count, over a window, and never a rate
//
// §17 forbids presenting a count as health, and that constrains this report more than
// the others: a gains number is the one somebody would want to make go up. So there is
// no total-since-the-beginning — a number that only grows says nothing — and no rate,
// because a rate invites a target and a target invites the padding this is meant to
// stop rewarding.
type Gains struct {
	// Since is the start of the window, and it is reported back because a count with
	// no period is uninterpretable.
	Since time.Time `json:"since"`

	// Promoted is how many documents entered the corpus.
	Promoted int `json:"promoted"`

	// Admitted is how many replies became quarantined documents: work that reached
	// tier 1 whether or not it has been promoted yet.
	Admitted int `json:"admitted"`

	// Archived is how many sources entered tier 0.
	Archived int `json:"archived"`

	// Declined is how many drafts a person looked at and dropped.
	//
	// **A gain, and the one entry in this list somebody will argue about.** A draft
	// declined is a judgement the corpus now holds and did not before — §10.7.4 makes
	// it a decision, and this report exists precisely because counting only additions
	// would make deciding-against invisible. Somebody who read forty drafts and
	// dropped thirty-nine did more for the corpus than somebody who admitted all
	// forty.
	Declined int `json:"declined"`

	// Unreadable is how many trail lines would not parse, making every count a floor.
	Unreadable int `json:"unreadable_lines,omitempty"`
}

// Complete reports whether the counts are totals rather than floors.
func (g *Gains) Complete() bool { return g.Unreadable == 0 }

// Any reports whether the corpus gained anything in the window.
//
// A separate question from the counts, because "nothing yet" and "we did not look" must
// not render alike — the same reason `Freshness` keeps `unknown` apart from `stale`.
func (g *Gains) Any() bool {
	return g.Promoted+g.Admitted+g.Archived+g.Declined > 0
}

// Gained counts what the corpus acquired since a moment.
//
// Requires: trail came from AuditTrail; since is the start of the window, and the zero
// time means the whole trail.
// Ensures: one count per kind of gain, and a floor rather than a total when the trail
// has unreadable lines. Pure.
//
// **Only rows that succeeded count.** A refused promotion did not add a document, and a
// report that counted attempts would reward trying rather than landing — which is the
// dynamic this exists to correct, not to invert.
//
// A discard counts whoever made it, agent or person. That differs from `log.md`, which
// records only a person's decline (§10.7.4), and the difference is what each is for: the
// committed log holds decisions, and this counts work. An agent clearing forty bad
// replies did work.
func Gained(trail *Trail, since time.Time) *Gains {
	out := &Gains{Since: since, Unreadable: len(trail.Malformed)}
	for i := range trail.Rows {
		row := &trail.Rows[i]
		if row.At.Before(since) {
			continue
		}
		if row.Outcome != string(gnosis.StatusOK) {
			continue
		}
		switch row.Op {
		case audit.OpPromote:
			out.Promoted++
		case audit.OpAdmit:
			out.Admitted++
		case audit.OpDiscard:
			out.Declined++
		case audit.OpFetch:
			// One row per invocation, and `Paths` names what reached the disk — so
			// the sources archived is the path count rather than the row count. A
			// fetch of thirty sources is one row and thirty gains.
			out.Archived += len(row.Paths)
		case audit.OpInit, audit.OpRebuild, audit.OpUnset:
			// Scaffolding and cache rebuilds are not gains: the corpus knows nothing
			// it did not know before. Named rather than defaulted so an operation
			// added later has to be classified deliberately.
		}
	}
	return out
}
