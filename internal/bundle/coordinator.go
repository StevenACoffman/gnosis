package bundle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

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

	// Warn receives a line about anything that went wrong beside the operation
	// rather than in it — an audit row that could not be written, so far. A nil
	// Warn discards, which is right for a library caller and wrong for a command:
	// `cmd` supplies its own stderr, because a note that exists only in a JSON
	// field is a note nobody running the tool in a terminal will see.
	Warn io.Writer

	// Now is the clock the audit trail stamps rows with. A nil Now uses
	// time.Now, so a caller that does not care need not supply one.
	//
	// It is a field rather than a package-level call because an audit row's whole
	// value is the time on it, and a value the tests cannot pin is a value the
	// tests do not check.
	Now func() time.Time

	// auditErr holds the last audit-append failure, if any. It is surfaced on the
	// outcome rather than returned, because the write it describes already
	// happened and reporting it as the operation's error would tell a caller to
	// retry something that succeeded.
	auditErr error

	// auditUnread holds a failure to read back a row the append reported writing.
	//
	// It is a second field rather than a reuse of auditErr because the two events
	// call for opposite handling, and collapsing them would force one policy on
	// both. §15 and Audit's own doc comment appear to contradict each other —
	// "returns an error rather than an Outcome" against "a failure here does not
	// fail the write it describes" — and they do not, because they are about
	// different events:
	//
	//   - The append returned an error. We *know* the record failed; nothing is
	//     hidden, and the loud fail-soft above is right. §15's "fail-soft would
	//     reproduce the failure" is about a *silent* fail-soft, which this is not.
	//   - The append returned success and the row is not there. The trail is
	//     lying, this is the only place that can be noticed, and it is exactly the
	//     observed failure §15 cites. Fail-soft here really would reproduce it.
	auditUnread error
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

	outcome, err := c.dispatch(ctx, cmd)

	// The unread-row failure joins the returned error rather than the outcome. A
	// caller must not be able to read this as success, and the outcome is still
	// populated so a caller that logs both can say what the corpus now holds.
	return c.withAuditNote(outcome), errors.Join(err, c.auditUnread)
}

// withAuditNote reports an audit-append failure without failing the operation.
//
// The status does not change. The operation did what it reported; what failed is
// the *record* of it, and a caller that treated a missing audit row as a failed
// write would undo work that succeeded.
//
// **But a swallowed error has to be swallowed loudly, and once was not enough.**
// The previous version appended a sentence to Message, which no machine reads and
// which a caller rendering only Data never shows. A trail with silent holes cannot
// answer the question a trail exists for, so the failure now lands in three places
// with different readers: `audit_failed` in Data for an agent branching on it, the
// message for a person reading the envelope, and Warn for whoever is watching the
// terminal. None of them is a status, because the write happened.
func (c *Coordinator) withAuditNote(outcome gnosis.Outcome) gnosis.Outcome {
	if c.auditErr == nil {
		return outcome
	}
	note := "the operation completed but its audit row was not written: " + c.auditErr.Error()

	if data, ok := outcome.Data.(map[string]any); ok {
		data["audit_failed"] = c.auditErr.Error()
	}
	if outcome.Message == "" {
		outcome.Message = note
	} else {
		outcome.Message += "; " + note
	}
	if c.Warn != nil {
		_, _ = fmt.Fprintf(c.Warn, "warning: %s\n", note)
	}
	return outcome
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
	case *command.Admit:
		return c.admit(ctx, v)
	default:
		// Not a usage error: the caller constructed something this build does not
		// implement, which is a fault in the pairing of caller and binary.
		return gnosis.Failure(gnosis.ReasonInvalidCommand,
			op+": no handler for "+cmd.Op()), nil
	}
}
