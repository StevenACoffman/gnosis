package gnosis

import "time"

// The four freshness states of SPEC §14.3.
//
// FreshnessUnknown is the zero value, and that placement is the whole point of
// the type. §14.3 keeps four states rather than three because `unknown` (never
// checked) and `not_applicable` (no upstream to compare) are genuinely distinct
// from `stale`, and **collapsing them turns "we never looked" into "it is fine"**.
// A zero value of Fresh would perform that collapse by default, on every document
// nobody has examined yet, which is every document in a new corpus.
//
// This is the third place the same discipline appears — `quotecheck.Unchecked`,
// `gate.VerdictUnchecked`, and here — and the repetition is not accidental. Each
// is a read path that has to be able to say it did not look.
//
// It is an integer enumeration rather than a string one for that reason alone: a
// string type's zero value is "", which is neither `unknown` nor anything else, and
// writing a comment claiming otherwise does not make it so. The first draft here
// did exactly that and its own test caught it. `gate.Verdict` had already solved
// the same problem the same way, which is the argument for the shape.
const (
	FreshnessUnknown Freshness = iota
	FreshnessFresh
	FreshnessStale
	FreshnessNotApplicable
)

// Freshness is how current a source is believed to be.
type Freshness int

// String renders the state as §14.3's vocabulary writes it.
func (f Freshness) String() string {
	switch f {
	case FreshnessFresh:
		return "fresh"
	case FreshnessStale:
		return "stale"
	case FreshnessNotApplicable:
		return "not_applicable"
	case FreshnessUnknown:
		return "unknown"
	default:
		return "invalid"
	}
}

// MarshalText renders the state as a word in the machine envelope, so an agent
// branches on "unknown" rather than on the integer 0, whose meaning depends on
// declaration order.
func (f Freshness) MarshalText() ([]byte, error) { return []byte(f.String()), nil }

// Trustworthy reports whether a claim resting on this source can be relied on
// without a fresh look.
//
// Requires: nothing.
// Ensures: true only for FreshnessFresh. Unknown is not trustworthy — that is the
// distinction the four-state vocabulary exists to preserve — and NotApplicable is
// not either, because a source with no upstream is not thereby current, it is
// merely not checkable. A caller wanting "nothing is wrong here" should ask for
// that, and this is not it.
func (f Freshness) Trustworthy() bool { return f == FreshnessFresh }

// FreshnessOf computes a source's state.
//
// Requires: now is the moment to judge against; checkedAt is when this user last
// verified the source against upstream, or the zero time if never; staleAfter is
// the document's declared expiry, or the zero time if it declares none;
// hasUpstream reports whether there is anything to check against.
// Ensures: never Fresh for a source that was never checked. Pure — the clock is a
// parameter, so two runs at one instant agree and a test can pin the answer
// exactly rather than asserting a range.
//
// The order of the tests is the order of the questions. Is there an upstream at
// all? Then: has anyone looked? Then: has the document declared itself expired?
// Only a source that survives all three is fresh. Reversing any pair would let a
// later question answer one an earlier one should have refused — a document with
// a future `stale_after` that nobody ever checked would read as fresh if the date
// were tested first, which is precisely §14.3's collapse.
//
// **`stale_after` is an absolute date and not a TTL**, which OKF argues for on
// determinism grounds: a date "keeps the staleness decision a plain date
// comparison with no reference to when the concept was read." That is why this
// takes a date rather than a duration, and why passing `now` costs nothing.
func FreshnessOf(now, checkedAt, staleAfter time.Time, hasUpstream bool) Freshness {
	switch {
	case !hasUpstream:
		// Nothing to compare against. A `referenced` source with no archived text
		// and no reachable original is in this state permanently, and reporting it
		// as stale would suggest an action nobody can take.
		return FreshnessNotApplicable
	case checkedAt.IsZero():
		return FreshnessUnknown
	case !staleAfter.IsZero() && !now.Before(staleAfter):
		// The document declared its own expiry and it has passed. This outranks
		// having been checked: a source can be verified unchanged and still be
		// past the date its author said it should be revisited.
		return FreshnessStale
	default:
		return FreshnessFresh
	}
}
