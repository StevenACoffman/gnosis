package command

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Adjudicate records a human decision on a claim and writes its warrant (§10.4).
//
// When two claims conflict and a person decides, the decision is knowledge present in
// neither source: it can carry no quote, so it fails the evidence invariant by
// construction — the highest-value artifact a team produces, rejected by the check that
// exists to protect quality. §10.4's answer is a second provenance class whose warrant
// is the review, the people, the date and the rationale, and this is the command that
// writes one.
//
// # Why the adjudicator must be a person
//
// §10.6.4's count is of **distinct human actors**, and an agent adjudicating would be
// an agent granting authority to itself — the collapse §9.5 refuses on the promotion
// path for the same reason. `Challenge.By` and `Discard.By` may be agents because
// neither grants anything; this one does.
type Adjudicate struct {
	// Path is the document holding the claim, relative to the bundle root.
	Path string

	// ClaimID names the claim being adjudicated. Required: a warrant on a document
	// would assert that somebody decided every claim on the page.
	ClaimID string

	// By is the adjudicator, and must be a `human:` actor.
	By gnosis.Actor

	// Rationale is why it was decided this way. Required at every authority
	// including sole, and §10.6.4 makes it the real gate: a permission bit asks
	// whether somebody may decide, and this asks them to write down why, in a
	// commit, in front of colleagues.
	Rationale string

	// CoSigner is the second signature an escalated claim needs at paired or quorum.
	CoSigner gnosis.Actor

	// Override is the recorded reason a required co-signature was waived. Present
	// only when one was, and permitted only where the authority allows it — at
	// quorum there are four or more adjudicators, so an unavailable one is not a
	// reason the corpus cannot proceed.
	Override string

	// Reverses names the warrant this decision overturns (§10.6.5). A link and never
	// a judgement: no score attaches to a reversed warrant and no reputation to its
	// author.
	Reverses string

	// Challenge is the challenge this decision answers, if any. Closing one is what
	// makes a challenge a route into the existing lifecycle rather than a parallel
	// one — including when the claim stands, because a rejected challenge closes
	// with a warrant and is never deleted.
	Challenge string

	// Eff is preview or apply.
	Eff Effect
}

// Op names the operation.
func (a *Adjudicate) Op() string { return "adjudicate" }

// Effect reports whether this command writes.
func (a *Adjudicate) Effect() Effect { return a.Eff }

// Validate reports why this command is not executable, or nil.
//
// Requires: nothing; a zero Adjudicate is a valid input and is rejected.
// Ensures: every problem at once, EINVALID.
//
// **The authority is not checked here**, deliberately. Whether a co-signer is required
// depends on the corpus's adjudicators and on whether the claim is escalated, neither
// of which a pure value knows — so that gate lives in the coordinator, where the corpus
// is in hand. What this checks is the shape: a decision with no decider, no claim, or
// no reasoning is not an adjudication whatever the corpus looks like.
func (a *Adjudicate) Validate() error {
	const op = "command.Adjudicate.Validate"

	var bad []string
	if strings.TrimSpace(a.Path) == "" {
		bad = append(bad, "path is empty")
	}
	if strings.TrimSpace(a.ClaimID) == "" {
		bad = append(bad, "claim is empty; a warrant records a decision about one "+
			"assertion, and a document-level one would claim every assertion on the page")
	}
	if !a.Eff.Valid() {
		bad = append(bad, "effect is "+a.Eff.String()+"; set preview or apply")
	}
	bad = append(bad, a.badActors()...)
	if strings.TrimSpace(a.Rationale) == "" {
		bad = append(bad, "rationale is empty; §10.6.4 requires one at every authority "+
			"including sole, because the reader you are writing for is yourself in six months")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}

// badActors reports what is wrong with the adjudicator and the co-signer.
//
// Separate from Validate so neither function has to be read twice: this one holds the
// whole of "who may sign", including the rule that they must be different people — a
// co-signature by the adjudicator is a second signature that checked nothing, and it is
// the cheapest way to satisfy the requirement without meeting it.
func (a *Adjudicate) badActors() []string {
	var bad []string
	switch {
	case a.By == gnosis.ActorUnset:
		bad = append(bad, "by is unset")
	case !gnosis.IsHumanActor(string(a.By)):
		bad = append(bad, "by "+string(a.By)+" is not a person; an adjudication is a "+
			"human decision (§10.6.4) and an agent signing one would grant itself authority")
	}
	if a.CoSigner != gnosis.ActorUnset {
		if !gnosis.IsHumanActor(string(a.CoSigner)) {
			bad = append(bad, "co-signer "+string(a.CoSigner)+" is not a person")
		}
		if a.CoSigner == a.By {
			bad = append(bad, "the co-signer is the adjudicator; a second signature "+
				"by the same person checked nothing")
		}
	}
	return bad
}
