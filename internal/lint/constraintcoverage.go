package lint

import (
	"cmp"
	"slices"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// constraintCoverageCheck reports, per subject key, how much of the corpus the operator
// patterns can read (§10.2.3).
//
// **Its consumer is `standards/operators.toml`, not an author.** Poor coverage on a key
// has exactly two causes and neither is answered by asking authors for more: the claims
// carry no quantity — "a few retries", "as many as needed" — and there is nothing to pin;
// or a quantity is present in a phrasing the patterns miss, which is a gap in the pattern
// set that closing improves every affected claim retroactively on the next reindex.
//
// So a lopsided key is a backlog item for the patterns and their test corpus, never
// evidence that the corpus needs a stricter authoring rule.
//
// **§10.2.1.1's too-wide rule is deliberately not implemented here, and the reason is
// worth more than the check would be.** That section reports "a pinned constraint whose
// bound spans the plausible range of its dimension" — and nothing declares a plausible
// range for `count`, `duration`, `bytes` or `ratio`. Inventing one is exactly what §6.2
// exists to prevent. The example it gives, *"the retry budget is between 1 and 100"*, is
// also a **range**: under the current single-operator patterns that is two claims rather
// than one constraint, so implementing the rule as written would not catch the case that
// motivates it. Both facts are recorded in §10.2.1.1 rather than only here.
func constraintCoverageCheck() Check {
	return Check{
		Name:       "constraint-coverage",
		Categories: []string{"constraint-coverage"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimNamesASubjectKey,
		Run:        unparsedBySubject,
	}
}

// someClaimNamesASubjectKey reports whether any claim resolves to a subject at all.
//
// Requires: nothing.
// Ensures: a reason whenever it declines. Pure.
//
// Derived applicability, per §12. A corpus whose claims name no declared subject has no
// key to report coverage for, and "coverage is fine" would be a statement about nothing.
func someClaimNamesASubjectKey(snap *Snapshot) (bool, string) {
	if len(snap.Bounds) == 0 {
		return false, "no claim names a subject the vocabulary declares, so there is no" +
			" key to report coverage for"
	}
	return true, ""
}

// unparsedBySubject reports each subject key whose claims the patterns could not read.
//
// Requires: snap.Bounds is keyed by claim id.
// Ensures: one diagnostic per key with at least one unread claim, sorted by key. Keys
// whose claims all parsed are silent. Pure.
//
// **No ratio, and `ok` rather than a gate.** §17 forbids presenting a count as health,
// and a coverage percentage is the most target-shaped number this specification could
// produce: it looks like progress and rises when somebody deletes the claims that do not
// parse. The counts are reported and the reader decides whether a key is lopsided.
func unparsedBySubject(snap *Snapshot) []finding.Diagnostic {
	type tally struct {
		parsed   int
		unparsed []string
	}
	bySubject := map[string]*tally{}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			b, ok := snap.Bounds[claim.ID]
			if !ok {
				continue
			}
			t := bySubject[b.SubjectKey]
			if t == nil {
				t = &tally{}
				bySubject[b.SubjectKey] = t
			}
			if b.Parsed() {
				t.parsed++
				continue
			}
			t.unparsed = append(t.unparsed, doc.Path+" "+claim.ID)
		}
	}

	out := make([]finding.Diagnostic, 0)
	for key, t := range bySubject {
		if len(t.unparsed) == 0 {
			continue
		}
		slices.Sort(t.unparsed)
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "constraint-coverage",
			Message: key + ": " + Noun(t.parsed, "claim") + " parsed to a value, " +
				Noun(len(t.unparsed), "claim") + " did not (" +
				strings.Join(t.unparsed, ", ") + ")" +
				" — either those claims carry no quantity, which is nothing to fix," +
				" or they carry one in a phrasing standards/operators.toml misses," +
				" which is a pattern to add and a case for its corpus",
			Action: finding.ActionHuman,
		})
	}
	slices.SortFunc(out, func(a, b finding.Diagnostic) int {
		return cmp.Compare(a.Message, b.Message)
	})
	return out
}
