package bundle

import (
	"sort"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
)

// Withheld is one claim a source was found not to support.
type Withheld struct {
	// Source is the archived text the quotations were checked against.
	Source string `json:"source"`

	// Claim is the assertion that was not supported, as the reply worded it.
	Claim string `json:"claim"`

	// Submitter is who offered it, and At is when. Both matter for the same reason:
	// the same claim offered twice by two agents is two events, and a reader asking
	// "has anyone tried to assert this before" wants to see both.
	Submitter string    `json:"submitter"`
	At        time.Time `json:"at"`
}

// Unauthorized is everything the corpus was asked to accept and did not.
type Unauthorized struct {
	// Withheld is every refused claim, sorted by source then claim so two runs over
	// one trail are comparable.
	Withheld []Withheld `json:"withheld"`

	// Sources is how many distinct archived files appear, which is the shape before
	// the list — one source refusing eleven claims is a different situation from
	// eleven sources refusing one each.
	Sources int `json:"sources"`

	// Unreadable is how many trail lines would not parse, making the list a floor.
	Unreadable int `json:"unreadable_lines,omitempty"`
}

// Complete reports whether the list is a total rather than a floor.
func (u *Unauthorized) Complete() bool { return u.Unreadable == 0 }

// NotAuthorized is every claim a source was found not to support.
//
// Requires: trail came from AuditTrail.
// Ensures: one entry per (source, claim) the trail recorded as unsupported, sorted; a
// corpus where every reply held yields none rather than an error. Pure.
//
// # Why the corpus needs this and did not have it
//
// gnosis records what a source supports and kept no trace of what it was found not to
// support. A reply asserting something the archived text does not contain was refused,
// reported once, and forgotten — so the same assertion could be offered again, by the
// same model, and nothing would say it had been tried. That is the asymmetry the
// rejected-alias record already fixed one level down (§5.8.2.1's `rejected` list keeps
// the reasoning, not only the conclusion), applied to claims.
//
// **This is the observation half.** The trail is per-user, so what it holds is "on this
// machine, this reply was refused against this source". A *committed* record of what a
// source does not support belongs with §10.7.4's challenge states, which are Phase 3 —
// and saying so here is what stops a later reader taking this for a corpus assertion.
func NotAuthorized(trail *Trail) *Unauthorized {
	out := &Unauthorized{
		Withheld:   []Withheld{},
		Unreadable: len(trail.Malformed),
	}
	sources := map[string]bool{}
	for i := range trail.Rows {
		row := &trail.Rows[i]
		if row.Op != audit.OpAdmit || len(row.Unsupported) == 0 {
			continue
		}
		source := primaryPath(row)
		sources[source] = true
		for _, claim := range row.Unsupported {
			out.Withheld = append(out.Withheld, Withheld{
				Source: source, Claim: claim,
				Submitter: row.Actor, At: row.At,
			})
		}
	}
	out.Sources = len(sources)
	sort.Slice(out.Withheld, func(i, j int) bool {
		if out.Withheld[i].Source != out.Withheld[j].Source {
			return out.Withheld[i].Source < out.Withheld[j].Source
		}
		return out.Withheld[i].Claim < out.Withheld[j].Claim
	})
	return out
}
