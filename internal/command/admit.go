package command

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Admit consumes an agent's reply to an extraction prompt and, on pass, writes it
// to quarantine (SPEC §8.2).
//
// It is the second command, and therefore the first evidence about whether
// §4.6.2's interface was worth having. It was: nothing here re-states how a write
// is gated, and `Effect`, validation, and the coordinator's lock all applied
// without a line of new plumbing.
//
// A reply is the least trustworthy input the system takes, so admit carries an
// approver like a promotion does. The tiers differ — admitting to quarantine is a
// smaller act than promoting out of it, and the review requirements in §10.6 scale
// accordingly — but "who caused this content to exist" is a question quarantine
// has to answer too, because a document nobody can attribute is a document nobody
// will take responsibility for reviewing.
type Admit struct {
	// Key is the §6.1 cache key the reply answers. It ties the reply to the exact
	// prompt, model, and source version that produced it — without it, a reply
	// could be filed against a question nobody asked.
	Key string

	// Reply is the agent's raw response, as received.
	Reply string

	// Eff is preview or apply.
	Eff Effect

	// Submitter is who supplied the reply. An agent is a legitimate submitter
	// here, unlike at a promotion tier requiring human review.
	Submitter gnosis.Actor
}

// Op names the operation.
func (a *Admit) Op() string { return "admit" }

// Effect reports whether this command writes.
func (a *Admit) Effect() Effect { return a.Eff }

// Validate reports why this command is not executable, or nil.
//
// Requires: nothing; a zero Admit is a valid input and is rejected.
// Ensures: every problem at once, and EINVALID, because the command was
// constructed wrongly and no retry of the same value will help.
//
// It does not parse the reply. Whether the reply is well-formed is a question
// about content and belongs to the handler, which can report it as a finding
// against the reply rather than as a malformed command — a caller told "your
// command is invalid" would go looking at their flags.
func (a *Admit) Validate() error {
	const op = "command.Admit.Validate"

	var bad []string
	if strings.TrimSpace(a.Key) == "" {
		bad = append(bad, "key is empty; it must name the prompt this answers")
	}
	if strings.TrimSpace(a.Reply) == "" {
		bad = append(bad, "reply is empty")
	}
	if !a.Eff.Valid() {
		bad = append(bad, "effect is "+a.Eff.String()+"; set preview or apply")
	}
	if a.Submitter == gnosis.ActorUnset {
		bad = append(bad, "submitter is unset")
	} else if a.Submitter.Kind() == "" {
		bad = append(bad, "submitter "+string(a.Submitter)+
			" is not <kind>:<id> with kind one of human, agent, check")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}
