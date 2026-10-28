package lint

import (
	"sort"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// conflictEdgeCheck reports a deferral that no longer says anything true.
//
// **This is the reader that keeps `gnosis_conflicts` from being stored state nobody
// checks** — the mistake this project has recorded seven times. A deferral suppresses a
// finding from the review queue, so an entry that has gone stale suppresses nothing and
// looks like it does: the reader believes a contradiction is being lived with when the
// corpus can no longer find it.
//
// Three ways an entry stops meaning anything, and each is a different mistake:
//
//   - it names a concept the corpus no longer holds, so the contradiction it describes
//     cannot exist;
//   - it is half-written, so §17.0's who-when-why is incomplete and nothing can say who
//     decided what;
//   - it defers a finding the checks no longer report, which is the good case and still
//     worth saying: the conflict was resolved, and the entry outlived it.
func conflictEdgeCheck() Check {
	return Check{
		Name:       "conflict-edge",
		Categories: []string{"conflict-edge"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    somethingDeferred,
		Run:        staleConflictEdges,
	}
}

// somethingDeferred reports whether any document defers a conflict.
//
// Derived applicability, as every check here has: a corpus where nobody has deferred
// anything is told there is nothing to examine rather than reported clean, which is the
// distinction §12 exists to keep.
func somethingDeferred(snap *Snapshot) (bool, string) {
	for i := range snap.Documents {
		if len(snap.Documents[i].Conflicts) > 0 {
			return true, ""
		}
	}
	return false, "no document defers a conflict — there is nothing to have gone stale"
}

// staleConflictEdges reports every deferral that no longer holds.
//
// Requires: snap.Documents carry their parsed conflict edges; the conflict check's own
// findings are recomputed here rather than passed, because a check receives a snapshot
// and not another check's output.
// Ensures: one diagnostic per stale entry, sorted, each naming what is wrong with it.
// Pure.
func staleConflictEdges(snap *Snapshot) []finding.Diagnostic {
	known := knownConcepts(snap)
	live := liveFindings(snap)

	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Conflicts {
			if d := staleEdge(doc, &doc.Conflicts[j], known, live); d != nil {
				out = append(out, *d)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// staleEdge reports what is wrong with one deferral, or nil when nothing is.
func staleEdge(
	doc *Document, edge *gnosis.ConflictEdge, known map[gnosis.ID]bool, live map[string]bool,
) *finding.Diagnostic {
	switch {
	case !edge.Valid():
		return edgeFinding(doc, edge, "it is incomplete: §17.0 records who saw a"+
			" finding, when, and why they are not acting yet, and an entry missing"+
			" any of those suppresses nothing while looking as though it does")
	case !known[edge.Concept]:
		return edgeFinding(doc, edge, "it names the concept "+edge.Concept.String()+
			", which this corpus does not hold — so the contradiction it defers"+
			" cannot exist and the entry can be removed")
	case !live[edge.Finding]:
		return edgeFinding(doc, edge, "no check reports the conflict it defers any"+
			" more, which usually means somebody resolved it: the entry outlived what"+
			" it was about and can be removed")
	default:
		return nil
	}
}

// edgeFinding renders one stale deferral.
//
// **A warning**, because a stale entry is untidiness rather than a corpus that cannot be
// trusted — and blocking on it would make removing a resolved conflict's paperwork
// urgent, which is the opposite of the priority.
func edgeFinding(doc *Document, edge *gnosis.ConflictEdge, why string) *finding.Diagnostic {
	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "conflict-edge",
		Path:     doc.Path,
		Action:   finding.ActionHuman,
		Message: "the deferred conflict " + edge.Finding + " on " + doc.Path +
			" is stale: " + why,
	}
}

// knownConcepts is every identifier the corpus holds.
func knownConcepts(snap *Snapshot) map[gnosis.ID]bool {
	out := make(map[gnosis.ID]bool, len(snap.Documents))
	for i := range snap.Documents {
		if snap.Documents[i].ID != "" {
			out[snap.Documents[i].ID] = true
		}
	}
	return out
}

// liveFindings is every conflict identity the predicates currently report.
//
// **Recomputed rather than received**, because a check is handed a snapshot and never
// another check's output — which keeps the two independent and means this one cannot be
// wrong about what the other found. The cost is enumerating the residue twice on a
// corpus that defers something, which is a review-queue path and not a hot one.
func liveFindings(snap *Snapshot) map[string]bool {
	pairs := Unseparated(snap)
	out := make(map[string]bool, len(pairs))
	for i := range pairs {
		out[pairs[i].ID()] = true
	}
	return out
}
