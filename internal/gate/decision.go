package gate

// The four things the gate can conclude about a candidate.
//
// DecisionUnavailable is the zero value, for the reason Verdict documents at
// length and this type will not re-argue: a value nobody populated must not
// assert. "The gate could not be trusted" is the safe reading of an unpopulated
// decision, and every other candidate for the zero would authorise something.
//
// The set exists because Withheld already distinguished a failure from an
// unchecked signal — and said in its own contract that the two "call for opposite
// responses" — after which Approved collapsed them both into false. This is that
// distinction carried through to the answer instead of being computed and thrown
// away.
const (
	DecisionUnavailable Decision = iota
	DecisionApproved
	DecisionNeedsHuman
	DecisionRefused
)

// Decision is the gate's whole conclusion, and it decides who may act.
//
// The load-bearing member is DecisionRefused, which nobody may override. A failed
// evidence check is not a judgement call: there is no confirmation phrase that
// makes a fabricated quotation acceptable, and no person senior enough to make one
// true. The human path (§9.5) opens for what could **not be checked**, never for
// what was checked and failed — and that line is the whole difference between an
// escalation and the `--yes` bypass §15 forbids.
type Decision int

// Decide reports the gate's conclusion.
//
// Requires: nothing; the zero Report decides Unavailable, which authorises
// nothing.
// Ensures: pure. Refused outranks NeedsHuman, so a candidate that both failed a
// signal and left one unchecked is refused rather than escalated — the reverse
// would offer a person a signature over a document with a known defect in it.
func (r *Report) Decide() Decision {
	if !r.Control.Held || len(r.Results) == 0 {
		return DecisionUnavailable
	}
	failed, unchecked := r.Withheld()
	switch {
	case len(failed) > 0:
		return DecisionRefused
	case len(unchecked) > 0:
		return DecisionNeedsHuman
	default:
		return DecisionApproved
	}
}

// Approved reports whether this candidate may be written with no human involved.
//
// Requires: nothing; the zero Report is valid and is not approved.
// Ensures: true only for DecisionApproved — control held and **every** signal
// passed. An unchecked signal withholds automatic approval exactly as a failed one
// does, because the question is "has this been shown to be admissible", and a
// check that did not run has shown nothing. What differs between them is what a
// person may then do about it, which is Decide's business rather than this one's.
func (r *Report) Approved() bool { return r.Decide() == DecisionApproved }

// Promotable reports whether a named human may promote this candidate by
// confirming it.
//
// Requires: nothing.
// Ensures: true only for DecisionNeedsHuman. False for Approved, which needs no
// human, and false for Refused, which admits none.
func (d Decision) Promotable() bool { return d == DecisionNeedsHuman }

// String renders the decision for a report a person reads.
func (d Decision) String() string {
	switch d {
	case DecisionApproved:
		return "approved"
	case DecisionNeedsHuman:
		return "needs_human"
	case DecisionRefused:
		return "refused"
	case DecisionUnavailable:
		return "unavailable"
	default:
		return "invalid"
	}
}

// MarshalText renders the decision in the machine envelope, so an agent branches
// on a word rather than on an integer whose meaning depends on declaration order.
func (d Decision) MarshalText() ([]byte, error) { return []byte(d.String()), nil }
