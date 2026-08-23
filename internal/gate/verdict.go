package gate

// The three verdicts a signal can reach.
//
// VerdictUnchecked is the zero value, and it **blocks**. That is the whole
// disagreement this type settles. Two of §9.5's seven signals have no subsystem
// to read yet, and the tempting options were to omit them — a silent pass on
// evidence nobody examined — or to fail them, which would be a lie, because the
// signal did not fail, it did not run. Reporting Unchecked and blocking on it is
// `quotecheck.Unchecked` one level up: "nobody looked" is not the claim "this is
// fine".
//
// §17.0.1 states the principle: a read path that cannot refuse is not
// trustworthy. A gate that cannot say "I could not check this" has only one
// answer available and is therefore not answering.
const (
	VerdictUnchecked Verdict = iota
	VerdictPass
	VerdictFail
)

// Verdict is one signal's conclusion about one candidate.
type Verdict int

// Approves reports whether this verdict permits a write.
//
// Requires: nothing.
// Ensures: true only for VerdictPass. Unchecked and Fail both withhold approval,
// and any value outside the enumeration does too — the direction in which a
// mistake refuses a legitimate write rather than admitting an illegitimate one.
func (v Verdict) Approves() bool { return v == VerdictPass }

// String renders the verdict for a report a person reads.
func (v Verdict) String() string {
	switch v {
	case VerdictPass:
		return "pass"
	case VerdictFail:
		return "fail"
	case VerdictUnchecked:
		return "unchecked"
	default:
		return "invalid"
	}
}

// MarshalText renders the verdict in the machine envelope, so an agent branches on
// a word rather than on an integer whose meaning depends on declaration order.
func (v Verdict) MarshalText() ([]byte, error) { return []byte(v.String()), nil }
