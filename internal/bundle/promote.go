package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// promote runs the gate over a quarantined document and, on approval, writes it.
//
// Requires: the writer lock is held; cmd has been validated.
// Ensures: the bytes the gate judged are the bytes written. There is exactly one
// read of the candidate, and the gate's Candidate carries it — a re-read between
// the verdict and the write is a defect, not an optimisation (§9.4), because it
// reopens the window the gate exists to close.
//
// Preview and apply are the same code path down to the final branch. That is not
// tidiness: it is what makes the diff guarantee a property of the data model
// rather than a claim that two functions agree.
func (c *Coordinator) promote(_ context.Context, cmd *command.Promote) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.promote"

	after, err := ReadQuarantined(c.Dir, cmd.Path)
	if err != nil {
		if errs.ErrorCode(err) == errs.ENOTFOUND {
			return gnosis.Blocked(gnosis.ReasonNeedsHuman,
				"nothing is quarantined at "+cmd.Path, nil), nil
		}
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	before, err := readIfPresent(op, filepath.Join(c.Dir, filepath.FromSlash(cmd.Path)))
	if err != nil {
		return gnosis.Outcome{}, err
	}

	candidate := c.candidate(cmd.Path, before, after)
	corpus, limits, err := c.gateInputs()
	if err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}

	report := gate.Evaluate(candidate, corpus, limits)
	if !report.Approved() {
		outcome := withheld(&report, cmd)
		// A refusal is recorded too. "We declined to promote this eleven times" is
		// a fact about the corpus that a successful-writes-only trail would not
		// hold, and it is the fact most worth having when somebody asks why a
		// document never landed.
		c.record(&audit.Row{
			At: c.now(), Op: audit.OpPromote, Actor: string(cmd.Approver),
			Paths: []string{cmd.Path}, Outcome: string(outcome.Status),
			Detail: outcome.Message,
		})
		return outcome, nil
	}
	if !cmd.Eff.Writes() {
		return gnosis.OK(map[string]any{
			"path": cmd.Path, "effect": cmd.Eff.String(),
			"approved": true, "report": report,
		}), nil
	}
	return c.apply(op, cmd, candidate, &report)
}

// apply writes the approved bytes and clears the draft.
//
// The order matters. The document lands first and the quarantine copy is removed
// second, so an interruption between them leaves a promoted document and a stale
// draft — visible, and harmless to re-promote. The reverse would lose the content.
func (c *Coordinator) apply(
	op string, cmd *command.Promote, candidate *gate.Candidate, report *gate.Report,
) (gnosis.Outcome, error) {
	full := filepath.Join(c.Dir, filepath.FromSlash(cmd.Path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	// candidate.After, not a re-read of the quarantine file: these are the exact
	// bytes the gate approved.
	if err := atomicfile.WriteFile(full, candidate.After, 0o640); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	if err := Discard(c.Dir, cmd.Path); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	c.record(&audit.Row{
		At: c.now(), Op: audit.OpPromote, Actor: string(cmd.Approver),
		Paths:      []string{cmd.Path},
		HashBefore: hashOrEmpty(candidate.Before),
		HashAfter:  hashOrEmpty(candidate.After),
		Outcome:    string(gnosis.StatusOK),
		Detail:     "promoted from quarantine",
	})
	return gnosis.OK(map[string]any{
		"path": cmd.Path, "effect": cmd.Eff.String(),
		"approved": true, "wrote": true,
		"approver": string(cmd.Approver), "report": report,
	}), nil
}

// withheld renders a refusal, distinguishing what failed from what could not run.
//
// The two are separated because they call for opposite responses: a failure is
// something the author fixes, and an unchecked signal is something this build
// cannot do. A caller told only "blocked" would go looking for a defect in a
// document that may not have one.
func withheld(report *gate.Report, cmd *command.Promote) gnosis.Outcome {
	failed, unchecked := report.Withheld()

	reason := gnosis.ReasonNeedsHuman
	message := "the promote gate withheld approval"
	switch {
	case !report.Control.Held:
		// The gate could not prove it discriminates, so no verdict below it means
		// anything. This is a tool failure and not the author's problem.
		return gnosis.Failure(gnosis.ReasonGateUnavailable,
			"the gate's own control failed; it cannot be trusted to gate")
	case len(failed) == 0:
		reason = gnosis.ReasonGateUnavailable
		message = "every implemented signal passed, but signals that cannot run withhold approval"
	}
	return gnosis.Blocked(reason, message, map[string]any{
		"path": cmd.Path, "effect": cmd.Eff.String(),
		"approved": false, "failed": failed, "unchecked": unchecked,
		"report": report,
	})
}

// hashOrEmpty is the content hash of some bytes, or "" for none. An empty
// HashBefore is how a creation is told from a revision, so nil must not hash.
func hashOrEmpty(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// readIfPresent reads a file, reporting nil rather than an error when it is
// absent. A promotion of new knowledge and a revision of existing knowledge are
// different events, and this is what tells them apart.
func readIfPresent(op, full string) ([]byte, error) {
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	return data, nil
}
