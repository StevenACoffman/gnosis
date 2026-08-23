package lint

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// logDateHeading matches the "## YYYY-MM-DD" form OKF §9 requires of a log
// entry heading. Other lines in log.md are free prose and are not examined.
var logDateHeading = regexp.MustCompile(`^## \d{4}-\d{2}-\d{2}\s*$`)

// conformanceCheck reports documents missing the one field OKF §4.1 requires.
//
// It deliberately checks nothing else. OKF §11 forbids rejecting a document for
// an unknown type, an unknown key, a broken link, or a missing optional family,
// so a "stricter" conformance check would make gnosis non-conformant.
func conformanceCheck() Check {
	return Check{
		Name:    "conformance",
		Applies: always,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				d := &snap.Documents[i]
				if d.Type != "" {
					continue
				}
				out = append(out, finding.Diagnostic{
					Severity: finding.SeverityError,
					Category: "conformance",
					Path:     d.Path,
					Message:  "document declares no type; OKF §4.1 requires one",
					Action:   finding.ActionGuided,
				})
			}
			return out
		},
	}
}

// indexRelative reports whether a reconciliation outcome is a statement about
// the index rather than about the files.
//
// The distinction decides which of the two checks below owns an outcome, and it
// is worth stating once: a duplicate identifier is a fact about the corpus and
// stays true if the index is deleted, whereas "not in the index" says nothing
// about the documents at all.
func indexRelative(k gnosis.Kind) bool {
	switch k {
	case gnosis.KindIndex, gnosis.KindUpdatePath, gnosis.KindTombstone,
		gnosis.KindConflict:
		return true
	case gnosis.KindDuplicate, gnosis.KindQuarantine:
		return false
	default:
		// An unhandled kind is surfaced by the identity check, which always
		// runs, rather than hidden behind an applicability gate.
		return false
	}
}

// identityCheck reports what the documents say about their own identity.
//
// Severity follows how recoverable each outcome is. A duplicate identifier is
// the only error: it is the case where acting automatically would discard
// somebody's work, so it must stop a caller that gates on blocking findings.
func identityCheck() Check {
	return Check{
		Name:    "identity",
		Applies: always,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			return diagnoseResolutions(snap.Resolutions, false)
		},
	}
}

// indexDriftCheck reports where the index and the bundle disagree.
//
// Applicability is derived: without an index every document is trivially
// unindexed, and a fresh clone reporting one finding per document would teach a
// reader to ignore the check rather than to run `gnosis index rebuild`.
func indexDriftCheck() Check {
	return Check{
		Name: "index-drift",
		Applies: func(snap *Snapshot) (bool, string) {
			if !snap.HasIndex {
				return false, "the bundle has no index yet; run `gnosis index rebuild`"
			}
			return true, ""
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			return diagnoseResolutions(snap.Resolutions, true)
		},
	}
}

// resolutionCategory names the check an outcome is reported by, so a caller
// grouping by category sees the same partition the registry does.
func resolutionCategory(k gnosis.Kind) string {
	if indexRelative(k) {
		return "index-drift"
	}
	return "identity"
}

// diagnoseResolutions renders the resolutions belonging to one check.
func diagnoseResolutions(rs []gnosis.Resolution, wantIndexRelative bool) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0, len(rs))
	for _, r := range rs {
		if indexRelative(r.Kind) != wantIndexRelative {
			continue
		}
		out = append(out, diagnoseResolution(r))
	}
	return out
}

// diagnoseResolution renders one reconciliation outcome.
func diagnoseResolution(r gnosis.Resolution) finding.Diagnostic {
	d := finding.Diagnostic{
		Category: resolutionCategory(r.Kind),
		Severity: finding.SeverityWarning,
		Action:   finding.ActionGuided,
		Path:     firstPath(r),
	}
	switch r.Kind {
	case gnosis.KindDuplicate:
		d.Severity, d.Action = finding.SeverityError, finding.ActionHuman
		d.Message = fmt.Sprintf(
			"identifier %s is carried by %d documents (%s); no winner is chosen",
			r.ID, len(r.Paths), strings.Join(r.Paths, ", "))
	case gnosis.KindConflict:
		d.Severity, d.Action = finding.SeverityError, finding.ActionHuman
		d.Message = fmt.Sprintf(
			"document declares %s but the index records %s at this path", r.ID, r.Other)
	case gnosis.KindQuarantine:
		d.Action = finding.ActionHuman
		d.Message = "document carries no identifier; it was created outside gnosis"
	case gnosis.KindUpdatePath:
		d.Action = finding.ActionAutomatic
		d.Message = fmt.Sprintf("identifier %s moved from %s", r.ID, r.Paths[0])
	case gnosis.KindIndex:
		d.Action = finding.ActionAutomatic
		d.Message = fmt.Sprintf("identifier %s is not in the index", r.ID)
	case gnosis.KindTombstone:
		d.Message = fmt.Sprintf("indexed identifier %s is no longer on disk", r.ID)
	default:
		d.Message = fmt.Sprintf("unhandled reconciliation outcome %q", r.Kind)
	}
	return d
}

// brokenLinkCheck reports links resolving to nothing.
//
// These are warnings, never errors. OKF §6.1 says a link whose target does not
// exist "may simply represent not-yet-written knowledge", so a broken link is a
// gap worth surfacing and never a defect worth blocking on.
func brokenLinkCheck() Check {
	return Check{
		Name:    "broken-link",
		Applies: hasInternalLinks,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			byID := documentPaths(snap)
			out := make([]finding.Diagnostic, 0)
			for _, l := range snap.Links {
				if l.External || l.ToID != "" {
					continue
				}
				out = append(out, finding.Diagnostic{
					Severity: finding.SeverityWarning,
					Category: "broken-link",
					Path:     byID[l.FromID],
					Message: fmt.Sprintf(
						"link to %q resolves to no document (a gap, not a defect)", l.Href),
					Action: finding.ActionHuman,
				})
			}
			return out
		},
	}
}

// orphanCheck reports documents nothing links to.
//
// Applicability is derived: in a corpus with no internal links at all, every
// document is trivially an orphan, and reporting that would be noise rather
// than information.
func orphanCheck() Check {
	return Check{
		Name:    "orphan",
		Applies: hasInternalLinks,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			linked := make(map[gnosis.ID]bool, len(snap.Links))
			for _, l := range snap.Links {
				if l.ToID != "" {
					linked[l.ToID] = true
				}
			}
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				d := &snap.Documents[i]
				if linked[d.ID] {
					continue
				}
				out = append(out, finding.Diagnostic{
					Severity: finding.SeverityWarning,
					Category: "orphan",
					Path:     d.Path,
					Message:  "no document links here",
					Action:   finding.ActionHuman,
				})
			}
			return out
		},
	}
}

// logFormatCheck reports log entry headings that are not the OKF §9 date form.
//
// Applicability is derived: log.md is optional, and an absent log is not a
// finding. Only a log that exists is examined.
func logFormatCheck() Check {
	return Check{
		Name: "log-format",
		Applies: func(snap *Snapshot) (bool, string) {
			if !snap.HasLog {
				return false, "the bundle has no log.md, which OKF §9 permits"
			}
			return true, ""
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i, line := range snap.LogLines {
				if !strings.HasPrefix(line, "## ") || logDateHeading.MatchString(line) {
					continue
				}
				out = append(out, finding.Diagnostic{
					Severity: finding.SeverityWarning,
					Category: "log-format",
					Path:     fmt.Sprintf("log.md:%d", i+1),
					Message:  "entry heading is not the OKF §9 form \"## YYYY-MM-DD\"",
					Action:   finding.ActionGuided,
				})
			}
			return out
		},
	}
}

// hasInternalLinks reports whether the corpus uses internal linking at all.
// Checks about the link graph are meaningless before it exists.
func hasInternalLinks(snap *Snapshot) (bool, string) {
	for _, l := range snap.Links {
		if !l.External {
			return true, ""
		}
	}
	return false, "the corpus has no internal links yet, so link structure cannot be judged"
}

// documentPaths indexes paths by identifier for message rendering.
func documentPaths(snap *Snapshot) map[gnosis.ID]string {
	byID := make(map[gnosis.ID]string, len(snap.Documents))
	for i := range snap.Documents {
		byID[snap.Documents[i].ID] = snap.Documents[i].Path
	}
	return byID
}

// firstPath returns a resolution's leading path, or "" when it has none.
func firstPath(r gnosis.Resolution) string {
	if len(r.Paths) == 0 {
		return ""
	}
	return r.Paths[0]
}
