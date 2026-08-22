// Package audit is the write trail: one row per mutation (SPEC §15).
//
// It is a **tracer** in §14.3.1's sense — instrumentation added so that something
// which otherwise leaves no trace begins leaving one. A write is a non-event from
// the outside: the file is simply different afterwards, and nothing records who
// caused that or on what basis. `clu` records every write and can answer who did
// what when; so must this.
//
// # The timestamp, and why tier 0 has none
//
// A fetch record carries no timestamp (§4.3.1) and an audit row carries one. That
// looks inconsistent and is the opposite. A fetch record is content-addressed, so
// a timestamp would make tier 0 grow when somebody *checks* rather than when the
// corpus *learns* — some 26,000 near-identical records a year for a weekly sweep.
// An audit row is a record of an event, and "when" is half the question it exists
// to answer.
//
// §10.7.4 is the rule that reconciles them: **decisions are committed,
// observations are cached.** A fetch record states a fact about the corpus and has
// to travel; an audit row states what this user's process did and must not. That
// is why the trail is per-user and gitignored, and why two colleagues at one commit
// have different audit files and are both right.
//
// Everything here is pure. The caller supplies the time.
package audit

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/StevenACoffman/skillet/errs"
)

// The operations a row can record.
//
// OpUnset is the zero value and names no operation. A row nobody populated must
// not read as a record of something — an audit trail whose empty rows look like
// events is worse than none, because it invites counting them.
const (
	OpUnset   Op = ""
	OpFetch   Op = "fetch"   // a source entered tier 0
	OpAdmit   Op = "admit"   // a reply became a quarantined document
	OpPromote Op = "promote" // a document entered the corpus
	OpInit    Op = "init"    // a bundle was scaffolded
	OpRebuild Op = "rebuild" // the derived index was rewritten
	OpDiscard Op = "discard" // a quarantined draft was dropped
)

// Op names what happened.
type Op string

// Row is one mutation.
//
// Field order fixes the encoding, as it does for a fetch record. Unlike a fetch
// record the encoding is not an identity — nothing is addressed by it — so the
// ordering is for a reader diffing two trails rather than for correctness.
type Row struct {
	// At is when. RFC 3339 with nanoseconds, in UTC: a trail compared across two
	// machines in different zones would otherwise interleave wrongly.
	At time.Time `json:"at"`

	// Op is what happened.
	Op Op `json:"op"`

	// Actor is who caused it. Required — a write nobody can be asked about is the
	// case this trail exists to prevent.
	Actor string `json:"actor"`

	// Paths are what changed, relative to the bundle root.
	Paths []string `json:"paths,omitempty"`

	// HashBefore and HashAfter are the content hashes of the primary path.
	// Before is empty for a creation, which is how the two are told apart.
	HashBefore string `json:"hash_before,omitempty"`
	HashAfter  string `json:"hash_after,omitempty"`

	// StandardsHash identifies the thresholds in force. §6.5 requires a verdict to
	// be inseparable from the configuration that produced it, and this is what
	// makes a row from six months ago interpretable rather than merely dated.
	StandardsHash string `json:"standards_hash,omitempty"`

	// Findings are the finding ids this write turned on, if any.
	Findings []string `json:"findings,omitempty"`

	// Outcome is the envelope status the operation reported, so a trail
	// distinguishes a write that happened from one that was refused. A refusal is
	// worth recording: "we declined to promote this eleven times" is a fact about
	// the corpus that no successful-writes-only log would hold.
	Outcome string `json:"outcome,omitempty"`

	// Signals are the gate signals that could not run when this row was written.
	//
	// It is the debt register (§9.5). A promotion carried by a person over an
	// unrun check is defensible only if the corpus can later find every claim
	// admitted that way — otherwise "a human approved it" is indistinguishable
	// from a bypass, and the unrun check quietly becomes a check that never runs.
	// When the subsystem behind a signal lands, this field is the query that says
	// what to re-examine.
	//
	// Recorded on refusals too, where it answers the other question: a document
	// that never landed may have been blocked by a check nobody has built.
	Signals []string `json:"signals,omitempty"`

	// Detail is one sentence for a person reading the trail directly.
	Detail string `json:"detail,omitempty"`
}

// Canonical is the row's line: compact JSON with a trailing newline.
//
// Requires: the row is valid.
// Ensures: exactly one line, so the trail is readable with a line scanner and
// appending is a single write with no read-modify-write.
func (r *Row) Canonical() ([]byte, error) {
	const op = "audit.Row.Canonical"

	if err := r.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return append(b, '\n'), nil
}

// Validate reports why a row may not be written, or nil.
//
// Requires: nothing.
// Ensures: EINVALID for a row missing the two fields that make it a record of
// anything. It does not check the paths or the hashes: a row for an operation that
// touched nothing is legitimate — a refused promotion is exactly that — and
// requiring a path would push callers into inventing one.
func (r *Row) Validate() error {
	const op = "audit.Row.Validate"

	var bad []string
	if r.Op == OpUnset {
		bad = append(bad, "op is unset")
	}
	if strings.TrimSpace(r.Actor) == "" {
		bad = append(
			bad,
			"actor is empty; a write nobody can be asked about is the case this trail prevents",
		)
	}
	if r.At.IsZero() {
		bad = append(bad, "at is the zero time")
	}
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}
