package web

import (
	"context"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// The mutations the web surface offers.
//
// **`CommandUnset` is the zero value and is refused**, which is `EffectUnset`'s rule one
// layer down: a payload whose kind field was forgotten must not fall through to the
// first arm of a switch.
//
// **There is no kind that writes a concept body.** §13: "No prose editing. The corpus
// body is model-written by design. The web UI writes warrants, adjudications, and
// approvals — never concept bodies." The absence is the mechanism.
const (
	CommandUnset CommandKind = ""

	// CommandAdjudicate records §10.4's decision on a claim.
	CommandAdjudicate CommandKind = "adjudicate"

	// CommandPromote applies §9.5's promotion of a quarantined draft.
	CommandPromote CommandKind = "promote"
)

// Queue is what needs a person's decision.
//
// **One method, because the queue is one question**: what is waiting, and enough about
// each item to decide with. §13 makes that the higher-leverage investment than any
// authorization rule — "if the queue shows enough, a non-expert correctly recognizes
// when to defer; if it shows too little, even an expert guesses" — so the weight is in
// the Item type rather than in the calls.
type Queue interface {
	// Waiting is everything needing a decision, ordered so two loads of the page
	// present the same list in the same order. A reviewer who has to re-find their
	// place after every refresh stops using the queue.
	Waiting(ctx context.Context) ([]Item, error)
}

// Reader is the corpus as the viewer sees it: never a writer, and structurally so.
//
// §4.6 requires that a reader never depend on the writer, and this interface is where
// that is enforceable — an implementation could take the lock, and a handler holding
// only this cannot ask it to.
type Reader interface {
	// Concept renders one document by identifier. ENOTFOUND when the corpus holds
	// none, which the caller turns into a 404 rather than a 500: a missing page is an
	// answer, not a failure.
	Concept(ctx context.Context, id string) (*Page, error)

	// Search answers at document grain, as the CLI's ladder does.
	Search(ctx context.Context, query string, limit int) ([]Hit, error)
}

// Executor runs a write command.
//
// **The command is a value and the interface takes it whole**, which is §4.6.2's design
// and the reason there is no web-only write path: the same command type the CLI builds
// is the one that crosses this seam, so the gate, the audit row and the git commit are
// the ones that already exist. An interface with a method per operation would have been
// a second place for the rules to live.
type Executor interface {
	Execute(ctx context.Context, cmd Command) (gnosis.Outcome, error)
}

// Command is one mutation, as the web layer can express it.
//
// # It cannot name an approver, and that is §4.6.2.1's rule made structural
//
// "`Approver` is supplied by the transport, never by the payload, and a command carrying
// a caller-set approver is refused. A remote caller sending `human:alice` is otherwise
// unverified, which turns §9.5's no-self-granted-approval rule into an honour system
// precisely when a non-human caller arrives."
//
// So there is no approver field. The shell reads the actor the middleware authenticated
// and fills it in on the far side of this type, and a request body carrying one is not
// refused by a check — it has nowhere to be decoded into. That is the same guarantee
// `CriticClaim` gets from having no warrant field, and it is stronger than a validator,
// which someone can forget to call.
type Command struct {
	// Kind is the operation. A closed set, because a caller that could name an
	// arbitrary one would be choosing which gate runs.
	Kind CommandKind `json:"kind"`

	// Path is the document, bundle-relative.
	Path string `json:"path"`

	// Claim is the claim a decision concerns, for the kinds that take one.
	Claim string `json:"claim,omitempty"`

	// Rationale is why. §10.6.4 requires it on an adjudication and §13 lists it among
	// what the queue must present, because a decision nobody explained is one nobody
	// can review later.
	Rationale string `json:"rationale"`
}

// CommandKind names a mutation.
type CommandKind string

// Valid reports whether a decoded command names a kind this surface offers and carries
// the rationale every decision requires.
//
// Requires: nothing; c may have been decoded from any body.
// Ensures: nil when the command is well formed, and otherwise a map from field name to
// what is wrong with it — the shape rules.md's `Validator` describes, so a handler
// reports every problem at once rather than one per round trip. Pure.
func (c *Command) Valid(_ context.Context) map[string]string {
	problems := map[string]string{}
	switch c.Kind {
	case CommandAdjudicate, CommandPromote:
	case CommandUnset:
		problems["kind"] = "no kind named; adjudicate and promote are the two this" +
			" surface offers"
	default:
		problems["kind"] = string(c.Kind) + " is not a mutation this surface offers"
	}
	if c.Path == "" {
		problems["path"] = "no document named"
	}
	if c.Rationale == "" {
		// Required on both kinds rather than only on adjudication. §9.5's promotion
		// carries one too, and a queue whose two actions had different requirements
		// would teach a reviewer that the field is optional.
		problems["rationale"] = "a decision with no stated reason cannot be reviewed" +
			" later, which is the whole purpose of recording it"
	}
	if len(problems) == 0 {
		return nil
	}
	return problems
}
