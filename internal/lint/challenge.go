package lint

import (
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// unansweredChallengeCheck reports a reader-filed challenge older than the declared
// window (§10.7.3).
//
// It is the complement of the rule that a challenge does not block. Only a `replay`
// challenge gnosis has itself verified becomes error-severity — at that point it is an
// evidence failure rather than an assertion — and anything else blocking on assertion
// alone would make the front door a denial of service. But the complement has to be
// that a challenge cannot be **ignored quietly**, or the class collapses into a
// suggestion box: silence is visible, and it ages.
//
// **Warning, never a gate**, for that reason exactly. §17.0's point about corrective
// permission is the same one: the corrective option existing is not the same as anybody
// being obliged to take it.
//
// **Age is measured from the challenge's own `at`**, and a challenge whose timestamp
// does not parse is reported rather than skipped. The alternative fails in the
// direction that loses the objection: a malformed date would make a challenge
// permanently invisible, and the corpus would look as though nobody had ever contested
// anything.
func unansweredChallengeCheck(now time.Time) Check {
	return Check{
		Name:       "unanswered-challenge",
		Categories: []string{"unanswered-challenge"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someChallengeIsOpen,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			return AgedChallenges(snap, now)
		},
	}
}

// someChallengeIsOpen reports whether the corpus holds a challenge to age.
//
// Requires: nothing.
// Ensures: a reason whenever it declines, naming which of the two things is missing —
// the window or the challenges. Pure.
func someChallengeIsOpen(snap *Snapshot) (bool, string) {
	if snap.ChallengeDays <= 0 {
		return false, "no unanswered_days is in force: standards/challenge.toml" +
			" either declares none or did not load, and `gnosis doctor` says which"
	}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Challenges {
			if doc.Challenges[j].Open() {
				return true, ""
			}
		}
	}
	return false, "no document carries an open challenge, so there is nothing waiting" +
		" for an answer"
}

// AgedChallenges reports each open challenge older than the window.
//
// Requires: snap.ChallengeDays is positive; now is the moment to judge against.
// Ensures: one diagnostic per aged challenge, in document order and then in the order
// the document declares them — which is the order they were filed, so the oldest
// reads first. Pure.
//
// Exported for the reason StaleFindings is: §6.2 requires a loosening to be recorded
// with the finding count before and after, and `unanswered_days` is a threshold this
// check reads, so `bundle.challengeFindingDelta` runs it twice over one corpus with
// the two windows.
func AgedChallenges(snap *Snapshot, now time.Time) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	window := time.Duration(snap.ChallengeDays) * 24 * time.Hour
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Challenges {
			if d := agedChallenge(doc, &doc.Challenges[j], now, window); d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// agedChallenge is one challenge's finding, or nil when it is closed or still inside
// the window.
func agedChallenge(
	doc *Document, c *gnosis.Challenge, now time.Time, window time.Duration,
) *finding.Diagnostic {
	if !c.Open() {
		return nil
	}
	filed, err := time.Parse(time.RFC3339, c.At)
	switch {
	case err != nil:
		// Reported rather than skipped: a challenge nobody can date is one nobody
		// can age out, and silence here would hide the objection permanently.
		return challengeFinding(doc, c,
			"its `at` is not an RFC 3339 timestamp, so nothing can say how long it"+
				" has waited")
	case now.Sub(filed) < window:
		return nil
	}
	return challengeFinding(doc, c, "it has been open since "+
		filed.Format(time.DateOnly)+", longer than standards/challenge.toml's window")
}

// challengeFinding renders one aged challenge.
//
// The rationale is quoted back, because the finding's whole job is to put the reader's
// objection in front of somebody who can answer it — a line saying only that a
// challenge is old asks a person to go and look up what it said.
func challengeFinding(doc *Document, c *gnosis.Challenge, why string) *finding.Diagnostic {
	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "unanswered-challenge",
		Path:     doc.Path,
		Message: string(c.Class) + " challenge by " + c.By + ": " + why +
			". It says: " + excerpt(c.Rationale) +
			". Answer it with `gnosis adjudicate`, or record why the claim stands —" +
			" a rejected challenge closes with a warrant and is never deleted",
		Action: finding.ActionHuman,
	}
}
