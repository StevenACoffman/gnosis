package gnosis

import (
	"sort"
	"strings"
)

// The six challenge classes of SPEC §10.7.1.
//
// A class is not a severity and not a topic. It states **what kind of thing would
// resolve the dispute**, which is what decides whether a person needs to be involved
// at all — and that is why `replay` is first among them rather than merely one of
// them.
const (
	// ChallengeReplay: "I checked the archived source and the quote is not there."
	// The only class gnosis can adjudicate itself, because the challenger is
	// asserting something mechanically decidable — so the response is not a
	// judgement but a re-run, and the corpus does not need to trust the challenger
	// at all.
	ChallengeReplay ChallengeClass = "replay"

	// ChallengeContradiction: "this claim conflicts with that one, and nothing
	// noticed." The class that closes a known hole: §6.2.1 records that the
	// candidate selector is systematically blind to conflicts between claims sharing
	// no source, no link and no vocabulary, and a reader who noticed one has done for
	// free what the selector provably cannot.
	ChallengeContradiction ChallengeClass = "contradiction"

	// ChallengeCoverage: "the evidence does not support the scope the claim asserts"
	// (§17.3.1).
	ChallengeCoverage ChallengeClass = "coverage"

	// ChallengeRung: "the claim is causal and its support is observational"
	// (§17.3.1.1).
	ChallengeRung ChallengeClass = "rung"

	// ChallengeDimensionDrift: "this subject's values changed dimension" (§5.8.2.1).
	ChallengeDimensionDrift ChallengeClass = "dimension-drift"

	// ChallengeScope: "the stated limitations are incomplete" (§17.2).
	ChallengeScope ChallengeClass = "scope"
)

// The three states a challenge can be in (§10.7.4).
//
// ChallengeOpen is the zero value, which is the honest default: a challenge nobody
// has recorded a disposition for is open, and the alternative would have an
// unpopulated value claim somebody dealt with it.
const (
	ChallengeOpen ChallengeState = "open"

	// ChallengeClosed: a decision was made. A **rejected** challenge closes this way
	// too, with a warrant explaining why the claim stands — never by deletion,
	// because a claim challenged three times and upheld three times is a different
	// artifact from one never questioned, and only one of them has evidence that
	// anyone looked.
	ChallengeClosed ChallengeState = "closed"

	// ChallengeDeferred: a person saw this and is not acting yet. It is a human
	// decision rather than a machine observation, which is why it is committed
	// alongside the challenge rather than cached (§10.7.4).
	ChallengeDeferred ChallengeState = "deferred"
)

// ChallengeClass is what kind of thing would settle a contested claim (§10.7.1).
type ChallengeClass string

// ChallengeState is where a challenge has got to (§10.7.4).
type ChallengeState string

// Challenge is a reader's contest of an accepted claim (§10.7).
//
// It is a first-class operation because the gap it fills is the most capable informant
// the corpus has: a reader who already knows a claim is wrong. Until there is a way in,
// such a person can open a pull request against a file — which routes a knowledge
// dispute through a diff review — or say something in chat, which the corpus never
// hears.
//
// **Being wrong costs the challenger nothing.** No count of rejected challenges
// attaches to an actor and none feeds a tier or a credibility signal. If challenging
// carries risk, the people best placed to challenge — the ones with most at stake in
// the claim being right — are the ones who stop.
type Challenge struct {
	// ID is the challenge's own identifier, so a resolution can name it.
	ID ID

	// Class states what would settle it.
	Class ChallengeClass

	// By is the challenger, as OKF §7's grammar writes it.
	By string

	// At is when it was filed, RFC 3339.
	At string

	// Rationale is required and non-empty (§10.7.2). A reader who cannot say why a
	// claim is wrong has filed a doubt rather than a challenge, and the corpus has
	// no way to act on a doubt. The same discipline as §10.6.4's warrant and for the
	// same reason: it costs one field and screens out most of what should never have
	// been filed.
	Rationale string

	// State is where it has got to. Empty reads as open.
	State ChallengeState
}

// ChallengeClasses lists the declared classes, sorted, for a diagnostic that has to
// name them.
//
// Requires: nothing.
// Ensures: a freshly built slice, so a caller cannot reorder the list another caller
// will see. Pure.
func ChallengeClasses() []string {
	out := []string{
		string(ChallengeReplay), string(ChallengeContradiction),
		string(ChallengeCoverage), string(ChallengeRung),
		string(ChallengeDimensionDrift), string(ChallengeScope),
	}
	sort.Strings(out)
	return out
}

// ParseChallengeClass reads a declared class.
//
// Requires: nothing.
// Ensures: the class and true, or ("", false) for anything the six do not name. Pure.
//
// A closed set with a parser rather than a validated string, so a class outside §10.7.1
// cannot reach a check: the type carries the constraint and no caller has to remember
// to test for it. The same shape ParseActor has, and for the same reason.
func ParseChallengeClass(s string) (ChallengeClass, bool) {
	switch ChallengeClass(strings.ToLower(strings.TrimSpace(s))) {
	case ChallengeReplay:
		return ChallengeReplay, true
	case ChallengeContradiction:
		return ChallengeContradiction, true
	case ChallengeCoverage:
		return ChallengeCoverage, true
	case ChallengeRung:
		return ChallengeRung, true
	case ChallengeDimensionDrift:
		return ChallengeDimensionDrift, true
	case ChallengeScope:
		return ChallengeScope, true
	default:
		return "", false
	}
}

// SelfAdjudicating reports whether gnosis can settle this class without a person.
//
// Requires: nothing.
// Ensures: true only for `replay`. Pure.
//
// `replay` is the strongest and the cheapest class, and it is worth being precise
// about why: the challenger asserts something mechanically decidable, so the response
// is a re-run rather than a judgement — and if the challenger is right, the resulting
// finding is an ordinary error-severity evidence failure that would have blocked
// anyway.
func (c ChallengeClass) SelfAdjudicating() bool { return c == ChallengeReplay }

// Open reports whether this challenge is still awaiting a decision.
//
// Requires: nothing.
// Ensures: true for the empty state, which is what a challenge filed by a writer that
// declared none is. Pure.
func (c *Challenge) Open() bool {
	return c.State == ChallengeOpen || c.State == ""
}
