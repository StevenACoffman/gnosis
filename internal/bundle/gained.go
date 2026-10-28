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

	// Challenged is how many claims a reader contested (§10.7).
	//
	// **A gain for the reason Declined is one, and a stronger one.** A challenge is
	// the corpus learning that somebody thinks it is wrong, which it could not learn
	// any other way: §6.2.1 records that the candidate selector is systematically
	// blind to conflicts between claims sharing no source, no link and no
	// vocabulary, and a reader who noticed one has done for free what the selector
	// provably cannot. Counting it also keeps the incentive pointing the right way,
	// since §10.7.3 requires that being wrong cost the challenger nothing.
	Challenged int `json:"challenged"`

	// Adjudicated is how many claims a person decided and wrote a warrant for
	// (§10.4).
	//
	// The clearest gain in this list: an adjudicated claim is knowledge present in
	// neither source, which is the largest category a group of experienced
	// practitioners produces and the one the evidence invariant would otherwise
	// refuse outright.
	Adjudicated int `json:"adjudicated"`

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
	return g.Promoted+g.Admitted+g.Archived+g.Declined+
		g.Challenged+g.Adjudicated > 0
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
		if row.At.Before(since) || row.Outcome != string(gnosis.StatusOK) {
			continue
		}
		out.count(row)
	}
	return out
}

// count adds one succeeded row to the totals.
//
// Split from Gained when the seventh operation arrived and the complexity limit
// objected, which was the right moment: the loop is about *which rows count* — since
// the window, and succeeded — and this is about *what each one gained*. They change for
// different reasons, and the switch grows by one case every time the corpus learns a
// new verb.
func (g *Gains) count(row *audit.Row) {
	switch row.Op {
	case audit.OpPromote:
		g.Promoted++
	case audit.OpAdmit:
		g.Admitted++
	case audit.OpDiscard:
		g.Declined++
	case audit.OpChallenge:
		g.Challenged++
	case audit.OpAdjudicate:
		g.Adjudicated++
	case audit.OpFetch:
		// One row per invocation, and `Paths` names what reached the disk — so the
		// sources archived is the path count rather than the row count. A fetch of
		// thirty sources is one row and thirty gains.
		g.Archived += len(row.Paths)
	case audit.OpDefer:
		// Not a gain, and the reason is sharper than for a supersession: a deferral
		// records that the corpus knows something is wrong and is not fixing it yet.
		// Counting it would make a number that rises as unresolved contradictions
		// accumulate, which is §17's refusal to score at its most inverted — the
		// figure would look like progress and mean the opposite. What reads the
		// deferrals is `lint`, which still reports every one of them.
	case audit.OpSupersede:
		// Not a gain and not nothing: a supersession records that the corpus changed
		// its mind, and the knowledge it gained is the *winning* claim, which was
		// counted when it was promoted. Counting it here would count one addition
		// twice, and §17's refusal to score is the same argument — a number that grows
		// when a corpus revises itself is a number somebody will try to grow.
	case audit.OpInit, audit.OpRebuild, audit.OpUnset:
		// Scaffolding and cache rebuilds are not gains: the corpus knows nothing it
		// did not know before. Named rather than defaulted so an operation added
		// later has to be classified deliberately.
	}
}
