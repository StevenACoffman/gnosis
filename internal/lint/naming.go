package lint

import (
	"path"
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// filenameDriftCheck reports a document whose filename no longer matches its title.
//
// §5.1.1 splits a concept filename in two: the identifier is the durable half — a link
// written before a retitle still resolves through it — and the slug is the readable half,
// which is allowed to go stale. So drift here costs nothing structural, and this is the
// one check §12 says is "corrected on the next write" rather than by a person.
//
// **The action is automatic and this check still does not rename.** Those are consistent
// and the distinction took a failing test to state: the fix needs nobody's confirmation
// — §12.1's definition of automatic — because it happens as part of the *next write of
// this document*, which is a write its author already asked for. What would need asking
// is renaming a file underneath a reader on the strength of a lint run, for a cosmetic
// gain; §5.6 makes a presented path a view, and moving one nobody asked to move is a
// change of a different kind. So the finding names the fix and the writer performs it.
func filenameDriftCheck() Check {
	return Check{
		Name:       "filename-drift",
		Categories: []string{"filename-drift"},
		Actions:    []finding.Action{finding.ActionAutomatic},
		Applies:    someDocumentIsNamedByGnosis,
		Run:        driftedFilenames,
	}
}

// someDocumentIsNamedByGnosis reports whether any document carries a gnosis-assigned
// name to have drifted from.
//
// Requires: nothing.
// Ensures: a reason whenever it declines. Pure.
//
// **A hand-written file is not drifted, it is simply named.** A document gnosis never
// assigned an identifier to has no slug convention to violate, and reporting every such
// file would make this the loudest check on any corpus somebody started by hand —
// which is every corpus before its first promotion.
func someDocumentIsNamedByGnosis(snap *Snapshot) (bool, string) {
	for i := range snap.Documents {
		if snap.Documents[i].ID != "" {
			return true, ""
		}
	}
	return false, "no document carries a gnosis identifier, so no filename was assigned" +
		" by gnosis to have drifted"
}

// driftedFilenames reports each identified document whose slug is stale.
//
// Requires: nothing.
// Ensures: one diagnostic per document, in document order. A document with no title is
// skipped — `conformance` reports that, and a slug derived from nothing would be
// "untitled", which every such document would then share. Pure.
func driftedFilenames(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		if doc.ID == "" || strings.TrimSpace(doc.Title) == "" {
			continue
		}
		want := gnosis.SlugFrom(doc.Title).String()
		if got := slugOf(doc.Path); got == "" || got == want {
			continue
		}
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "filename-drift",
			Path:     doc.Path,
			Message: "the filename's slug no longer matches the title: the title yields " +
				strconv.Quote(want) + " and the file says " + strconv.Quote(slugOf(doc.Path)) +
				" — the identifier still resolves every link, so nothing is broken;" +
				" the next write of this document corrects the name",
			Action: finding.ActionAutomatic,
		})
	}
	return out
}

// slugOf recovers the readable half of a concept filename.
//
// Requires: nothing.
// Ensures: "" for any name that is not "<36-character identifier>-<slug>.md", which
// includes every hand-written file. Pure.
func slugOf(p string) string {
	name := strings.TrimSuffix(path.Base(p), ".md")
	// The identifier is 36 characters and a hyphen separates it from the slug.
	const idLen = 36
	if len(name) < idLen+2 || name[idLen] != '-' {
		return ""
	}
	return name[idLen+1:]
}

// limitationsCheck reports a normative concept that declares no limits (§17.2).
//
// A page that prescribes and states nothing it does not cover is asserting a scope it has
// not examined. §17.2's remedy is to write the limits down, which is a judgement about
// the subject matter — so this is a warning and a person's, never a gate.
//
// **Only normative types**, which `VocabType.Normative` already carries. A Reference
// records rather than prescribes, and asking it for limits would be asking a fact to
// bound itself.
func limitationsCheck() Check {
	return Check{
		Name:       "limitations",
		Categories: []string{"limitations"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someTypePrescribes,
		Run:        unboundedNormativeConcepts,
	}
}

// someTypePrescribes reports whether any declared type is normative.
//
// Derived applicability, per §12: a corpus whose every type merely records has nothing
// for this to find, and "no findings" would read as a clean bill for a question nobody
// asked.
func someTypePrescribes(snap *Snapshot) (bool, string) {
	if !snap.Vocabulary.Declared {
		return false, "the bundle declares no ontology.toml"
	}
	for i := range snap.Vocabulary.Types {
		if snap.Vocabulary.Types[i].Normative {
			return true, ""
		}
	}
	return false, "no declared type prescribes, so no concept owes a limitations list"
}

// unboundedNormativeConcepts reports each prescribing document with no declared limits.
func unboundedNormativeConcepts(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		declared, ok := snap.Vocabulary.TypeNamed(doc.Type)
		if !ok || !declared.Normative || len(doc.Limitations) > 0 {
			continue
		}
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "limitations",
			Path:     doc.Path,
			Message: "this concept is of prescribing type " + doc.Type.String() +
				" and declares no gnosis_limitations — a page that says what must be" +
				" done and nothing about what it does not cover asserts a scope nobody" +
				" examined; write the limits, or reconsider whether the type fits",
			Action: finding.ActionHuman,
		})
	}
	return out
}
