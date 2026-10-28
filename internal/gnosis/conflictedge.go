package gnosis

import "strings"

// The states a recorded conflict can be in (§17.0's three, minus the one that is not
// recorded here).
//
// ConflictDeferred is the only state this family carries, and the constant exists so
// that a reader of frontmatter is not comparing against a bare string. See ConflictEdge
// for why the other two are absent.
const (
	// ConflictDeferred: a person saw this contradiction and is not acting yet.
	ConflictDeferred ConflictState = "deferred"
)

// ConflictState is where a recorded conflict has got to (§17.0).
type ConflictState string

// ConflictEdge is one contradiction a person has recorded a decision about (§5.4).
//
// # Why only deferrals live here
//
// §10.7.4's rule is **decisions are committed, observations are cached**, with the
// operative test being *does later work have to rely on this?* A conflict the predicates
// can compute is re-derived by every run, so committing it would be `checked.jsonl`'s
// mistake inside a reviewed file — churn in frontmatter for a value nobody decided.
//
// A **deferral** is the exception, and §10.7.4 names it: a deferred state "says *a person
// saw this and is not acting yet*, which no rebuild can re-derive". §17.0 spends a
// paragraph on why it is worth a state at all — "the common failure of a findings system
// is not that problems go undetected but that detected problems go unanswered, and
// silence is indistinguishable from nobody having looked".
//
// The other two states are absent on purpose. An **open** conflict is what the check
// reports, freshly, every run. A **closed** one is a warrant plus a supersession (§10.4),
// both of which exist — and a second record of one decision is two places to disagree.
//
// # It names identifiers, never paths
//
// §5.4 is explicit for this family and `gnosis_supersedes`: "an edge that survives
// reorganization is the point". So `Concept` is an ID and not a path, and this shipped as
// a path once elsewhere and was wrong within a day (§10.4).
type ConflictEdge struct {
	// Concept is the other document in the contradiction.
	Concept ID

	// Finding is the contradiction's stable identity, as the check computes it: a
	// content address over the two claim references. A minted identifier would make a
	// recorded deferral unmatchable the next time the check ran, which is the one
	// thing this record exists to prevent.
	Finding string

	// State is where the decision got to. Only `deferred` is recorded here.
	State ConflictState

	// By is who deferred it and At is when, as OKF §7's grammar and RFC 3339 write
	// them. Both required: §17.0 asks for "who saw it, when, and why they are not
	// acting yet", and a deferral missing any of the three is a conflict that went
	// quiet rather than one somebody decided about.
	By string
	At string

	// Reason is why they are not acting yet.
	Reason string
}

// Valid reports whether an edge records a decision somebody could review.
//
// Requires: nothing; e may have been read from any frontmatter.
// Ensures: false when any of the five load-bearing fields is missing or when the state is
// one this family does not carry. Pure.
//
// **All five, and the strictness is §10.6.4's bet applied here.** A permission bit asks
// whether somebody may decide; this asks them to write down why, in a commit, in front of
// colleagues — and an edge with no reason records that a conflict stopped being reported
// and not that anybody chose to live with it.
func (e *ConflictEdge) Valid() bool {
	return e.Concept != "" &&
		strings.TrimSpace(e.Finding) != "" &&
		e.State == ConflictDeferred &&
		strings.TrimSpace(e.By) != "" &&
		strings.TrimSpace(e.At) != "" &&
		strings.TrimSpace(e.Reason) != ""
}

// Defers reports whether this edge records a deferral of the given finding.
//
// A method rather than a comparison at each call site, because three readers ask it —
// §6.2's selector, §13's queue and the stale-edge check — and three spellings of one
// question is how two of them come to disagree about whether an invalid edge counts.
//
// **An invalid edge defers nothing**, which is the safe direction: a half-written entry
// leaves the conflict reported rather than silently suppressed.
func (e *ConflictEdge) Defers(finding string) bool {
	return e.Valid() && e.Finding == finding
}
