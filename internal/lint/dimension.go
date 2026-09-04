package lint

import (
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// dimensionDriftCheck reports a subject whose claims are written in a unit its
// declaration does not describe.
//
// **This is the buildable third of §5.8.2.1's silent-definition-drift problem**, and
// naming which third matters. That entry wants a subject "whose claim population changes
// character": bimodal values, a dimension inconsistent with its declaration, a cluster of
// new aliases. Only the middle one is observable from a single snapshot. Bimodality needs
// a plausible range nobody declares, which is the wall §10.2.1.1 hit and what §6.2
// forbids inventing; an alias cluster needs a stored baseline, which is the wall
// `newly-orphaned` hit. Both are recorded and neither is guessed at.
//
// **What it catches is a definition that moved without anybody saying so.** A subject
// declared `count` whose claims start saying "400ms" is not a typo — it is two groups
// using one key for two things, which invalidates every comparison ever made under the
// old meaning. Hamming's version: "definitions have a habit of changing over time without
// any formal statement of this fact."
//
// **One finding per subject, naming both dimensions and a count.** Per claim it would be
// the loudest check in the tool on the corpus that has the problem worst, and the remedy
// is one edit — to the vocabulary or to the claims — not one per occurrence.
//
// A warning and never a gate, following §5.8.3: the corpus notices, it does not police.
// Which side is wrong is a judgement — the declaration may be stale, or the claims may be
// about a different subject — and a check cannot tell those apart.
func dimensionDriftCheck() Check {
	return Check{
		Name:       "dimension-drift",
		Categories: []string{"dimension-drift"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someBoundCarriesAUnit,
		Run:        driftedDimensions,
	}
}

// someBoundCarriesAUnit reports whether any claim wrote a unit to compare.
//
// **A corpus whose claims are all bare numbers cannot be checked**, and saying so is the
// point: "3" is what every dimension's value looks like when the author omitted the unit,
// so a silent pass and a clean result would be indistinguishable (§12).
func someBoundCarriesAUnit(snap *Snapshot) (bool, string) {
	for id := range snap.Bounds {
		if snap.Bounds[id].Written != "" {
			return true, ""
		}
	}
	return false, "no claim states a value with a unit, so no value's dimension can be" +
		" compared against its subject's"
}

// driftedDimensions reports each subject written in a dimension it does not declare.
//
// Requires: bounds carry both the declared dimension and the written one.
// Ensures: one diagnostic per subject, in key order so two runs agree. Bounds with no
// unit, and bounds whose subject declares no dimension, are skipped — neither states a
// disagreement. Pure.
func driftedDimensions(snap *Snapshot) []finding.Diagnostic {
	type drift struct {
		declared string
		written  map[string]int
	}
	bySubject := map[string]*drift{}
	for id := range snap.Bounds {
		b := snap.Bounds[id]
		if b.Written == "" || b.Dimension == "" || b.Written == b.Dimension {
			continue
		}
		d, ok := bySubject[b.SubjectKey]
		if !ok {
			d = &drift{declared: b.Dimension, written: map[string]int{}}
			bySubject[b.SubjectKey] = d
		}
		d.written[b.Written]++
	}

	keys := make([]string, 0, len(bySubject))
	for k := range bySubject {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]finding.Diagnostic, 0, len(keys))
	for _, key := range keys {
		d := bySubject[key]
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "dimension-drift",
			Path:     "ontology.toml",
			Message: key + " declares dimension " + strconv.Quote(d.declared) +
				" and " + describeWritten(d.written) +
				"; a subject whose values change dimension is one whose meaning moved," +
				" which invalidates every comparison made under the old one — correct" +
				" the declaration, or move these claims to a subject of their own",
			Action: finding.ActionHuman,
		})
	}
	return out
}

// describeWritten renders the units found, in dimension order so the message is stable.
func describeWritten(written map[string]int) string {
	dims := make([]string, 0, len(written))
	for d := range written {
		dims = append(dims, d)
	}
	sort.Strings(dims)

	var out strings.Builder
	for i, d := range dims {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(Noun(written[d], "claim") + " written as " + strconv.Quote(d))
	}
	return out.String()
}
