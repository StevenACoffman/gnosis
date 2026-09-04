package command

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Defer records that a person saw a contradiction and is not acting on it yet (§17.0).
//
// # Why a deferral is a command and not a note
//
// It mutates the committed tier: the document gains a `gnosis_conflicts` entry, which
// travels to every other user through the same `git pull` that carries the claim.
// §10.7.4's rule is that decisions are committed and observations are cached, and this is
// the one conflict state no rebuild can re-derive — the check re-finds an open conflict
// every run and cannot re-find a decision to live with one.
//
// # Why it is not an adjudication
//
// §10.4 resolves a contradiction and writes a warrant; this resolves nothing. §17.0's
// argument for spending a state on it is that "the common failure of a findings system is
// not that problems go undetected but that detected problems go unanswered, and silence
// is indistinguishable from nobody having looked". A deferral converts that silence into
// a record, and §13 asks for it by name: each queue item "must also be cheap to dismiss".
//
// # What it deliberately does not carry
//
// **No severity and no resolution.** A deferral says nothing about whether either claim
// is right, which is what keeps it from being an adjudication somebody performed without
// a warrant.
type Defer struct {
	// Path is the document the entry lands on, relative to the bundle root.
	Path string

	// Concept is the other document in the contradiction, by identifier — §5.4
	// requires it, because an edge that survives reorganisation is the point.
	Concept gnosis.ID

	// Finding is the contradiction's stable identity, as the conflict check computes
	// it. Required: a deferral that named no finding would silence whichever conflict
	// a later reader guessed it meant.
	Finding string

	// By is who is deferring.
	//
	// **A human, unlike a challenge's actor.** Challenging grants no authority and
	// admits nothing, so §10.7.3 lets an agent file one; deferring *suppresses* a
	// finding from the review queue, and a machine that could decide to live with a
	// contradiction is a machine closing the corpus's own findings.
	By gnosis.Actor

	// Reason is why they are not acting yet. Required (§17.0): a deferral with no
	// reason is a conflict that went quiet.
	Reason string

	// Eff is preview or apply, and preview is the zero value for §4.6.2's reason —
	// the field's default must not be the one that writes.
	Eff Effect
}

// Op names the operation.
func (d *Defer) Op() string { return "defer" }

// Effect reports whether this command writes.
func (d *Defer) Effect() Effect { return d.Eff }

// Validate reports why this command is not executable, or nil.
//
// Requires: nothing; a zero Defer is a valid input and is rejected.
// Ensures: every problem at once, EINVALID, so a caller fixing one is not told about the
// next on the following run.
func (d *Defer) Validate() error {
	const op = "command.Defer.Validate"

	var bad []string
	if strings.TrimSpace(d.Path) == "" {
		bad = append(bad, "path is empty")
	}
	if d.Concept == "" {
		bad = append(bad, "concept is empty; §5.4 names the other document by "+
			"identifier, never by path")
	}
	if strings.TrimSpace(d.Finding) == "" {
		bad = append(bad, "finding is empty; `gnosis lint --check conflict` prints "+
			"the id of each contradiction it reports")
	}
	if !d.Eff.Valid() {
		bad = append(bad, "effect is "+d.Eff.String()+"; set preview or apply")
	}
	switch {
	case d.By == gnosis.ActorUnset:
		bad = append(bad, "by is unset")
	case d.By.Kind() != gnosis.KindHuman:
		bad = append(bad, "by "+string(d.By)+" is not human:<id>; deferring "+
			"suppresses a finding, and a machine deciding to live with a "+
			"contradiction is a machine closing the corpus's own findings")
	}
	if strings.TrimSpace(d.Reason) == "" {
		bad = append(bad, "reason is empty; §17.0 records who saw a finding, when, "+
			"and why they are not acting yet, and a deferral missing the why is a "+
			"conflict that went quiet")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}
