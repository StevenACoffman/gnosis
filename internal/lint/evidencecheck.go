package lint

import (
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/quotecheck"
)

// evidenceCheck reports a sourced claim whose quotation is no longer in the file it
// names.
//
// **It cannot be about the archive changing, and working out what it *is* about was the
// design.** Tier 0 is content-addressed: a file at a hash cannot come to say something
// else, and a file that is gone is `archive-path`'s finding. What can change is the
// **frontmatter** — somebody edits a quote after admission to tidy the wording, or
// repoints `archive_paths`, and the claim silently stops being supported by the file it
// cites. The promote gate validated it once; nothing has looked since.
//
// So this is a post-admission edit detector, and the message says so, because a reader
// told "the evidence no longer validates" will otherwise go looking at the source.
//
// **An unchecked passage is not a finding**, which is the distinction `quotecheck` exists
// to make and §9.4 leans on throughout: a run below `MinPassageWords` was never searched
// for, and reporting it as unsupported would put "this source does not support X" in
// front of a reader on the strength of a passage too short to check.
func evidenceCheck() Check {
	return Check{
		Name:       "evidence",
		Categories: []string{"evidence"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimCitesArchivedText,
		Run:        unsupportedClaims,
	}
}

// someClaimCitesArchivedText reports whether any claim names both a quote and a file
// whose text was read.
//
// Requires: nothing.
// Ensures: a reason whenever it declines. Pure.
func someClaimCitesArchivedText(snap *Snapshot) (bool, string) {
	for i := range snap.Documents {
		for _, claim := range snap.Documents[i].Claims {
			if len(claim.Quotes) == 0 {
				continue
			}
			for _, path := range claim.ArchivePaths {
				if _, ok := snap.ArchiveText[path]; ok {
					return true, ""
				}
			}
		}
	}
	return false, "no claim names both a quotation and an archived file that could be" +
		" read, so nothing can be re-validated"
}

// unsupportedClaims reports each claim whose quotations the archive no longer holds.
//
// Requires: snap.ArchiveText holds the text of the paths claims cite.
// Ensures: one diagnostic per claim, naming the passages. A claim whose archived files
// were not read is skipped rather than reported — that is `archive-path`'s finding, and
// reporting it here would say a source contradicts a claim when the source was never
// opened. Pure.
func unsupportedClaims(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			if d := unsupportedClaim(snap, doc, &doc.Claims[j]); d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// unsupportedClaim reports one claim, or nil when its evidence still holds.
func unsupportedClaim(snap *Snapshot, doc *Document, claim *Claim) *finding.Diagnostic {
	sources := make([]quotecheck.Source, 0, len(claim.ArchivePaths))
	for _, path := range claim.ArchivePaths {
		text, ok := snap.ArchiveText[path]
		if !ok {
			continue
		}
		sources = append(sources, quotecheck.Source{Name: path, Text: text})
	}
	if len(sources) == 0 || len(claim.Quotes) == 0 {
		return nil
	}

	var missing []string
	for _, f := range quotecheck.Check(claim.Quotes, sources) {
		// Missing is deliberately false for an Unchecked finding: a passage nobody
		// searched for has not failed.
		if f.Missing() {
			missing = append(missing, excerpt(f.Passage))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)

	return &finding.Diagnostic{
		Severity: finding.SeverityError,
		Category: "evidence",
		Path:     doc.Path,
		Message: "claim " + claim.ID + " cites " + noun(len(missing), "passage") +
			" the archived text does not hold: " + strings.Join(missing, ", ") +
			" — the archive is content-addressed and cannot have changed, so the" +
			" frontmatter did: a quotation was edited after admission, or" +
			" archive_paths was repointed. Restore the passage verbatim, or re-admit" +
			" the claim against the source it actually rests on",
		Action: finding.ActionHuman,
	}
}
