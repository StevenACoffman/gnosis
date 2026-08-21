// outcome.go is the machine-output envelope: the JSONL result shape climax
// generates (`climax init --jsonl`), specialized with gnosis's status and reason
// vocabulary. climax owns the envelope's shape; gnosis owns its words.
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
)

// Status values: the class of a command's result, and the field an agent
// switches on under --jsonl.
const (
	StatusOK       = "ok"       // completed; nothing blocking to report
	StatusFindings = "findings" // completed, and reported a blocking finding — not a failure
	StatusBlocked  = "blocked"  // could not complete; a person must act
	StatusError    = "error"    // the tool itself failed
)

// Reason tokens: a stable machine string for a non-ok outcome, so an agent
// branches on the token rather than matching prose. Every token here is paired
// with a human-readable Message; neither substitutes for the other.
const (
	ReasonDuplicateIdentity = "duplicate_identity" // one identifier, several documents
	ReasonIdentityConflict  = "identity_conflict"  // document and index disagree at a path
	ReasonIndexDrift        = "index_drift"        // the index no longer matches the bundle
	ReasonUnparsable        = "unparsable"         // a document could not be read as OKF
	ReasonVocabularyInvalid = "vocabulary_invalid" // ontology.toml was rejected
	ReasonNoBundle          = "no_bundle"          // no bundle at the given path
	ReasonNeedsHuman        = "needs_human"        // a decision is required to proceed
	ReasonUsage             = "usage"              // bad flags or arguments
	ReasonFetchFailed       = "fetch_failed"       // a source could not be read
	ReasonStandardsInvalid  = "standards_invalid"  // standards/archive.toml was rejected
)

// Exit codes. Findings and errors are deliberately distinct: a CI job needs to
// tell "the corpus has problems" from "gnosis broke", and collapsing them makes
// a broken tool look like a dirty corpus.
const (
	CodeOK       = 0
	CodeError    = 1
	CodeUsage    = 2
	CodeFindings = 3
	CodeBlocked  = 4
)

// Outcome is one result record. The shape is climax's, shared with adh, so an
// agent that can read one family tool's output can read another's.
type Outcome struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

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

// EmitOK writes a success outcome carrying data.
func (c *Config) EmitOK(data any) error {
	return c.EmitJSONL(Outcome{Status: StatusOK, Code: CodeOK, Data: data})
}

// EmitFindings writes an outcome for a completed command that reported at least
// one blocking finding. The payload is included: a caller acting on findings
// needs them, and making it fetch them separately would invite acting on a
// stale read.
func (c *Config) EmitFindings(reason, message string, data any) error {
	return c.EmitJSONL(Outcome{
		Status: StatusFindings, Code: CodeFindings,
		Reason: reason, Message: message, Data: data,
	})
}

// EmitBlocked writes an outcome for a command that could not complete because a
// person must act.
func (c *Config) EmitBlocked(reason, message string, data any) error {
	return c.EmitJSONL(Outcome{
		Status: StatusBlocked, Code: CodeBlocked,
		Reason: reason, Message: message, Data: data,
	})
}

// EmitError writes an outcome for a tool failure.
func (c *Config) EmitError(reason, message string) error {
	return c.EmitJSONL(Outcome{
		Status: StatusError, Code: CodeError, Reason: reason, Message: message,
	})
}

// EmitUsage writes an outcome for a bad invocation.
//
// The status is StatusError rather than a fifth value: calling gnosis wrongly is
// a tool-level failure, and the reason token already says which kind. What
// distinguishes it is the exit code, because the repair differs — code 2 means
// "call me differently" and code 1 means "changing the arguments will not help".
func (c *Config) EmitUsage(message string) error {
	return c.EmitJSONL(Outcome{
		Status: StatusError, Code: CodeUsage, Reason: ReasonUsage, Message: message,
	})
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
// output format would be unusable. The message goes to stderr rather
// than stdout even in human form, because stdout carries results and an
// invocation that was rejected produced none.
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
