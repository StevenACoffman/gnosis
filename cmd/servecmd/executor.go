package servecmd

import (
	"context"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/scan"
	"github.com/StevenACoffman/gnosis/internal/web"
	"github.com/StevenACoffman/skillet/errs"
)

// executor runs a web decision through the same coordinator the CLI drives.
//
// **The composition happens here because this is the composition root.** `internal/web`
// cannot import `internal/bundle` — depguard forbids it — so the server holds interfaces
// and this package, which is allowed to know both, joins them. That is rules.md's "main
// is the only place where concrete implementation packages are imported together",
// applied one level down at the command that wires them.
type executor struct {
	dir   string
	rules *scan.Ruleset

	// actorOf reads the authenticated caller from the request's context.
	//
	// A function rather than a value, because the actor is per request and this
	// object is per server. Injected rather than read directly, so the rule that an
	// approver comes from the transport is visible at the wiring site.
	actorOf func(ctx context.Context) gnosis.Actor
}

// Execute maps a web command onto a coordinator command and runs it.
//
// Requires: cmd passed web.Command.Valid; the caller was authenticated.
// Ensures: the outcome the coordinator produced, unchanged — the web surface reports the
// gate's verdict and never its own.
//
// # The approver comes from here and can come from nowhere else
//
// §4.6.2.1: "`Approver` is supplied by the transport, never by the payload". `web.Command`
// has no approver field, so this is the only place one can be set, and it is set from the
// actor the middleware authenticated.
//
// # The confirmation phrase does not survive the wire, and its absence is the rule
//
// §4.6.2.1 again: "until §13 has a review queue, the escalated path is refusable only at
// a terminal". Nothing here sets `Confirmation`, so a promotion the gate escalates is
// refused by the coordinator with `needs_human` — the interim rule enforced by omission
// rather than by a check that could be removed without anybody noticing.
func (e *executor) Execute(ctx context.Context, cmd web.Command) (gnosis.Outcome, error) {
	const op = "servecmd.executor.Execute"

	actor := e.actorOf(ctx)
	if actor == gnosis.ActorUnset {
		return gnosis.Outcome{}, &errs.Error{
			Code:    errs.EUNAUTHORIZED,
			Message: op + ": no authenticated actor, so no decision can be recorded",
		}
	}
	inner, err := e.build(op, cmd, actor)
	if err != nil {
		return gnosis.Outcome{}, err
	}
	coordinator := bundle.Coordinator{Dir: e.dir, Rules: e.rules}
	outcome, err := coordinator.Execute(ctx, inner)
	if err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	return withTransportRemedy(outcome), nil
}

// withTransportRemedy adds what a reviewer at this surface can actually do.
//
// Requires: outcome came from the coordinator.
// Ensures: the status, the code, the reason and the data are unchanged; only the
// human-readable message gains a sentence. Pure.
//
// # Why the message differs and the verdict does not
//
// §8.0 requires "one verdict, two renderings — never two verdicts", and is explicit that
// `message` "is for a person and MUST NOT be parsed". A blocked promotion asks the caller
// to "confirm by typing the document's path exactly" — which is the right instruction at
// a terminal and an impossible one here, because §4.6.2.1 says the confirmation phrase
// does not survive the wire and nothing on this path supplies it.
//
// So the verdict crosses unchanged and the remedy is corrected for where it is being
// read. A reviewer told to type something they cannot type concludes the server is
// broken; the honest sentence sends them to the one place that can finish it.
//
// **A hand run found this.** Every test asserted the refusal and none read it.
func withTransportRemedy(outcome gnosis.Outcome) gnosis.Outcome {
	if outcome.Status != gnosis.StatusBlocked || outcome.Reason != gnosis.ReasonNeedsHuman {
		return outcome
	}
	outcome.Message += " — this cannot be completed over the web (§4.6.2.1): the" +
		" confirmation is a person typing the path at a terminal, and §13's queue" +
		" token that would replace it is not built. Run `gnosis promote` there."
	return outcome
}

// build turns a web command into the coordinator's own.
//
// Requires: actor is the authenticated caller.
// Ensures: a command carrying the actor as its approver, or EINVALID for a kind this
// surface does not offer. Pure.
func (e *executor) build(
	op string, cmd web.Command, actor gnosis.Actor,
) (command.Command, error) {
	switch cmd.Kind {
	case web.CommandAdjudicate:
		return &command.Adjudicate{
			Path: cmd.Path, ClaimID: cmd.Claim, By: actor, Rationale: cmd.Rationale,
		}, nil
	case web.CommandPromote:
		return &command.Promote{
			Path: cmd.Path, Eff: command.EffectApply, Approver: actor,
			Rationale: cmd.Rationale,
		}, nil
	case web.CommandUnset:
		return nil, &errs.Error{
			Code: errs.EINVALID, Message: op + ": no kind named",
		}
	default:
		// Unreachable through the handler, which validates first. Named rather than
		// defaulted so a kind added to `web` has to be classified here, which is the
		// whole value of an exhaustive switch over a vocabulary that grows.
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + string(cmd.Kind) + " is not a mutation this serves",
		}
	}
}
