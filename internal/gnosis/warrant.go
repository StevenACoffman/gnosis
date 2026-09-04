package gnosis

import (
	"strconv"
	"strings"
)

// The three adjudication authorities of SPEC §10.6.1, derived from the adjudicators a
// corpus actually has.
//
// AuthoritySole is the zero value, and it is the safe one for a reason that inverts
// the usual argument. Every other zero value here is chosen so an unpopulated value
// cannot flatter the corpus; this one is chosen so an unpopulated value cannot
// *block*. At `sole` there is no second signer to require, so a corpus whose fold
// never ran keeps working and records rather than refusing — which is §10.6.3's
// fourth property, that escalation must never deadlock.
const (
	AuthoritySole Authority = iota
	AuthorityPaired
	AuthorityQuorum
)

// Authority is how much co-signing an escalated adjudication needs (§10.6.1).
//
// **Not named Tier, although §10.6.1 calls it one.** `gnosis.Tier` already means
// §14.1's trust tier: a different fold, over different data, with different
// consequences — advisory there, admission-governing here. Two folds named Tier in one
// package is a name a reader has to disambiguate at every call site, and the rule
// against reusing one name for dissimilar things is what this repository leans on when
// `hasUpstream` and `everChecked` look like the same question.
//
// It is derived and never configured, which is the same discipline as trust tiers
// (§14.1), credibility (§14.2), durability (§14.4) and check applicability (§12): a
// fold over recorded facts rather than a flag somebody must remember to change. It
// moves in both directions on its own as people arrive and leave.
type Authority int

// AuthorityMove is a change in the corpus's adjudication authority (§10.6.3).
//
// "**A tier change is announced, never silent.** When the derived tier moves, `gnosis
// doctor` and `log.md` say so and say why. A gate that tightens or loosens without
// telling anyone is the 'no silent caps' failure this specification refuses elsewhere."
//
// **It is a value rather than a pair of authorities**, because the announcement and the
// fact are one thing: `log.md` records this, `doctor` reads it back, and a line and a
// comparison built separately are two spellings of one decision. The count travels with
// it because §10.6.3 asks the announcement to say *why*, and the why is the population —
// a reader told the corpus moved to `paired` still cannot tell whether one person
// arrived or three.
type AuthorityMove struct {
	// From is the authority the corpus last announced, and To is the one it derives
	// now.
	From Authority
	To   Authority

	// Adjudicators is the count behind To — the distinct human actors §10.6 counts.
	Adjudicators int
}

// Warrant is the record of a human adjudication (§10.6.4).
//
// An adjudicated claim carries no quotation by construction — the decision is
// knowledge present in neither source — so its only warrant is that a person said so,
// and this is that record. It ships whole or not at all: `skillet` and `canonizer`
// both cite §10.6.4 as the family's model of adjudication authority, so a subset under
// this name would break something that is not in this repository to notice.
//
// **It carries no role and no team**, and §10.6.2.1 gives three reasons of which the
// structural one decides it: `gate.Candidate` already holds the parsed document, so a
// role field would sit inside what the gate reads and only a comment would keep it out
// of a permission decision. Accountability is a property of the subject, reported and
// never enforced.
type Warrant struct {
	// By is the adjudicator, as OKF §7's grammar writes it. A raw string for the
	// reason Verification.By is one: §14.1.1 makes frontmatter actors a wider
	// population than gnosis.Actor, and rejecting a conformant document for the shape
	// of an optional family is what OKF §11 forbids.
	By string

	// At is when the decision was made, as declared.
	At string

	// Authority is the one in force when the decision was made, recorded so scaling
	// down never invalidates what was already decided (§10.6.3). Empty for a warrant
	// that declares none, which is not the same as `sole` — see AuthorityOf.
	Authority string

	// Review is where the decision was discussed, usually a pull request.
	Review string

	// Rationale is required and non-empty at every authority including sole
	// (§10.6.4). It is the field that does the work: a permission bit asks whether
	// somebody may decide, and this asks them to write down why, in front of
	// colleagues.
	Rationale string

	// CoSignedBy is the second signer, required at paired and quorum for an
	// escalated claim.
	CoSignedBy string

	// OverrideReason is present only when a required co-signature was waived. The
	// recording *is* the mechanism: a waiver that leaves no trace is
	// indistinguishable from an authority that was never in force, and a gate whose
	// overrides are countable is still a gate.
	OverrideReason string

	// Reverses names the warrant this one overturns — the warrant, not the claim
	// (§10.6.5). A link and never a judgement: no score attaches to a reversed
	// warrant and no reputation to its author, because reversal is the ordinary
	// consequence of deciding under incomplete information.
	Reverses string
}

// String is the token §10.6.1 names each authority by.
func (a Authority) String() string {
	switch a {
	case AuthorityQuorum:
		return "quorum"
	case AuthorityPaired:
		return "paired"
	case AuthoritySole:
		return "sole"
	default:
		// An unrecognised value reports the one that requires least, because the
		// failure this type cannot afford is a corpus that cannot adjudicate at all.
		return "sole"
	}
}

// MarshalText renders the authority as a word in the machine envelope.
func (a Authority) MarshalText() ([]byte, error) { return []byte(a.String()), nil }

// UnmarshalText reads the word back, so §8.0's envelope round-trips.
//
// **It exists because a consumer needs it**, which is the only reason a codec's second
// half should be written: `doctor`'s envelope carries the authority and its test reads
// the envelope back into the same type, so a word that marshalled and would not parse
// made the machine interface one-way.
//
// An unrecognised word is `sole` and not an error, for the reason AuthorityOf gives one
// layer up: refusing to decode a corpus's envelope because a later gnosis wrote an
// authority this one does not know would fail closed in the direction that stops work.
//
// The sibling word-typed values here — Tier, Durability, Weakness, Freshness,
// DriftState — deliberately have no UnmarshalText: nothing reads any of them back yet,
// and each gains the method at its first consumer rather than in a symmetry pass.
func (a *Authority) UnmarshalText(text []byte) error {
	*a, _ = AuthorityOf(string(text))
	return nil
}

// RequiresCoSigner reports whether an escalated claim needs a second signature.
//
// Requires: nothing.
// Ensures: false at sole, true at paired and quorum. Pure.
//
// The two that require one differ in whether the requirement can be waived, which is
// OverridePermitted's question rather than this one — keeping them separate is what
// stops a caller reading "requires a co-signer" as "cannot proceed without one".
func (a Authority) RequiresCoSigner() bool {
	return a == AuthorityPaired || a == AuthorityQuorum
}

// OverridePermitted reports whether a missing co-signature may be waived with a
// recorded reason.
//
// Requires: nothing.
// Ensures: true everywhere except quorum. Pure.
//
// At sole it is vacuously true because nothing is required; at paired it is the escape
// hatch §10.6.3 insists on, because a queue that can block indefinitely stops being
// used and an unused queue admits nothing. At quorum there are four or more
// adjudicators, so an unavailable one is not a reason the corpus cannot proceed.
func (a Authority) OverridePermitted() bool {
	return a != AuthorityQuorum
}

// FoldAuthority derives the corpus's adjudication authority from its adjudicators.
//
// Requires: adjudicators is the count of **distinct** human actors appearing in
// warrants and verification lists within the declared window.
// Ensures: sole for zero or one, paired for two or three, quorum for four or more.
// Pure, total.
//
// **A corpus with no adjudicators folds to sole rather than to nothing.** There is no
// fourth state, and adding one would mean a fresh corpus could not adjudicate its
// first conflict until somebody had already adjudicated one.
//
// This is variety matching rather than a convenience: a controller needs at least as
// much variety as the system it governs, so a corpus whose contradictions outrun its
// adjudicators has a real deficit, and scaling authority with population is the
// legitimate way to close it. The illegitimate way — adding process steps — is
// amplification by volume, which is why no authority here adds a *step* without adding
// a different *kind* of reviewer.
func FoldAuthority(adjudicators int) Authority {
	switch {
	case adjudicators >= 4:
		return AuthorityQuorum
	case adjudicators >= 2:
		return AuthorityPaired
	default:
		return AuthoritySole
	}
}

// AuthorityOf reads the authority a warrant records as having been in force.
//
// Requires: declared is the warrant's `tier` field, which may be empty.
// Ensures: the named authority and true, or (AuthoritySole, false) when the warrant
// declares none or names something unrecognised. Pure.
//
// **Comma-ok rather than defaulting silently**, because "the warrant says sole" and
// "the warrant says nothing" are different facts about a decision, and only the second
// leaves a reader unable to tell what was required at the time. A caller that needs an
// answer anyway can fall back to the corpus's current authority and say that it did.
func AuthorityOf(declared string) (Authority, bool) {
	switch strings.ToLower(strings.TrimSpace(declared)) {
	case "quorum":
		return AuthorityQuorum, true
	case "paired":
		return AuthorityPaired, true
	case "sole":
		return AuthoritySole, true
	default:
		return AuthoritySole, false
	}
}

// Adjudicated reports whether this warrant records a decision at all.
//
// Requires: nothing.
// Ensures: false for the zero Warrant. Pure.
//
// A pointer receiver because the value is eight strings and the vet-level rule about
// copying it is worth obeying even for a predicate: a method that copies 128 bytes to
// answer a boolean is the kind of thing that gets copied itself.
//
// A warrant is the marker of §10.4's adjudicated provenance class, and the class is
// therefore **derived rather than declared**. A `class: adjudicated` key would let a
// claim assert the class and carry no decision, which is the state a reader most needs
// to be told about — so the assertion and the evidence for it would be the same field,
// wearing different names.
func (w *Warrant) Adjudicated() bool {
	return w.By != "" || w.At != "" || w.Rationale != "" || w.Review != "" ||
		w.CoSignedBy != "" || w.OverrideReason != "" || w.Reverses != "" ||
		w.Authority != ""
}

// Moved reports whether this is a change worth announcing.
//
// Requires: nothing.
// Ensures: false when the two authorities agree, whatever the counts. Pure.
//
// **The count moving is not the authority moving**, and that is the whole reason this
// is a method rather than a comparison at each call site. A corpus going from two
// adjudicators to three stays at `paired`; announcing it would put a line in `log.md`
// that says nothing changed, and a log whose entries mean nothing is one people stop
// reading — the same argument §12 makes for a check that reports the ordinary case.
func (m AuthorityMove) Moved() bool { return m.From != m.To }

// String is the sentence the announcement and the diagnostic both use.
//
// One rendering rather than two, because a reader comparing what `log.md` recorded with
// what `doctor` says must not have to work out whether two differently worded sentences
// describe one event.
func (m AuthorityMove) String() string {
	return m.From.String() + " → " + m.To.String() + ", " +
		adjudicatorCount(m.Adjudicators)
}

// adjudicatorCount renders the population behind an authority.
//
// The count sits in a noun phrase so no verb has to agree with it (§17.5), and the zero
// case says what it means rather than reading as an error: a corpus nobody has
// adjudicated in is at `sole`, which requires nothing and is a supported configuration.
func adjudicatorCount(n int) string {
	switch n {
	case 0:
		return "no adjudicator has signed anything yet"
	case 1:
		return "1 distinct human adjudicator"
	default:
		return strconv.Itoa(n) + " distinct human adjudicators"
	}
}
