package command

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Promote moves a quarantined document into the authoritative corpus (§9.4).
//
// It is stated over a path rather than over content because the gate runs on a
// **diff**: between checking a candidate and committing it there is a window, and
// a corpus whose gate can be raced is a corpus whose gate is decorative. The
// coordinator computes the diff once and Effect decides only whether the final
// write happens.
type Promote struct {
	// Path is the quarantined document, relative to the bundle root.
	Path string

	// Eff is preview or apply. It is named Eff because Effect is the accessor the
	// Command interface requires, and a field cannot share a method's name.
	Eff Effect

	// Approver is who authorised this. ActorUnset is rejected: a write whose
	// approver was never populated has no approver, and the point of carrying one
	// on the command is that it cannot be supplied silently later.
	Approver gnosis.Actor

	// Rationale is why, and whether one is required is not this type's business.
	//
	// **An earlier version carried a RequiresRationale flag beside it, and nothing
	// ever set it.** The requirement is a property of the gate's decision, which
	// only the coordinator can compute, so `authorisedBy` derives it there — and a
	// field whose only effect is to make the type look like it enforces something it
	// does not is worse than its absence. It is the pattern this project has now
	// recorded four times: stored state with no writer.
	Rationale string

	// Confirmation is the phrase a person typed to authorise a promotion the gate
	// would not approve on its own (§9.5). It must equal Path, not "yes": a
	// confirmation a reader can supply from muscle memory confirms nothing, and
	// naming the document is what makes them look at which one it is.
	//
	// It is a field on the command rather than a coordinator argument for the
	// reason every other gating field here is one — a write carries its own
	// authorisation, so no caller can acquire it after the fact (§4.6.2). Whether
	// it is *required* depends on the gate report, which only the coordinator can
	// compute; whether it is *present* is this type's business.
	Confirmation string
}

// Op names the operation.
func (p *Promote) Op() string { return "promote" }

// Effect reports whether this command writes.
func (p *Promote) Effect() Effect { return p.Eff }

// Validate reports why this command is not executable, or nil.
//
// Requires: nothing; a zero Promote is a valid input and is rejected.
// Ensures: every problem is reported at once, so one round trip tells a caller
// everything it must fix. An EINVALID code, because the command was constructed
// wrongly and no retry of the same value will help.
//
// **What this checks is what a command can be wrong about on its own**: a missing
// path, an unset effect, an approver that is not `<kind>:<id>`. It deliberately
// does not ask whether the rationale is required or whether the confirmation
// matches, because both depend on the gate's decision over this corpus — which
// `authorisedBy` has and this does not. A validator that guessed at them would
// refuse commands the gate would have approved, which is the failure mode a caller
// cannot diagnose.
func (p *Promote) Validate() error {
	const op = "command.Promote.Validate"

	var bad []string
	if strings.TrimSpace(p.Path) == "" {
		bad = append(bad, "path is empty")
	}
	if !p.Eff.Valid() {
		// Named rather than described, because "unset" is the case a caller hits
		// by forgetting the field and the message has to say which field.
		bad = append(bad, "effect is "+p.Eff.String()+"; set preview or apply")
	}
	if p.Approver == gnosis.ActorUnset {
		bad = append(bad, "approver is unset")
	} else if p.Approver.Kind() == "" {
		bad = append(bad, "approver "+string(p.Approver)+
			" is not <kind>:<id> with kind one of human, agent, check")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{
		Code:    errs.EINVALID,
		Message: op + ": " + strings.Join(bad, "; "),
	}
}
