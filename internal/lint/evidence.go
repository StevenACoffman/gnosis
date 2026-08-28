package lint

import (
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// archivePathCheck reports a claim naming archived text that is not there.
//
// A claim's `archive_paths` say where its quotations were found, and §5.5.1 makes
// that address part of the document so it survives a rebuild. Nothing verifies it
// after the fact. The promote gate checks the paths of a document it is promoting,
// which catches the case one step later than it could and only for documents on
// their way in — a document already in the corpus whose evidence was pruned, moved,
// or never committed goes unreported forever.
//
// **What the corpus loses when a path dangles is the ability to fail honestly.**
// §9.4's invariant is that a quotation appears in the named archived file; with no
// file there is nothing to appear in, so the claim is not refuted and not supported.
// It is unverifiable, which §14.4 says must be visible per claim rather than
// averaged away, and an unverifiable claim that reads as an ordinary one is exactly
// the silent pass this corpus exists to prevent.
//
// It reports and never repairs. A missing archive path has three quite different
// causes — the evidence was never fetched, it was fetched and pruned, or the path
// was mistyped — and they call for opposite responses.
func archivePathCheck() Check {
	return Check{
		Name:       "archive-path",
		Categories: []string{"archive-path"},
		Actions:    []finding.Action{finding.ActionGuided},
		Applies: func(snap *Snapshot) (bool, string) {
			// Derived applicability, per §12: a corpus whose documents declare no
			// claims has no addresses to dangle, and reporting nothing found is
			// different from reporting there was nothing to look for.
			for i := range snap.Documents {
				if len(snap.Documents[i].Claims) > 0 {
					return true, ""
				}
			}
			return false, "no document declares claims with archived evidence yet"
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				out = append(out, danglingPaths(&snap.Documents[i], snap.ArchivedText)...)
			}
			return out
		},
	}
}

// danglingPaths reports one diagnostic per claim naming a path that is absent.
//
// One per claim rather than one per path: a claim citing three files from a source
// that was pruned has one problem, and three findings would make the report about
// the paths instead of about the claim that lost its evidence.
func danglingPaths(doc *Document, archived map[string]bool) []finding.Diagnostic {
	var out []finding.Diagnostic
	for i := range doc.Claims {
		claim := &doc.Claims[i]

		missing := make([]string, 0, len(claim.ArchivePaths))
		for _, path := range claim.ArchivePaths {
			if !archived[path] {
				missing = append(missing, path)
			}
		}
		if len(missing) == 0 {
			continue
		}
		sort.Strings(missing)
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "archive-path",
			Path:     doc.Path,
			Message: "claim " + claim.ID + " names " + strconv.Itoa(len(missing)) +
				" archived file(s) that are not in the bundle: " + join(missing) +
				" — the claim cannot be verified or refuted until they are fetched",
			Action: finding.ActionGuided,
		})
	}
	return out
}

// join renders a path list for a message, bounded so one badly-broken document
// cannot make a report unreadable.
func join(paths []string) string {
	const shown = 3
	if len(paths) <= shown {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:shown], ", ") +
		" and " + strconv.Itoa(len(paths)-shown) + " more"
}
