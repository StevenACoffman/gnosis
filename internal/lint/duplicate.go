package lint

import (
	"slices"
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/textnorm"
)

// duplicateCheck reports two documents that say the same thing under different
// identifiers.
//
// **This is not a hygiene check about careless copying; it is the merge reconciliation
// step for a distributed corpus** (§4.6.1). Identity is assigned rather than derived from
// content, so two people independently documenting one subject produce two different
// identifiers — and git has no reason to object. It merges both files cleanly, no
// conflict marker appears, no check fails at write time, and the corpus quietly contains
// the same knowledge twice. The condition is invisible until somebody looks, which is why
// this runs after a pull rather than before a commit.
//
// Two categories, because §4.6.1 names two signals whose remedies differ:
//
//   - `duplicate-title`: `Fold`-equal titles. A naming collision — usually a merge, and
//     usually resolved by merging the pages.
//   - `duplicate-evidence`: `Fold`-equal evidence sets. Two pages resting on exactly the
//     same passages under different names, which is a duplication of *content* and is
//     resolved by deciding which page owns the subject.
//
// **One finding per group rather than per pair.** Three documents sharing a title is one
// problem, and three findings would make the report about the documents rather than
// about the collision they are in — the same rule `claim-anchor` applies to colliding
// anchors, one level up.
func duplicateCheck() Check {
	return Check{
		Name:       "duplicate",
		Categories: []string{"duplicate-title", "duplicate-evidence"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    twoDocumentsExist,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			return append(duplicateTitles(snap), duplicateEvidence(snap)...)
		},
	}
}

// twoDocumentsExist reports whether a collision is possible at all.
//
// Derived applicability, per §12: one document cannot duplicate anything, and a corpus
// of one being told it has no duplicates is an answer to a question that could not be
// asked.
func twoDocumentsExist(snap *Snapshot) (bool, string) {
	if len(snap.Documents) < 2 {
		return false, "the corpus holds fewer than two documents, so nothing can collide"
	}
	return true, ""
}

// duplicateTitles reports groups of documents whose titles fold equal.
//
// Requires: nothing.
// Ensures: one diagnostic per colliding group, sorted; documents with no title are
// skipped, because `conformance` reports those and every one of them would otherwise
// collide with every other. Pure.
func duplicateTitles(snap *Snapshot) []finding.Diagnostic {
	byTitle := map[string][]string{}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		title := strings.TrimSpace(doc.Title)
		if title == "" {
			continue
		}
		// Case-insensitively, unlike an anchor: a title names a subject and "Retry
		// Budget" and "retry budget" are the same subject. `claim-anchor` makes the
		// opposite choice one level down because an anchor locates a quotation, where
		// case carries meaning.
		key := strings.ToLower(textnorm.Fold(title))
		byTitle[key] = append(byTitle[key], doc.Path)
	}
	return collisions(byTitle, "duplicate-title",
		"share a title, which a clean merge produces when two people write about one "+
			"subject: identity is assigned rather than derived, so git had no reason to "+
			"object (§4.6.1). Merge them, or retitle whichever is the narrower page")
}

// duplicateEvidence reports groups of documents resting on identical evidence.
//
// Requires: nothing.
// Ensures: one diagnostic per colliding group, sorted. Documents citing no evidence are
// skipped — otherwise every hand-written page in a corpus would share the empty set and
// collide with every other, which is the loudest possible way to say nothing. Pure.
func duplicateEvidence(snap *Snapshot) []finding.Diagnostic {
	byEvidence := map[string][]string{}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		key := evidenceKey(doc)
		if key == "" {
			continue
		}
		byEvidence[key] = append(byEvidence[key], doc.Path)
	}
	return collisions(byEvidence, "duplicate-evidence",
		"rest on exactly the same passages under different names, which is a "+
			"duplication of content rather than of naming. Decide which page owns the "+
			"subject; the other is either a narrower claim or nothing new")
}

// evidenceKey is a document's whole evidence set, folded and ordered.
//
// Requires: nothing.
// Ensures: "" for a document citing no quotations. Order-independent, so two pages that
// cite the same passages in different orders still collide. Pure.
func evidenceKey(doc *Document) string {
	var quotes []string
	for i := range doc.Claims {
		for _, q := range doc.Claims[i].Quotes {
			if trimmed := strings.TrimSpace(q); trimmed != "" {
				quotes = append(quotes, textnorm.Fold(trimmed))
			}
		}
	}
	if len(quotes) == 0 {
		return ""
	}
	slices.Sort(quotes)
	quotes = slices.Compact(quotes)
	return strings.Join(quotes, "\x00")
}

// collisions renders one diagnostic per group of two or more paths.
func collisions(groups map[string][]string, category, why string) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for _, paths := range groups {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: category,
			Path:     paths[0],
			Message: noun(len(paths), "document") + " " + strings.Join(paths, ", ") +
				" " + why,
			Action: finding.ActionHuman,
		})
	}
	// Sorted because the grouping came out of a map, and two runs over one corpus must
	// be comparable.
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}
