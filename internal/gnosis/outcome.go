package gnosis

// Status values: the class of a result, and the field an agent switches on under
// --jsonl (SPEC §8.0).
//
// StatusUnset is the zero value and is not a status. It exists because the
// alternative — a zero value meaning StatusOK — would have an unpopulated
// envelope report success, which is the one mistake this type must not make.
const (
	StatusUnset    Status = ""
	StatusOK       Status = "ok"       // completed; nothing blocking to report
	StatusFindings Status = "findings" // completed, and reported a blocking finding — not a failure
	StatusBlocked  Status = "blocked"  // could not complete; a person must act
	StatusError    Status = "error"    // the tool itself failed
)

// Reason tokens: a stable machine string for a non-ok outcome, so an agent
// branches on the token rather than matching prose. Every token here is paired
// with a human-readable message; neither substitutes for the other.
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
	ReasonWriterBusy        = "writer_busy"        // another writer holds the bundle
	ReasonInvalidCommand    = "invalid_command"    // the command was not constructible as asked
	ReasonGateUnavailable   = "gate_unavailable"   // the check that must approve this write is not built

	// ReasonRefused is a check that ran and failed. There is no route through it:
	// no actor, no phrase, and no rationale changes the answer (§9.5.1).
	//
	// It is distinct from ReasonNeedsHuman and the distinction is load-bearing.
	// §9.5.1's policy is that "the human path opens for what could not be checked
	// and stays shut for what was checked and failed", and a *refusal* reported as
	// needs_human made that policy invisible in the one place a caller reads: the
	// CLI prompted for a confirmation it would then decline, and a machine caller
	// branching on the token could not tell a document to fix from a signature to
	// give. Found by running the command against a candidate carrying an injected
	// directive — the refusal was correct and the reason it gave was not.
	ReasonRefused = "refused" // a signal ran and failed; there is no route through
)

// Exit codes. Findings and errors are deliberately distinct: a CI job needs to
// tell "the corpus has problems" from "gnosis broke", and collapsing them makes a
// broken tool look like a dirty corpus.
//
// There is no CodeUnset, and that is not an oversight. Zero is a real code and
// means success, so a Code carries no safe zero value — which is why an Outcome
// is only constructible through the functions below, each of which sets the
// status and the code together. Nothing in this package produces a zero Outcome,
// and Valid reports one built by hand.
const (
	CodeOK       Code = 0
	CodeError    Code = 1
	CodeUsage    Code = 2
	CodeFindings Code = 3
	CodeBlocked  Code = 4
)

// Status is the class of a result.
type Status string

// Code is a process exit status.
//
// It is a distinct type from Status so that the two cannot be transposed. They
// are not in one-to-one correspondence — a rejected invocation is StatusError
// with CodeUsage, because the invocation failed either way and what differs is
// the repair — so a single field could not carry both.
type Code int

// Outcome is one result record (SPEC §8.0). The shape is climax's, shared with
// adh, so an agent that can read one family tool's output can read another's.
//
// Construct one with OK, Findings, Blocked, Failure, or BadUsage. Those are the
// only pairings of status and code this specification defines, so making them the
// only way to build an Outcome removes the possibility of a mismatched pair
// rather than checking for one.
type Outcome struct {
	Status  Status `json:"status"`
	Code    Code   `json:"code"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// OK reports a completed operation with nothing blocking.
func OK(data any) Outcome {
	return Outcome{Status: StatusOK, Code: CodeOK, Data: data}
}

// Findings reports a completed operation that found at least one blocking
// problem.
//
// The payload is carried rather than fetched separately: a caller acting on
// findings needs them, and a second round trip would invite acting on a stale
// read.
func Findings(reason, message string, data any) Outcome {
	return Outcome{
		Status: StatusFindings, Code: CodeFindings,
		Reason: reason, Message: message, Data: data,
	}
}

// Blocked reports an operation that could not complete because a person must act.
//
// The line between this and Findings is not severity — it is whether the
// operation's own work completed. `lint` finding a duplicate identifier finished
// its examination and found something; `promote` hitting the same duplicate could
// not proceed.
func Blocked(reason, message string, data any) Outcome {
	return Outcome{
		Status: StatusBlocked, Code: CodeBlocked,
		Reason: reason, Message: message, Data: data,
	}
}

// Failure reports that the tool itself failed.
func Failure(reason, message string) Outcome {
	return Outcome{Status: StatusError, Code: CodeError, Reason: reason, Message: message}
}

// BadUsage reports a rejected invocation.
//
// The status is StatusError rather than a fifth value: calling gnosis wrongly is a
// tool-level failure and the reason token already says which kind. What
// distinguishes it is the exit code, because the repair differs — code 2 means
// "call me differently" and code 1 means "changing the arguments will not help".
func BadUsage(message string) Outcome {
	return Outcome{
		Status: StatusError, Code: CodeUsage, Reason: ReasonUsage, Message: message,
	}
}

// Valid reports whether an Outcome carries a status and code this specification
// pairs.
//
// Requires: nothing.
// Ensures: false for the zero Outcome, so an envelope nobody populated does not
// read as success. Intended for a test or a transport boundary; a value from the
// constructors above always satisfies it.
func (o Outcome) Valid() bool {
	switch o.Status {
	case StatusOK:
		return o.Code == CodeOK
	case StatusFindings:
		return o.Code == CodeFindings
	case StatusBlocked:
		return o.Code == CodeBlocked
	case StatusError:
		return o.Code == CodeError || o.Code == CodeUsage
	default:
		return false
	}
}
