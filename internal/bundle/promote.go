package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// promote runs the gate over a quarantined document and, on approval, writes it.
//
// Requires: cmd has been validated; w still holds the lock.
// Ensures: the bytes the gate judged are the bytes written. There is exactly one
// read of the candidate, and the gate's Candidate carries it — a re-read between
// the verdict and the write is a defect, not an optimisation (§9.4), because it
// reopens the window the gate exists to close.
//
// Preview and apply are the same code path down to the final branch. That is not
// tidiness: it is what makes the diff guarantee a property of the data model
// rather than a claim that two functions agree.
func (c *Coordinator) promote(
	_ context.Context, w *Writer, cmd *command.Promote,
) (gnosis.Outcome, error) {
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

	// The inputs come first because the candidate's §9.3 scan needs stage 4's
	// bounds, which are part of them. The order was the other way round until stage
	// 4 existed.
	corpus, limits, err := c.gateInputs()
	if err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	candidate := c.candidate(cmd.Path, before, after, limits)

	report := gate.Evaluate(candidate, corpus, limits)

	// A preview asks what would happen and is not an attempt to write, so it is
	// answered before authorisation is examined. Running the checks anyway told
	// somebody who supplied no approver that their promotion "cannot be
	// self-granted by an agent" — an accusation about an action they had not
	// taken. Found by running the command rather than by a test, which is the
	// third time that has been the case.
	if !cmd.Eff.Writes() {
		return preview(&report, cmd), nil
	}

	carried, refusal, mayWrite := authorise(&report, cmd)
	if !mayWrite {
		return c.refuse(w, &report, cmd, refusal), nil
	}
	if why := c.reused(&report, cmd); why != "" {
		return c.refuse(w, &report, cmd, rationaleRefused(&report, cmd, why)), nil
	}

	return c.apply(op, w, cmd, candidate, &report, carried)
}

// reused reports why this promotion's rationale may not be accepted, or "".
//
// Requires: w holds the lock, so the trail cannot grow between this read and the
// write that follows; authorise has already allowed the write.
// Ensures: "" for every promotion the gate approved on its own, whatever rationale
// was supplied.
//
// **Only on the human path.** A rationale nobody required is not a gating field, and
// refusing a promotion the gate approved because of one would be inventing a
// requirement §9.5 does not state.
//
// **After authorise, not before.** `authorisedBy` names every unmet requirement at
// once so one refusal tells a caller everything to fix; running this first would tell
// somebody their rationale repeats the template while they had also forgotten the
// approver — answering the smaller question and hiding the larger one.
func (c *Coordinator) reused(report *gate.Report, cmd *command.Promote) string {
	if report.Decide() != gate.DecisionNeedsHuman {
		return ""
	}
	return reusedRationale(c.Dir, cmd.Path, cmd.Rationale, askedFor())
}

// preview answers "what would happen", without judging an authorisation nobody
// offered.
//
// Requires: cmd.Eff does not write.
// Ensures: never writes and never records an audit row — a preview is a read, and
// a mutation log that also holds reads is a log somebody stops reading. The
// outcome mirrors what an apply would decide, so a preview reporting ok is a
// promise the write would succeed, which is the property §9.4 is about.
//
// For a needs_human candidate it names the requirements rather than reporting them
// as unmet. The distinction is small and it is the difference between a tool that
// tells you what is needed and one that tells you what you did wrong.
func preview(report *gate.Report, cmd *command.Promote) gnosis.Outcome {
	decision := report.Decide()
	failed, unchecked := report.Withheld()
	data := map[string]any{
		"path": cmd.Path, "effect": cmd.Eff.String(), "decision": decision,
		"failed": failed, "unchecked": unchecked, "report": report,
	}

	switch decision {
	case gate.DecisionApproved:
		data["approved"] = true
		return gnosis.OK(data)
	case gate.DecisionNeedsHuman:
		data["approved"] = false
		data["requires"] = []string{
			willNeedApprover, willNeedRationale, willNeedConfirm,
		}
		return gnosis.Blocked(gnosis.ReasonNeedsHuman,
			"every implemented signal passed; applying this would need a person to "+
				"carry the signals that could not run", data)
	default:
		data["approved"] = false
		return withheld(report, cmd)
	}
}

// authorise applies §9.5's policy to a gate report.
//
// Requires: report is the gate's answer for cmd's candidate.
// Ensures: pure. mayWrite is true only when the promotion may proceed; carried
// names the unrun signals a person is taking responsibility for, and is empty for
// a promotion the gate approved on its own. refusal is meaningful only when
// mayWrite is false.
//
// **The human path opens for what could not be checked and stays shut for what was
// checked and failed.** That sentence is the policy and this function is the only
// place it is enforced. A `refused` candidate has no route through here at any
// actor, with any phrase, carrying any rationale — there is no confirmation that
// makes a fabricated quotation acceptable, and providing one would make this the
// `--yes` bypass §15 forbids rather than the escalation §9.5 requires.
func authorise(report *gate.Report, cmd *command.Promote) (
	carried []string, refusal gnosis.Outcome, mayWrite bool,
) {
	switch report.Decide() {
	case gate.DecisionApproved:
		return nil, gnosis.Outcome{}, true
	case gate.DecisionNeedsHuman:
		if why := authorisedBy(cmd); why != "" {
			return nil, needsHuman(report, cmd, why), false
		}
		return unrunSignals(report), gnosis.Outcome{}, true
	case gate.DecisionRefused, gate.DecisionUnavailable:
		return nil, withheld(report, cmd), false
	default:
		// An unrecognised decision authorises nothing. A member added to the
		// enumeration without a branch here refuses rather than falling through.
		return nil, withheld(report, cmd), false
	}
}

// refuse records a withheld promotion and returns the outcome.
//
// A refusal is recorded too. "We declined to promote this eleven times" is a fact
// about the corpus that a successful-writes-only trail would not hold, and it is
// the fact most worth having when somebody asks why a document never landed.
func (c *Coordinator) refuse(
	w *Writer, report *gate.Report, cmd *command.Promote, outcome gnosis.Outcome,
) gnosis.Outcome {
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpPromote, Actor: string(cmd.Approver),
		Paths: []string{cmd.Path}, Outcome: string(outcome.Status),
		Detail: outcome.Message, Signals: unrunSignals(report),
		// Recorded on a refusal, and deliberately *not* read back as a prior
		// rationale: a withheld promotion adjudicated nothing. It is here because
		// "we declined to promote this eleven times, and here is what was offered
		// each time" is a fact about the corpus worth holding — see
		// priorRationales for why reading it back was wrong.
		Rationale: cmd.Rationale,
	})
	return outcome
}

// apply writes the approved bytes and clears the draft.
//
// The order matters. The document lands first and the quarantine copy is removed
// second, so an interruption between them leaves a promoted document and a stale
// draft — visible, and harmless to re-promote. The reverse would lose the content.
func (c *Coordinator) apply(
	op string, w *Writer, cmd *command.Promote, candidate *gate.Candidate,
	report *gate.Report, carried []string,
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
	if err := w.Discard(cmd.Path); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	detail := "promoted from quarantine"
	if len(carried) > 0 {
		// The debt, written where it can be found. When §10 lands, every document
		// admitted without a conflict check is one query away.
		detail = "promoted from quarantine over unrun signals (" +
			strings.Join(carried, ", ") + "): " + cmd.Rationale
	}
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpPromote, Actor: string(cmd.Approver),
		Paths:      []string{cmd.Path},
		HashBefore: hashOrEmpty(candidate.Before),
		HashAfter:  hashOrEmpty(candidate.After),
		Outcome:    string(gnosis.StatusOK),
		Detail:     detail,
		Signals:    carried,
		// Also in Detail, deliberately. Detail is the sentence a person reading the
		// trail directly gets, and dropping the reason out of it to avoid saying it
		// twice would make the readable half the useless half.
		Rationale: cmd.Rationale,
	})
	return gnosis.OK(map[string]any{
		"path": cmd.Path, "effect": cmd.Eff.String(),
		"approved": true, "wrote": true, "carried": carried,
		"approver": string(cmd.Approver), "report": report,
	}), nil
}

// needsHuman renders the escalation: what the gate could not check, and what a
// person must supply to carry it anyway.
//
// Distinct from withheld because the two say opposite things about the document.
// A withheld promotion has something wrong with it. This one may have nothing
// wrong with it at all — the gate simply cannot say, and the message must not send
// an author hunting for a defect that is not there.
func needsHuman(report *gate.Report, cmd *command.Promote, why string) gnosis.Outcome {
	_, unchecked := report.Withheld()
	return gnosis.Blocked(gnosis.ReasonNeedsHuman,
		"every implemented signal passed; a person must carry the signals that "+
			"could not run — "+why,
		map[string]any{
			"path": cmd.Path, "effect": cmd.Eff.String(),
			"decision": gate.DecisionNeedsHuman, "approved": false,
			"unchecked": unchecked, "report": report,
		})
}

// rationaleRefused renders a promotion that was authorised and whose reason was not
// accepted.
//
// It does not reuse `needsHuman`'s sentence, and running the command is what showed
// why: that sentence says "a person must carry the signals that could not run", which
// is a requirement this caller has already met. Told that, they would go looking for a
// missing approver or an untyped path. The distinction is small and it is the whole
// difference between a refusal somebody can act on and one they have to decode.
func rationaleRefused(
	report *gate.Report, cmd *command.Promote, why string,
) gnosis.Outcome {
	_, unchecked := report.Withheld()
	return gnosis.Blocked(gnosis.ReasonNeedsHuman,
		"this promotion is authorised and its rationale was not accepted: "+why,
		map[string]any{
			"path": cmd.Path, "effect": cmd.Eff.String(),
			"decision": gate.DecisionNeedsHuman, "approved": false,
			"unchecked": unchecked, "report": report,
		})
}

// withheld renders a refusal, distinguishing what failed from what could not run.
//
// The two are separated because they call for opposite responses: a failure is
// something the author fixes, and an unchecked signal is something this build
// cannot do. A caller told only "blocked" would go looking for a defect in a
// document that may not have one.
func withheld(report *gate.Report, cmd *command.Promote) gnosis.Outcome {
	failed, unchecked := report.Withheld()

	// A refusal and an escalation must not share a reason token. §9.5.1's policy is
	// that the human path opens for what could not be checked and stays shut for
	// what was checked and failed — and while `authorise` enforces that, reporting
	// both as needs_human made it invisible where a caller reads: the CLI prompted
	// for a confirmation it would decline, which teaches somebody that typing the
	// path is what unlocks a refusal. Found by running the command.
	reason := gnosis.ReasonRefused
	message := "the promote gate refused this document; " +
		"a signal ran and failed, and no confirmation changes that. " +
		"Fix the input and re-admit, then drop this draft with " +
		"`gnosis quarantine --discard`"
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
