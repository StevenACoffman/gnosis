package splitcmd

import (
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/scan"
)

// Unfetchable is a file split would leave behind that fetch will not archive as text,
// and what fetch will say about it.
type Unfetchable struct {
	Path     string   `json:"path"`
	Reason   string   `json:"reason"`
	Findings []string `json:"findings,omitempty"`
}

// plannedFile is one file split intends to leave for fetch: a part it writes, or a
// source small enough to be fetched whole.
type plannedFile struct {
	path    string
	content []byte
}

// predict reports which planned files fetch would refuse, and why.
//
// Requires: gates and rules are the ones fetch would load from the same bundle.
// Ensures: one Unfetchable per file whose disposition would not be `archived`, in the
// order given, each carrying the same findings fetch prints. Pure.
//
// It runs fetch's own decision rather than a list of what fetch tends to refuse,
// because the point is to agree with fetch exactly. Every source the admission scan
// refused in agent-orange/kb was discovered only after its parts had been written,
// committed and fetched; predicting it here moves that discovery to before anything
// is written, when the fix is an edit to one source rather than a re-split.
func predict(files []plannedFile, gates archive.Gates, rules *scan.Ruleset) []Unfetchable {
	var out []Unfetchable
	for _, f := range files {
		cand := &archive.Candidate{
			URI:       f.path,
			Bytes:     f.content,
			Extension: strings.ToLower(filepath.Ext(f.path)),
		}
		why := archive.Decide(cand, gates).Record.RejectReason
		if why == archive.ReasonNone {
			continue
		}
		out = append(out, Unfetchable{
			Path:     f.path,
			Reason:   string(why),
			Findings: bundle.RefusalFindings(rules, f.content, why, gates),
		})
	}
	return out
}
