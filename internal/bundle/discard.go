package bundle

import (
	"context"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// discard drops a quarantined draft and records that it was dropped.
//
// Requires: cmd has been validated; w still holds the lock.
// Ensures: under a preview nothing is removed and the outcome says what would be;
// under an apply the draft is gone and the trail says who dropped it and why.
//
// The draft is read before it is removed, and not because anything needs the
// content: the read is what distinguishes "there was nothing there" from "it is
// gone now". Discarding something absent must not report a successful discard, or a
// mistyped path reads as a completed cleanup and the real draft stays in the queue.
func (c *Coordinator) discard(
	_ context.Context, w *Writer, cmd *command.Discard,
) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.discard"

	before, err := ReadQuarantined(c.Dir, cmd.Path)
	if err != nil {
		if errs.ErrorCode(err) == errs.ENOTFOUND {
			return gnosis.Blocked(gnosis.ReasonNeedsHuman,
				"nothing is quarantined at "+cmd.Path, map[string]any{
					"path": cmd.Path, "effect": cmd.Eff.String(),
				}), nil
		}
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}

	data := map[string]any{
		"path": cmd.Path, "effect": cmd.Eff.String(),
		"bytes": len(before), "by": string(cmd.By), "reason": cmd.Reason,
	}
	if !cmd.Eff.Writes() {
		// A preview writes no audit row, as a promote preview does not: a preview is
		// a read, and a mutation log that also holds reads is a log somebody stops
		// reading. It matters more here than there — the content is about to be
		// unrecoverable, tier 1 is not committed, and a caller checking what they are
		// about to lose should not have to pay for the check in the trail.
		data["discarded"] = false
		data["would_discard"] = true
		return gnosis.OK(data), nil
	}

	// The committed record comes first, and the order is the safe one. A crash
	// between the two leaves a log entry saying the draft was declined and the draft
	// still in quarantine — visible in `gnosis quarantine`, and recoverable by
	// discarding it again. The reverse leaves the draft gone and no record of the
	// decision, which is the state §10.7.4 is about and cannot be recovered from.
	if lErr := c.logDecline(w, cmd); lErr != nil {
		return gnosis.Outcome{}, lErr
	}
	if err = w.Discard(cmd.Path); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	data["discarded"] = true
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpDiscard, Actor: string(cmd.By),
		Paths: []string{cmd.Path},
		// The hash of what was dropped, in HashBefore with nothing after it. That
		// is the same shape a deletion takes everywhere else here, and it is the
		// one piece of evidence that survives: a colleague asking what was in the
		// draft can at least be told whether it was the same bytes as another.
		HashBefore: hashOrEmpty(before),
		Outcome:    string(gnosis.StatusOK),
		Detail:     "discarded from quarantine: " + cmd.Reason,
	})
	return gnosis.OK(data), nil
}

// logDecline records a person's decision to decline a draft in the committed tier.
//
// Requires: cmd is an applying discard; w holds the lock.
// Ensures: nothing for an agent's discard; otherwise one line in log.md, and an error
// only when the write failed.
//
// # Why this is committed when the gate's own refusals are not
//
// The backlog entry behind this asked whether a declined promotion belongs in the
// committed tier, since §10.7.4 makes a decision to decline a decision. It is three
// events wearing one word, and separating them answers it.
//
//   - **The gate refused.** Mechanical and *recomputable*: run the gate again and get
//     the same answer. Committing it would put a derived fact in the authoritative
//     tier, which §12 already refuses for the index. It stays in the per-user trail.
//   - **A person was asked and walked away.** Not recomputable, and not a decision —
//     nobody decided anything. `gnosis audit --outstanding` is what surfaces it.
//   - **A person looked at the draft and dropped it.** Not recomputable from
//     anything: the draft is gone and the reason existed only in their head until
//     they typed it. That is a decision, and this is where it goes.
//
// # Why an agent's discard is not logged
//
// `Discard.By` may be an agent, deliberately: dropping a draft grants no authority.
// An agent clearing a reply its own gate refused is housekeeping, and committing every
// one of those would fill the corpus's history with the noise that teaches a reader to
// skip it — the same argument §12 makes about a warning true of everything. §10.7.4 is
// about decisions, and a decision is a person's. The agent's discard is still in the
// per-user trail, with its reason.
//
// It is not best-effort. Everywhere else in this package a failed *annotation* — an
// audit row, a prompt removal — is a warning rather than the operation's failure,
// because the operation succeeded and telling a caller to retry it would be wrong.
// Here the log entry is not an annotation: it is the only durable record of the
// decision, and it is written before the draft is removed precisely so that failing
// leaves both intact.
func (c *Coordinator) logDecline(w *Writer, cmd *command.Discard) error {
	if !cmd.By.IsHuman() {
		return nil
	}
	// A bullet, because OKF §9's entry form is a list and `Loosened.LogEntry` — the
	// only other writer of this file — already writes one. A bare line renders as a
	// paragraph and reads as prose somebody typed rather than as an entry.
	note := "- Declined `" + cmd.Path + "` (" + string(cmd.By) + "): " + cmd.Reason
	if err := w.Log(c.now(), note); err != nil {
		return &errs.Error{Op: "bundle.Coordinator.logDecline", Err: err}
	}
	return nil
}
