package gnosis

import "strings"

// The reasons a deterministic path gave way to a model (§6.4).
//
// MissReasonUnset is the zero value and names nothing, so a row nobody characterised is
// refused rather than counted as the commoner case — the discipline `audit.OpUnset`
// follows, and for the sharper reason here: this log's whole output is a count grouped
// by reason, and an unpopulated value would swell whichever group it defaulted to.
const (
	MissReasonUnset MissReason = ""

	// MissNoPath: no deterministic path exists for this operation, so asking a model
	// is the design rather than a fallback.
	//
	// Extraction is the case. §19 records why a deterministic phase cannot honestly
	// say where one claim ends and the next begins, and §11 demonstrates that naive
	// splitting is not an alternative — so a row of this kind recurs for as long as
	// the corpus ingests anything, and it is **not** a check waiting to be written.
	MissNoPath MissReason = "no_deterministic_path"

	// MissNoPredicate: a deterministic path exists, ran, and decided nothing.
	//
	// This is §6.4's backlog signal. The critic is the standing case — no check
	// decides whether a quotation *bears on* the claim it is offered for (§17.1) — and
	// conflict adjudication will land here with `checks_run` naming the predicates
	// that were tried and `checks_fired` empty.
	MissNoPredicate MissReason = "no_deterministic_predicate"
)

// MissReason is why a prompt was emitted rather than a question answered locally.
//
// **A closed type rather than a string, because the report branches on it.** §6.4's
// payoff is that "a reason that recurs is a deterministic check waiting to be written",
// and that is true of one of these two and false of the other — so the distinction has
// to be a property of the value rather than a rule each emitter remembers. Three
// emitters spelling one reason three ways would make the grouping meaningless and
// nothing would report it.
type MissReason string

// String renders the reason as §6.4's own token.
func (r MissReason) String() string {
	if r == MissReasonUnset {
		return "unset"
	}
	return string(r)
}

// MarshalText renders the reason in the machine envelope, so a reader sees the token
// rather than a bare string that happens to match.
func (r MissReason) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// Actionable reports whether a recurrence of this reason is a check waiting to be
// written (§6.4).
//
// Requires: nothing.
// Ensures: true only for MissNoPredicate. Pure.
//
// **This is the whole reason the type is closed.** Without it the report would count
// every model call together, and the commonest line — extraction, which has no
// deterministic alternative and never will — would dominate the one line that names
// work somebody could do. A report whose loudest row is unactionable is the noise this
// repository has withdrawn three times.
func (r MissReason) Actionable() bool { return r == MissNoPredicate }

// ParseMissReason reads a recorded reason.
//
// Requires: nothing.
// Ensures: the reason and true, or (MissReasonUnset, false) for anything the two do not
// name. Pure.
//
// Comma-ok rather than defaulting, because a row written by a later gnosis with a third
// reason must be *reported as unrecognised* rather than silently folded into one of
// these two — the count is the output, and a miscounted row is a wrong answer rather
// than a missing one.
func ParseMissReason(s string) (MissReason, bool) {
	switch MissReason(strings.ToLower(strings.TrimSpace(s))) {
	case MissNoPath:
		return MissNoPath, true
	case MissNoPredicate:
		return MissNoPredicate, true
	case MissReasonUnset:
		return MissReasonUnset, false
	default:
		return MissReasonUnset, false
	}
}
