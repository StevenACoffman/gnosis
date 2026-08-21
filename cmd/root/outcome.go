// outcome.go emits the machine-output envelope: the JSONL result shape climax
// generates (`climax init --jsonl`), specialized with gnosis's status and reason
// vocabulary. climax owns the envelope's shape; gnosis owns its words.
//
// The envelope itself lives in internal/gnosis. It moved there when the write
// coordinator acquired a need for it (SPEC §4.6.2's `Execute` returns one), and
// internal packages cannot import cmd — the same promote-on-second-consumer rule
// the family applies to skillet, applied inside one repository. What stays here
// is the part that is actually command infrastructure: writing a line to stdout
// and turning a code into an exit status.
//
// The vocabulary is re-exported rather than referenced through gnosis, so a
// command author has one place to look and the layering stays invisible at the
// call site.
//
// Why gnosis has a status adh does not
//
// adh's three statuses are dispositional: did the work proceed? gnosis is a
// findings tool, and SPEC §17 insists a finding is not a failure. A `lint` that
// returned "error" for discovering a problem would teach callers to ignore the
// field, and one that returned "ok" would leave a CI gate nothing to branch on.
// So there is a fourth status, and the line between it and "blocked" is not
// severity — it is whether the command's own work completed:
//
//   - `gnosis lint` finds a duplicate identifier: the examination finished, and
//     it found something. StatusFindings.
//   - `gnosis promote` hits the same duplicate: the promotion could not proceed
//     and a person must decide. StatusBlocked.

package root

import (
	"encoding/json"
	"fmt"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Status values, re-exported from internal/gnosis.
const (
	StatusOK       = gnosis.StatusOK
	StatusFindings = gnosis.StatusFindings
	StatusBlocked  = gnosis.StatusBlocked
	StatusError    = gnosis.StatusError
)

// Reason tokens, re-exported from internal/gnosis.
const (
	ReasonDuplicateIdentity = gnosis.ReasonDuplicateIdentity
	ReasonIdentityConflict  = gnosis.ReasonIdentityConflict
	ReasonIndexDrift        = gnosis.ReasonIndexDrift
	ReasonUnparsable        = gnosis.ReasonUnparsable
	ReasonVocabularyInvalid = gnosis.ReasonVocabularyInvalid
	ReasonNoBundle          = gnosis.ReasonNoBundle
	ReasonNeedsHuman        = gnosis.ReasonNeedsHuman
	ReasonUsage             = gnosis.ReasonUsage
	ReasonFetchFailed       = gnosis.ReasonFetchFailed
	ReasonStandardsInvalid  = gnosis.ReasonStandardsInvalid
	ReasonWriterBusy        = gnosis.ReasonWriterBusy
	ReasonInvalidCommand    = gnosis.ReasonInvalidCommand
	ReasonGateUnavailable   = gnosis.ReasonGateUnavailable
)

// Exit codes, re-exported from internal/gnosis.
const (
	CodeOK       = gnosis.CodeOK
	CodeError    = gnosis.CodeError
	CodeUsage    = gnosis.CodeUsage
	CodeFindings = gnosis.CodeFindings
	CodeBlocked  = gnosis.CodeBlocked
)

// These are aliases rather than redeclarations so a value crosses the boundary
// unchanged: a command can hand an Outcome to the coordinator's caller and back
// with no conversion, and no second definition can drift from the first.
type (
	// Outcome is one result record.
	Outcome = gnosis.Outcome

	// Status is the class of a result.
	Status = gnosis.Status

	// Code is a process exit status.
	Code = gnosis.Code
)

// EmitJSONL writes v to stdout as a single JSON line.
//
// Requires: v marshals to JSON.
// Ensures: exactly one line is written, terminated by a newline, so a reader can
// consume a stream with a line scanner.
func (c *Config) EmitJSONL(v any) error {
	if err := json.NewEncoder(c.Stdout).Encode(v); err != nil {
		return fmt.Errorf("emit jsonl: %w", err)
	}
	return nil
}

// EmitOutcome writes an already-built envelope.
//
// It exists for a caller that received one from elsewhere — the write coordinator
// returns an Outcome (SPEC §4.6.2) — so a command relaying that result does not
// have to take it apart and put it back together.
func (c *Config) EmitOutcome(o Outcome) error {
	return c.EmitJSONL(o)
}

// EmitOK writes a success outcome carrying data.
func (c *Config) EmitOK(data any) error {
	return c.EmitJSONL(gnosis.OK(data))
}

// EmitFindings writes an outcome for a completed command that reported at least
// one blocking finding.
func (c *Config) EmitFindings(reason, message string, data any) error {
	return c.EmitJSONL(gnosis.Findings(reason, message, data))
}

// EmitBlocked writes an outcome for a command that could not complete because a
// person must act.
func (c *Config) EmitBlocked(reason, message string, data any) error {
	return c.EmitJSONL(gnosis.Blocked(reason, message, data))
}

// EmitError writes an outcome for a tool failure.
func (c *Config) EmitError(reason, message string) error {
	return c.EmitJSONL(gnosis.Failure(reason, message))
}

// EmitUsage writes an outcome for a bad invocation.
func (c *Config) EmitUsage(message string) error {
	return c.EmitJSONL(gnosis.BadUsage(message))
}

// Fail reports a tool failure in whichever output form was requested.
//
// Requires: cause is non-nil.
// Ensures: the result is never nil, so a caller can wrap it unconditionally, and
// carries CodeError in both output forms.
//
// The human path writes the cause to stderr and returns an ExitError rather than
// returning the cause itself. That distinction is not cosmetic: the dispatcher
// prints command usage for any error it does not recognise as an ExitError, and
// usage is the wrong response to "that document does not exist" — it answers a
// question the caller did not ask and buries the sentence that did answer them.
func (c *Config) Fail(reason string, cause error) error {
	if c.JSONL {
		if err := c.EmitError(reason, cause.Error()); err != nil {
			return err
		}
		return ExitError(CodeError)
	}
	_, _ = fmt.Fprintf(c.Stderr, "error: %v\n", cause)
	return ExitError(CodeError)
}

// Usage reports a bad invocation.
//
// Requires: cause explains what was wrong and, where it can, names the valid
// alternatives — a caller that has to read the source to recover has been told
// nothing useful.
// Ensures: the result is never nil. Exit code 2 in both output forms — the
// invocation was rejected either way, and an exit code that depended on the
// output format would be unusable. The message goes to stderr rather than stdout
// even in human form, because stdout carries results and an invocation that was
// rejected produced none.
func (c *Config) Usage(cause error) error {
	if c.JSONL {
		if err := c.EmitUsage(cause.Error()); err != nil {
			return err
		}
		return ExitError(CodeUsage)
	}
	_, _ = fmt.Fprintf(c.Stderr, "error: %v\n", cause)
	return ExitError(CodeUsage)
}
