package lint

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/StevenACoffman/skillet/finding"
)

// AuditHealth is what the shell observed about the write trail.
type AuditHealth struct {
	// Rows is how many rows parsed.
	Rows int `json:"rows"`

	// Malformed are the 1-based line numbers that did not parse.
	Malformed []int `json:"malformed,omitempty"`

	// Newest is the newest row's timestamp, and Head is the bundle's current
	// commit time. Either may be zero, meaning "no answer" rather than "long ago".
	//
	// **Neither produces a finding, and that is a correction to §15.** That
	// section asks for the newest row to be compared against the newest commit,
	// on the reasoning that a trail whose last row predates the last write is the
	// observable form of a silently-lost row. Running it showed the comparison
	// cannot mean that: a person editing a document and committing it is the
	// ordinary way a plain-text corpus is used, and it produces a commit newer
	// than any audit row without anything having gone wrong. The check would fire
	// on the normal workflow, which is worse than not checking.
	//
	// The failure §15 wants caught is caught directly instead, at the moment it
	// happens, by verifying each row after the append (`bundle.verifyAudit`).
	// That is strictly better than inferring it later from timestamps, and it is
	// why removing this finding loses nothing.
	//
	// Both are still reported, because Environment exists so that "a report pasted
	// into an issue is self-contained" — a person diagnosing a trail wants to see
	// these two numbers even where no rule fires on them.
	Newest time.Time `json:"newest,omitzero"`
	Head   time.Time `json:"head,omitzero"`

	// Unreadable is non-empty when the trail could not be read at all. That is
	// operational rather than corruption (§15) and is reported separately, because
	// the two send a reader to different places: one to the disk, one to the file.
	Unreadable string `json:"unreadable,omitempty"`
}

// diagnoseAudit reports on the write trail.
//
// Both findings are **warnings and not errors**, on Diagnose's own rule: a
// condition blocks only when continuing would mean judging the corpus against
// something other than its own rules. A damaged trail does not affect whether the
// corpus is right — it affects whether its history can be recounted. Blocking here
// would make `doctor` exit non-zero on a corpus with nothing wrong with it.
//
// There were three findings in the first draft. The timestamp comparison §15 asks
// for is not among them, for the reason recorded on AuditHealth.Head: it cannot
// distinguish a hand-edited commit from a lost row, and hand-edited commits are how
// a plain-text corpus is normally used.
func diagnoseAudit(env *Environment) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0, 2)
	h := &env.Audit

	if h.Unreadable != "" {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "audit",
			Path:     ".gnosis/audit.jsonl",
			Message:  "the write trail could not be read: " + h.Unreadable,
			Action:   finding.ActionHuman,
		})
		// Nothing below can be judged from a trail nobody could open, and saying
		// so once beats saying "0 rows" as though that were an observation.
		return out
	}
	if len(h.Malformed) > 0 {
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "audit",
			Path:     ".gnosis/audit.jsonl",
			Message: fmt.Sprintf(
				"%d of %d line(s) in the write trail do not parse (line %s); "+
					"the trail cannot be counted",
				len(h.Malformed), len(h.Malformed)+h.Rows, commas(h.Malformed)),
			Action: finding.ActionHuman,
		})
	}
	return out
}

// commas renders line numbers for a person.
func commas(lines []int) string {
	out := make([]string, 0, len(lines))
	for _, n := range lines {
		out = append(out, strconv.Itoa(n))
	}
	return strings.Join(out, ", ")
}
