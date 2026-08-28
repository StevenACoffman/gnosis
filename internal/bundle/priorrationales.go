package bundle

import (
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// priorRationales are the reasons already recorded for one document, oldest first.
//
// Requires: trail came from AuditTrail; path is bundle-relative.
// Ensures: one entry per promotion row for that path that carried a rationale,
// labelled so a diagnostic can name it. Pure — the reading is the caller's.
//
// **Only successful promotions count, and the first draft of this had it backwards.**
// Counting refusals looked like the safer choice — a rationale somebody wrote is a
// rationale somebody wrote — and it made `promote` refuse its own second half: the
// confirmation flow previews first, records the blocked outcome with the rationale on
// it, and then applies, so the apply found the preview's row and called it a repeat.
// Two CLI tests caught it.
//
// The correct reading is also the faithful one. §10.6.4 refuses a rationale identical
// to one "already recorded for the same subject" — a *warrant*, which is a decision
// that landed. A withheld promotion adjudicated nothing, so there is nothing for a
// second attempt to be a copy of. `Owed` scopes itself to successful promotions for a
// neighbouring reason and says so.
//
// Nor does this open an evasion. Reusing boilerplate still requires it to have been
// accepted once, and every use after that is refused — which is what the check is
// for. Being refused on purpose first buys nothing.
//
// A row written before `audit.Row.Rationale` existed carries its reason inside
// Detail and is skipped rather than parsed out. Parsing gnosis's own prose to
// recover a value gnosis now stores properly would be a second reader of a format
// that is about to have no writers, and the cost of skipping is one missed
// duplicate on a per-user trail nobody has migrated.
func priorRationales(trail *Trail, path string) []gnosis.PriorRationale {
	var out []gnosis.PriorRationale
	for i := range trail.Rows {
		row := &trail.Rows[i]
		if row.Op != audit.OpPromote || row.Rationale == "" {
			continue
		}
		if row.Outcome != string(gnosis.StatusOK) {
			continue
		}
		if primaryPath(row) != path {
			continue
		}
		out = append(out, gnosis.PriorRationale{
			Label: label(row),
			Text:  row.Rationale,
		})
	}
	return out
}

// label names a recorded decision the way a person would refer to it: who, and when.
//
// The date alone, not the full timestamp. A refusal that said
// "2026-08-20T14:02:11.418293Z" would be precise about the thing the reader does not
// need and hard to match against the trail they are about to open.
func label(row *audit.Row) string {
	who := row.Actor
	if who == "" {
		who = "an unrecorded actor"
	}
	return who + " on " + row.At.Format(time.DateOnly)
}

// reusedRationale reports why this promotion's rationale may not be accepted, or "".
//
// Requires: w holds the lock, so the trail cannot grow underneath the read; path and
// rationale come from the command.
// Ensures: "" when the rationale is acceptable, when there is none, and when the
// trail could not be read.
//
// **A trail that will not load must not block a promotion**, and that is a judgement
// rather than an oversight. This check is a filter on the quality of an explanation;
// the trail's health is `doctor`'s subject and is reported there. Refusing to promote
// because a log file is damaged would make a reporting problem into a writing
// problem, which is the coupling §15 warns about — and the failure would be silent
// about its real cause, since the message a caller got would be about their
// rationale.
//
// The damaged trail is not thereby ignored: a malformed line is `Trail.Malformed`,
// `doctor` reports it, and `Debt` already refuses to call its count complete while
// one exists.
func reusedRationale(bundleDir, path, rationale string, asked []string) string {
	if rationale == "" {
		return ""
	}
	trail, err := AuditTrail(bundleDir)
	if err != nil {
		return ""
	}
	return gnosis.UnusableRationale(rationale, asked, priorRationales(&trail, path))
}
