// Package lint is the corpus health registry.
//
// Every check is a pure function over a Snapshot: the caller gathers the facts
// from disk and the index, and nothing here performs I/O. That split is what
// makes each check testable from a literal rather than from a fixture directory.
//
// Two structural properties apply to every check, and both come from SPEC §12:
//
// Applicability is derived, not declared. An orphan check is meaningless in a
// corpus that has no links yet, and reporting every document as orphaned on day
// one would teach a reader to ignore the check. Each check reports whether the
// corpus exhibits the convention it examines, and is skipped when it does not.
//
// A run states what it skipped. A check that silently declines to run is
// indistinguishable from a check that found nothing, and the difference matters
// to anyone deciding whether the corpus is healthy or merely unexamined.
package lint

import (
	"sort"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// Snapshot is everything the checks need, gathered by the caller.
//
// It is a value, not an interface: the checks are pure functions and there is
// nothing here to substitute. Assembling one is the shell's job.
type Snapshot struct {
	// Documents is every document found in the bundle.
	Documents []Document

	// Links is every link found in every document body.
	Links []Link

	// Resolutions is the output of gnosis.Reconcile for this corpus.
	Resolutions []gnosis.Resolution

	// LogLines is log.md split into lines, or nil when the bundle has none.
	// OKF §9 makes log.md optional, so absent is not a finding.
	LogLines []string

	// HasLog distinguishes an absent log from an empty one.
	HasLog bool

	// HasIndex reports whether the bundle has a derived index at all. A bundle
	// freshly cloned has none, and in that state every document differs from the
	// index trivially — which is why the index-relative checks are skipped
	// rather than run against nothing.
	HasIndex bool
}

// Document is the subset of a document the checks examine.
type Document struct {
	ID    gnosis.ID
	Path  string
	Type  gnosis.TypeKey
	Title string
}

// Link is one link found in a body.
type Link struct {
	FromID gnosis.ID
	// ToID is empty when the href resolves to no document in the bundle.
	ToID gnosis.ID
	Href string
	// External is true for a link leaving the bundle; those are never orphans
	// or broken links as far as this corpus is concerned.
	External bool
}

// Check is one named health check.
//
// Applies reports whether the corpus exhibits the convention this check
// examines. When it returns false the check is skipped and the reason is
// surfaced, rather than the check running and reporting noise.
type Check struct {
	Name    string
	Applies func(*Snapshot) (bool, string)
	Run     func(*Snapshot) []finding.Diagnostic
}

// Report is the outcome of a run.
type Report struct {
	Diagnostics []finding.Diagnostic `json:"diagnostics"`
	Skipped     []Skip               `json:"skipped,omitempty"`
}

// Skip records a check that did not run, and why.
type Skip struct {
	Check  string `json:"check"`
	Reason string `json:"reason"`
}

// Checks returns the Phase 1 registry, ordered by name.
//
// Requires: nothing.
// Ensures: every returned check has a non-empty Name, an Applies, and a Run.
// The slice is freshly built per call, so a caller cannot mutate the registry
// another caller will see.
func Checks() []Check {
	checks := []Check{
		conformanceCheck(),
		identityCheck(),
		indexDriftCheck(),
		brokenLinkCheck(),
		orphanCheck(),
		logFormatCheck(),
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].Name < checks[j].Name })
	return checks
}

// Run executes every check whose Applies reports true.
//
// Requires: snap is fully populated by the caller; a zero Snapshot is valid and
// describes an empty corpus.
// Ensures: diagnostics are sorted, so two runs over one corpus are comparable.
// Every check that did not run appears in Skipped with a reason — a check that
// silently declines is indistinguishable from one that found nothing.
func Run(snap *Snapshot, checks []Check) Report {
	report := Report{Diagnostics: []finding.Diagnostic{}, Skipped: []Skip{}}
	for _, c := range checks {
		if ok, reason := c.Applies(snap); !ok {
			report.Skipped = append(report.Skipped, Skip{Check: c.Name, Reason: reason})
			continue
		}
		report.Diagnostics = append(report.Diagnostics, c.Run(snap)...)
	}
	finding.Sort(report.Diagnostics)
	return report
}

// always is the applicability of a check that is meaningful on any corpus,
// including an empty one.
func always(*Snapshot) (bool, string) { return true, "" }
