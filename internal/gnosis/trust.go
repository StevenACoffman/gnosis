package gnosis

import "strings"

// The three trust tiers of OKF §5.3, as SPEC §14.1 folds them.
//
// TierUnverified is the zero value, and that is the whole safety property of this
// type: a concept with no trust frontmatter, a fold nobody ran, and a value nobody
// populated all read as unverified. The opposite default would have an unpopulated
// value claim the strongest tier in the set.
const (
	TierUnverified Tier = iota
	TierMachineConfirmed
	TierHumanReviewed
)

// Tier is how far a concept's `verified` list has been confirmed.
//
// It is advisory (OKF §5.3) and never a permission. SPEC §14.1 states the rule and
// it is worth keeping next to the type, because the pressure to make a tier gate
// something will come from whoever maintains the largest set of claims: a tier is a
// **signal about provenance**, and a concept with no trust frontmatter MUST stay
// consumable (OKF §11).
type Tier int

// String is the token OKF §5.3 names each tier by.
func (t Tier) String() string {
	switch t {
	case TierHumanReviewed:
		return "human-reviewed"
	case TierMachineConfirmed:
		return "machine-confirmed"
	case TierUnverified:
		return "unverified"
	default:
		// An unrecognised tier reports the weakest thing it could be. A tier that
		// fell through to a name asserting confirmation would be the one failure
		// direction this type cannot afford.
		return "unverified"
	}
}

// MarshalText renders the tier as a word in the machine envelope, so an agent
// reading it sees the token OKF §5.3 names rather than an integer whose meaning
// depends on the order of the constants above.
//
// It exists because the type was built in Step 2.12 with no consumer, and an
// integer is what the envelope would have carried on the day one arrived — the
// same defect as a value nobody reads, wearing its other face. `Freshness` and
// `DriftState` carry this method for the same reason.
func (t Tier) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

// Tiers lists the declared tiers weakest first, for a diagnostic that has to name them.
//
// Requires: nothing.
// Ensures: a freshly built slice, so a caller cannot reorder the list another caller
// will see. Pure.
func Tiers() []string {
	return []string{
		TierUnverified.String(), TierMachineConfirmed.String(),
		TierHumanReviewed.String(),
	}
}

// ParseTier reads a tier by the token OKF §5.3 names it.
//
// Requires: nothing.
// Ensures: the tier and true, or (TierUnverified, false) for anything the three do not
// name. Pure.
//
// **Comma-ok rather than defaulting to unverified**, because the caller here is a flag:
// a reader who typed `--trust reviewed` must be told the word is wrong rather than
// handed every document in the corpus, which is what the weakest tier would admit.
func ParseTier(s string) (Tier, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case TierHumanReviewed.String():
		return TierHumanReviewed, true
	case TierMachineConfirmed.String():
		return TierMachineConfirmed, true
	case TierUnverified.String():
		return TierUnverified, true
	default:
		return TierUnverified, false
	}
}

// FoldTrust derives a concept's tier from the actors in its `verified` list.
//
// Requires: nothing. A nil or empty list is unverified, which is the state of every
// document in a corpus that has not started recording verification.
// Ensures: TierHumanReviewed when any actor carries the `human:` prefix,
// TierMachineConfirmed when the list is non-empty and none does, TierUnverified for
// an empty list. Pure, total, and it never returns an error.
//
// # Why this takes strings and not Actor
//
// SPEC §14.1.1 states the divergence and §18.5.1 pins it: `gnosis.Actor` is a
// **closed** three-kind enum, and OKF §7's grammar has forms it refuses —
// `process:<id>` and `<producer>/<version>`. Both are conformant OKF, and §11
// forbids rejecting a concept for the shape of an optional family. So this fold
// runs over the raw strings.
//
// The two populations are deliberately not merged, and merging them is the natural
// thing to write:
//
//   - Widening Actor to accept OKF's forms would give up what §10.6.4 depends on.
//     That section counts *distinct human actors* to decide whether a review tier
//     amplified anything, and a kind that could pass for a person makes the count
//     wrong in the direction that flatters the corpus.
//   - Narrowing this fold to Actor would reject conformant documents. A concept
//     carrying `verified: [{by: reference_agent/gemini-2.5-pro}]` is valid OKF and
//     ParseActor refuses it.
//
// So this asks the one question OKF §7 says a trust classifier needs — is the actor
// `human:`-prefixed? — and nothing else. It is strictly more permissive than
// ParseActor and strictly less capable: it cannot say *which* non-human wrote
// something, which is right, because the tier does not depend on that and a reader
// who wants it can read the field.
//
// **An unrecognised actor is never an error and never promotes a tier.** That is the
// sentence the whole design rests on, and it is why the default branch of every
// switch here goes to the weaker answer.
//
// It is a pure function over []string rather than a method on anything, because
// `skillet` will promote exactly this shape when a second repository classifies an
// actor, and a function with no receiver lifts unchanged.
func FoldTrust(verifiedBy []string) Tier {
	tier := TierUnverified
	for _, by := range verifiedBy {
		if strings.TrimSpace(by) == "" {
			// A blank entry is not an actor. Counting it as machine-confirmed would
			// let an empty list item promote a tier, which is the one thing a fold
			// over untrusted frontmatter must not do.
			continue
		}
		if IsHumanActor(by) {
			// Human review is the strongest tier, so nothing later can lower it and
			// there is no reason to keep looking.
			return TierHumanReviewed
		}
		tier = TierMachineConfirmed
	}
	return tier
}

// IsHumanActor reports whether a raw actor string names a person.
//
// Requires: nothing.
// Ensures: true only for the `human:` prefix with a non-empty identifier. Pure.
//
// It is the permissive read of §14.1.1, separate from Actor.IsHuman: that method
// asks its question of a value this package minted, and this function asks it of a
// string that arrived from somebody else's frontmatter. `process:finance-nightly`
// answers false here and is *rejected* by ParseActor, which is the divergence
// §18.5.1's table exists to pin.
//
// The prefix is matched exactly and case-sensitively. OKF §7 specifies `human:` and
// accepting `Human:` would be guessing at a producer's intent in the direction that
// raises a tier.
func IsHumanActor(by string) bool {
	kind, id, found := strings.Cut(by, ":")
	return found && id != "" && kind == KindHuman
}

// FoldTrustDocument derives a document's tier from the tiers of its claims.
//
// Requires: nothing. A document declaring no claims is TierUnverified, which is
// what a page carrying no verification is — not an error and not a fourth state.
// Ensures: the weakest tier among the claims. Pure, total.
//
// **The weakest, for the reason DocFreshness takes the oldest check.** A page is
// only as reviewed as its least reviewed assertion, and reporting the strongest
// would let one human-reviewed claim vouch for every unverified sentence beside
// it. The two grains are both reported, so a reader who wants to know *which*
// sentence is unverified is not left inferring it from a page-level word.
//
// It takes tiers rather than claims because the per-claim fold is FoldTrust's, and
// a function taking claims here would have to know how a claim stores its actors —
// which is exactly the frontmatter knowledge the domain must not carry.
func FoldTrustDocument(claims []Tier) Tier {
	if len(claims) == 0 {
		return TierUnverified
	}
	weakest := TierHumanReviewed
	for _, t := range claims {
		if t < weakest {
			weakest = t
		}
	}
	return weakest
}
