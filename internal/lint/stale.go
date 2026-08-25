package lint

import (
	"strconv"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// staleCheck reports documents whose sources are past their declared date or have
// never been verified.
//
// §12's table declares this check as *"archived text ≠ upstream, or today ≥
// stale_after"*. **Only the second disjunct is implemented, and the first cannot
// be**: comparing archived text to upstream requires a fetch, and `lint` has no
// network — §4.6 keeps readers independent of the writer, and the same argument
// keeps them independent of the network. A corpus should be checkable on a train.
//
// That is not a half-check, because §14.3's vocabulary has a state for exactly
// this. A source whose upstream nobody has compared is **`unknown`**, which is a
// real answer rather than a gap, and reporting it as `fresh` is the collapse the
// four states exist to prevent. What the check cannot do it names.
//
// The two windows are different questions and this is where they separate:
//
//   - `stale_after` is a property of the **claim** — its author said to revisit it
//     by this date. It is an absolute date on OKF's determinism argument.
//   - `staleness_days` is a property of the **check** — this user last verified the
//     source more than N days ago. Relative, and safely so: it describes an
//     observation that is already per-user and already timestamped, not a claim.
//
// Applying the relative window to the claim would reintroduce exactly the
// read-time dependence §14.3 chose an absolute date to remove.
func staleCheck(now time.Time) Check {
	return Check{
		Name:       "stale",
		Categories: []string{"stale"},
		Actions:    []finding.Action{finding.ActionGuided},
		Applies: func(snap *Snapshot) (bool, string) {
			// Derived applicability, per §12. Neither half of this check can fire
			// on a corpus that declares no expiry and has verified nothing, and
			// saying so is different from reporting nothing found.
			for i := range snap.Documents {
				if !snap.Documents[i].StaleAfter.IsZero() {
					return true, ""
				}
			}
			if len(snap.SourceChecks) > 0 {
				return true, ""
			}
			return false, "no document declares stale_after and no source has been verified"
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			return StaleFindings(snap, now)
		},
	}
}

// StaleFindings is the stale check's findings, callable without the registry.
//
// Requires: snap is populated; now is the moment to judge against.
// Ensures: the same diagnostics the `stale` check produces, in document order. Pure.
//
// It is exported for one caller and the caller is the reason it exists rather than a
// convenience. §6.2 requires a loosening to be recorded with the finding count before
// and after, and `staleness_days` is a threshold this check reads — so
// `bundle.staleFindingDelta` runs it twice over one corpus with the two windows. The
// alternative was for that delta to reimplement the comparison, which would put a
// second answer to "is this stale" one package away from the first.
//
// The registry calls it too, so there is one implementation rather than a check and a
// copy of the check. **Applicability is deliberately not included**: a caller asking
// for the count under two windows wants the same population both times, and skipping
// would make the delta a comparison of two different questions.
func StaleFindings(snap *Snapshot, now time.Time) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		if d := staleness(now, &snap.Documents[i], snap); d != nil {
			out = append(out, *d)
		}
	}
	return out
}

// staleness reports one document's freshness problem, or nil when it has none.
//
// A document past its declared date is reported whether or not its sources were
// checked: the author's date is a statement about the claim and does not become
// less true because nobody looked at the source.
func staleness(now time.Time, doc *Document, snap *Snapshot) *finding.Diagnostic {
	if !doc.StaleAfter.IsZero() && !now.Before(doc.StaleAfter) {
		return &finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "stale",
			Path:     doc.Path,
			Message: "past its declared stale_after of " +
				doc.StaleAfter.Format(time.DateOnly) + "; the author asked for it to " +
				"be revisited by then",
			Action: finding.ActionGuided,
		}
	}
	return unverified(now, doc, snap)
}

// unverified reports a document whose sources were checked too long ago.
//
// **Never-checked is deliberately not a finding**, and working out why changed this
// check's shape. The first draft reported it at an informational severity, which
// `finding` does not have — it has error and warning, and "only SeverityError is
// blocking". That absence turned out to be the right constraint rather than an
// obstacle: a finding is something to act on, and "these sources have never been
// verified" is true of every document in a corpus that has just started fetching.
// It would be noise on day one for exactly the reason §12's derived applicability
// exists, and a warning that is true of everything teaches a reader to skip the
// category.
//
// Never-checked is a **state**, not a problem, and §14.3 already has a name for it.
// `FreshnessOf` reports it as `unknown` and `show` renders it, which puts the fact
// in front of the person reading the claim rather than in a list they scroll past.
func unverified(now time.Time, doc *Document, snap *Snapshot) *finding.Diagnostic {
	if len(doc.SourceKeys) == 0 || snap.StalenessDays <= 0 {
		return nil
	}
	// **An episodic type is exempt from the window, and only from this half of the
	// check** (§5.8.3.1). Its claims assert what happened at a moment, and its
	// evidence is a commit hash — immutable by construction — so "its sources were
	// last verified 40 days ago; re-run `gnosis fetch` on them" is advice nobody can
	// act on and a finding that can never clear. A finding that cannot be cleared is
	// worse than none: it teaches a reader that this category is permanent
	// background.
	//
	// The declared-date half above still applies. That date is the author's own
	// statement about their claim, and an author may legitimately ask for an episode
	// to be revisited — the exemption is about evidence that cannot change, not about
	// silencing a person who asked a question.
	if declared, ok := snap.Vocabulary.TypeNamed(doc.Type); ok && declared.Episodic {
		return nil
	}
	oldest, everChecked := oldestCheck(doc.SourceKeys, snap.SourceChecks)
	if !everChecked {
		return nil
	}
	age := int(now.Sub(oldest).Hours() / 24)
	if age < snap.StalenessDays {
		return nil
	}
	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "stale",
		Path:     doc.Path,
		Message: "its sources were last verified " + strconv.Itoa(age) +
			" days ago, past the " + strconv.Itoa(snap.StalenessDays) +
			"-day window; re-run `gnosis fetch` on them",
		Action: finding.ActionGuided,
	}
}

// oldestCheck returns the least recent check across a document's sources.
//
// The oldest rather than the newest: a document resting on four sources is only as
// verified as its least-verified one, and taking the newest would let one recent
// re-fetch vouch for three nobody has looked at.
func oldestCheck(keys []string, checks map[string]time.Time) (time.Time, bool) {
	var oldest time.Time
	for _, k := range keys {
		at, ok := checks[k]
		if !ok {
			return time.Time{}, false
		}
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
	}
	return oldest, !oldest.IsZero()
}

// FreshnessOf reports a document's state in §14.3's vocabulary.
//
// Requires: now is the moment to judge against; doc and snap are as the checks see
// them.
// Ensures: the same four states the domain type defines, computed from the same
// inputs the check uses, so a rendered state and a reported finding never disagree.
func FreshnessOf(now time.Time, doc *Document, snap *Snapshot) gnosis.Freshness {
	// hasUpstream and everChecked are different questions and the first draft
	// passed the second for the first, which made a document nobody had checked
	// report as `not_applicable` — "there is nothing to check" rather than "nobody
	// looked". That is the collapse the four states exist to prevent, inside the
	// function written to prevent it, and its own test caught it.
	//
	// A document has an upstream when it cites an archived source. Whether anyone
	// verified that source is what `checkedAt` carries, and the zero time is how
	// gnosis.FreshnessOf is told nobody has.
	oldest, _ := oldestCheck(doc.SourceKeys, snap.SourceChecks)
	return gnosis.FreshnessOf(now, oldest, doc.StaleAfter, len(doc.SourceKeys) > 0)
}
