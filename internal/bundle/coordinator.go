package bundle

import (
	"context"

	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Coordinator is the single writing authority over one bundle (SPEC §4.6).
//
// Within one user there are many clients — a CLI invocation, one or more agents, a
// viewer — and all of them reach the corpus through here. Between users nothing is
// shared but git: every user has their own gitignored `.gnosis/index.db`, so there
// is no distributed database, no locking protocol, and no synchronisation story.
//
// It is a struct with one field rather than a bare function because §4.6 gives
// `gnosis serve` this role alongside the viewer, and a served coordinator will
// hold the lock across many commands rather than per call. The shape anticipates
// that; the current implementation does not need it.
type Coordinator struct {
	// Dir is the bundle root.
	Dir string
}

// Execute runs one command against the bundle.
//
// Requires: cmd is non-nil. Dir names an existing, writable bundle.
// Ensures: returns §8.0's envelope for every outcome a caller should act on —
// including refusal — and an error only when the coordinator itself could not
// function. A rejected command is a result, not a crash: it exits usage, and the
// envelope names which field was wrong.
//
// **Validation happens before the lock is taken**, and the order matters: a
// malformed command must not make a well-formed one wait. It is also the point at
// which "no transport can skip validation" becomes true, since every transport
// arrives here.
//
// A command whose Effect does not write still takes the lock. That looks
// unnecessary and is not: a preview computes the diff the apply will use, and a
// preview racing a concurrent write would report a diff against a bundle that no
// longer exists — which is exactly the window §9.4 closes.
func (c *Coordinator) Execute(ctx context.Context, cmd command.Command) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.Execute"

	if err := cmd.Validate(); err != nil {
		return gnosis.BadUsage(err.Error()), nil
	}

	lock, err := AcquireWriterLock(ctx, c.Dir)
	if err != nil {
		if WriterBusy(err) {
			return writerBusyOutcome(c.Dir), nil
		}
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	defer lock.Release()

	return c.dispatch(ctx, cmd)
}

// dispatch routes a validated command to its handler.
//
// It is separate from Execute so that the preconditions every command shares —
// validated, lock held — are established in one place and cannot be skipped by a
// handler added later.
func (c *Coordinator) dispatch(ctx context.Context, cmd command.Command) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.dispatch"

	switch v := cmd.(type) {
	case *command.Promote:
		return c.promote(ctx, v)
	default:
		// Not a usage error: the caller constructed something this build does not
		// implement, which is a fault in the pairing of caller and binary.
		return gnosis.Failure(gnosis.ReasonInvalidCommand,
			op+": no handler for "+cmd.Op()), nil
	}
}
