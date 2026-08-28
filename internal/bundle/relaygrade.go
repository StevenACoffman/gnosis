package bundle

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// RelayRun is what a real-model relay attempt is graded against (§18.6).
//
// It is the transcript, translated. §18.6 borrows "machine-readable transcript events"
// from a harness that wraps the agent and watches what it does; gnosis does not wrap it
// — the seam is prompt file → agent → reply file, and the agent runs outside this
// process entirely. What gnosis holds instead is the **write trail**, which already
// records every mutation with a time and an actor, so the ordering question is a
// predicate over rows that exist rather than a facility that would have to be built.
type RelayRun struct {
	// Since bounds the run: rows before it belong to whatever happened earlier.
	Since time.Time

	// Key is the prompt the agent was asked to answer.
	Key string

	// Allowed are the operations that may happen before the admission without
	// counting as something else having happened first.
	//
	// **An explicit allowlist rather than a tolerance.** §18.6 calls this the
	// instructive half: a run that merely reached the required step tells you less
	// than one that reached it *first*, and "first" is only meaningful against a
	// stated list of what does not count.
	Allowed []audit.Op
}

// RelayGrade is a real-model run's verdict.
//
// Two findings, and they fail differently. `Admitted` false means the agent never
// produced a reply this corpus would take. `Interfered` names writes that happened
// before it did — a run that reached the right end by a route nobody sanctioned.
type RelayGrade struct {
	// Admitted reports whether the required admission happened.
	Admitted bool `json:"admitted"`

	// Interfered are the operations that happened first and were not allowed, in
	// the order they occurred.
	Interfered []string `json:"interfered,omitempty"`

	// Rows is how many trail rows were examined, so a grade of "clean" on an empty
	// trail is distinguishable from a clean grade on a real one.
	Rows int `json:"rows"`
}

// Held reports whether the run satisfied both assertions.
func (g *RelayGrade) Held() bool { return g.Admitted && len(g.Interfered) == 0 }

// GradeRelay judges a real-model run against the write trail.
//
// Requires: trail came from AuditTrail; run.Key names the prompt that was answered.
// Ensures: a pure predicate over the rows — no clock, no model, no prose. Two calls
// over one trail agree.
//
// **It grades the transcript and never the reply's prose**, which is §18.6's rule and
// the reason this is gradeable at all. Whether a reply reads well is a judgement; whether
// it was admitted, and whether anything else was written first, are facts the trail
// already holds.
func GradeRelay(trail *Trail, run *RelayRun) *RelayGrade {
	allowed := map[audit.Op]bool{}
	for _, op := range run.Allowed {
		allowed[op] = true
	}

	rows := make([]*audit.Row, 0, len(trail.Rows))
	for i := range trail.Rows {
		if !trail.Rows[i].At.Before(run.Since) {
			rows = append(rows, &trail.Rows[i])
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].At.Before(rows[j].At) })

	out := &RelayGrade{Rows: len(rows)}
	for _, row := range rows {
		if isAdmission(row) {
			out.Admitted = true
			// Everything after the admission is somebody else's business: the
			// assertion is about what came *first*.
			break
		}
		if !allowed[row.Op] {
			out.Interfered = append(out.Interfered, string(row.Op))
		}
	}
	return out
}

// isAdmission reports whether a row is the successful admission being graded.
//
// A refused admission does not count. The question §18.6 asks is whether a real model
// produced a reply this corpus would take, and a reply that was examined and declined
// is an answer to that question rather than a partial success.
func isAdmission(row *audit.Row) bool {
	return row.Op == audit.OpAdmit && row.Outcome == string(gnosis.StatusOK)
}

// RelayReport renders a grade for a person, since this run never goes in a gate.
//
// Requires: grade came from GradeRelay.
// Ensures: says which assertion failed, never merely that one did. Pure.
func RelayReport(grade *RelayGrade) string {
	if grade.Held() {
		return "the reply was admitted, and nothing else was written first (" +
			strconv.Itoa(grade.Rows) + " row(s) examined)"
	}
	var why []string
	if !grade.Admitted {
		why = append(why, "no reply was admitted")
	}
	if len(grade.Interfered) > 0 {
		why = append(why, "these were written first and are not on the allowlist: "+
			strings.Join(grade.Interfered, ", "))
	}
	return strings.Join(why, "; ")
}
