package lint

import (
	"slices"
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// subjectMissingCheck reports claims that do not say what they are about.
//
// §5.8.2 lets a type declare `expects_subject`, and §5.8.3 settles what happens when
// a claim of such a type carries none: it is **reported for review, never blocked and
// never assigned one automatically**. Both halves of that matter and they fail in
// opposite directions — blocking would make the corpus refuse ordinary knowledge, and
// guessing would put an inferred key underneath §10's comparison gate, which §10.3
// refuses on principle. Reporting puts it in front of a person, which is where the
// judgment belongs.
//
// So the severity is a warning and the action is a person's. This check earns its
// place by being cheap to dismiss: many claims of a normative type legitimately
// constrain nothing.
func subjectMissingCheck() Check {
	return Check{
		Name:       "subject-missing",
		Categories: []string{"subject-missing"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    expectsSubjectSomewhere,
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				doc := &snap.Documents[i]
				declared, ok := snap.Vocabulary.TypeNamed(doc.Type)
				if !ok || !declared.ExpectsSubject {
					// An undeclared type is the `ontology` check's finding, not this
					// one. Reporting it twice would make a single vocabulary edit
					// look like two problems.
					continue
				}
				out = append(out, unsubjectedClaims(doc)...)
			}
			return out
		},
	}
}

// subjectUnknownCheck reports a claim naming a subject the vocabulary does not
// declare.
//
// This is the other half of §5.8.2.1's exclusivity rule, seen from the document. That
// rule guarantees one surface phrase resolves to one key; this reports the phrase that
// resolves to none — a typo, a key that was renamed, or a word somebody expected to be
// declared and never was. All three are a person's to settle, and the third is the one
// worth having: a claim about something the corpus has no word for is a vocabulary gap
// stated in the only place it is visible.
//
// It resolves through aliases, because §5.8.2.1's whole point is that engineering's
// "retry budget" and support's "retry cap" reach one key. A check comparing against
// keys alone would report every alias as unknown and teach people to stop using them.
func subjectUnknownCheck() Check {
	return Check{
		Name:       "subject-unknown",
		Categories: []string{"subject-unknown"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimNamesASubject,
		Run:        unresolvedSubjects,
	}
}

// someClaimNamesASubject reports whether there is anything for subject-unknown to
// resolve, and says which of the three ways there is not.
//
// Requires: nothing.
// Ensures: a reason whenever it declines, naming the missing thing rather than the
// check. Pure.
//
// **The middle case is the one that had to be found by running the tool.** The
// starter vocabulary declares no subjects, only a commented example — so without that
// guard the first claim anybody writes a subject on is reported unknown, and the
// lesson a reader takes is to stop writing subjects. A corpus that has not started
// declaring subjects is not a corpus whose phrase failed to resolve, and reporting one
// as the other is the absence of the ruler reported as a fault in the thing measured.
func someClaimNamesASubject(snap *Snapshot) (bool, string) {
	if !snap.Vocabulary.Declared {
		return false, "the bundle declares no ontology.toml"
	}
	if len(snap.Vocabulary.SubjectOf) == 0 {
		return false, "ontology.toml declares no subjects yet, so no phrase could resolve"
	}
	for i := range snap.Documents {
		claims := snap.Documents[i].Claims
		for j := range claims {
			if strings.TrimSpace(claims[j].Subject) != "" {
				return true, ""
			}
		}
	}
	return false, "no claim names a subject yet"
}

// unresolvedSubjects reports each claim whose subject phrase names no declared key.
//
// Requires: the vocabulary declares at least one subject.
// Ensures: one diagnostic per unresolved claim, and none for a claim naming no
// subject — that is subject-missing's finding, on the types that ask for one. Pure.
func unresolvedSubjects(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			surface := strings.TrimSpace(claim.Subject)
			if surface == "" {
				continue
			}
			if _, ok := snap.Vocabulary.ResolvesSubject(surface); ok {
				continue
			}
			out = append(out, finding.Diagnostic{
				Severity: finding.SeverityWarning,
				Category: "subject-unknown",
				Path:     doc.Path,
				Message: "claim " + claim.ID + " is about " + excerpt(surface) +
					", which ontology.toml does not declare as a subject key or" +
					" an alias of one — declare it, or correct the claim to a" +
					" phrase that resolves",
				Action: finding.ActionHuman,
			})
		}
	}
	return out
}

// expectsSubjectSomewhere reports whether any declared type asks its claims what they
// are about.
//
// Derived applicability, per §12. A corpus whose every type has `expects_subject =
// false` has nothing for this check to find, and "found nothing" would read as a
// clean bill rather than as a question nobody asked.
func expectsSubjectSomewhere(snap *Snapshot) (bool, string) {
	if !snap.Vocabulary.Declared {
		return false, "the bundle declares no ontology.toml"
	}
	for i := range snap.Vocabulary.Types {
		if snap.Vocabulary.Types[i].ExpectsSubject {
			return true, ""
		}
	}
	return false, "no declared type expects its claims to name a subject"
}

// unsubjectedClaims reports each of a document's claims carrying no subject.
//
// Requires: doc's type is declared and expects a subject.
// Ensures: one diagnostic per claim, never per document. Pure.
//
// Per claim rather than per document because the remedy is per claim: a document with
// four claims of which one is unsubjected needs one edit, and a document-level finding
// would send its reader looking for which.
func unsubjectedClaims(doc *Document) []finding.Diagnostic {
	var out []finding.Diagnostic
	for i := range doc.Claims {
		claim := &doc.Claims[i]
		if strings.TrimSpace(claim.Subject) != "" {
			continue
		}
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "subject-missing",
			Path:     doc.Path,
			Message: "claim " + claim.ID + " is of type " + doc.Type.String() +
				", which expects a subject, and names none" +
				" — add one, or leave it if this claim constrains nothing," +
				" which is ordinary and not a defect",
			Action: finding.ActionHuman,
		})
	}
	return out
}

// ontologyCheck reports a vocabulary and a corpus that have drifted apart.
//
// Three findings, one join. All three come from comparing the declared types against
// the types documents actually carry, and a reader acting on any of them opens
// `ontology.toml` — which is why this is one check rather than three sharing a helper.
//
//   - `type-undeclared`: a document declares a type the vocabulary does not. The
//     document is not wrong: OKF §11 requires unknown `type` values be tolerated, so
//     this reports a vocabulary that has fallen behind its corpus, never a document
//     to be rejected.
//   - `type-unused`: a declared type no document uses. §10.6's attenuation argument
//     in miniature — a vocabulary entry nothing exercises is a knob whose behaviour
//     nobody has observed.
//   - `type-deprecated`: a type still in use after its announcement. §5.8.1's
//     announce-then-enforce path only works if somebody is told during the announce
//     half, and this is the telling.
func ontologyCheck() Check {
	return Check{
		Name:       "ontology",
		Categories: []string{"type-undeclared", "type-unused", "type-deprecated"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies: func(snap *Snapshot) (bool, string) {
			if !snap.Vocabulary.Declared {
				return false, "the bundle declares no ontology.toml"
			}
			return true, ""
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			used := typesInUse(snap.Documents)
			out := undeclaredTypes(snap, used)
			out = append(out, unusedTypes(snap, used)...)
			return append(out, deprecatedTypesInUse(snap, used)...)
		},
	}
}

// typesInUse counts the documents carrying each type key.
//
// Requires: nothing.
// Ensures: a count rather than a set, because two of the three findings want to name
// how many documents an edit would touch. Pure.
func typesInUse(docs []Document) map[gnosis.TypeKey]int {
	out := map[gnosis.TypeKey]int{}
	for i := range docs {
		if key := docs[i].Type; key != "" {
			out[key]++
		}
	}
	return out
}

// undeclaredTypes reports type keys the corpus uses and the vocabulary does not hold.
func undeclaredTypes(snap *Snapshot, used map[gnosis.TypeKey]int) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for _, key := range sortedKeys(used) {
		if _, ok := snap.Vocabulary.TypeNamed(key); ok {
			continue
		}
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "type-undeclared",
			Message: documentsCarrying(used[key]) + " type " + excerpt(key.String()) +
				", which ontology.toml does not declare" +
				" — the documents are conformant either way (OKF §11 tolerates" +
				" an unknown type); it is the vocabulary that has fallen behind",
			Action: finding.ActionHuman,
		})
	}
	return out
}

// unusedTypes reports declared types no document carries, as one diagnostic.
//
// Requires: nothing.
// Ensures: at most one diagnostic, naming every unused type in declaration order.
// Empty for a corpus that uses no types at all. Pure.
//
// **One finding rather than one per type, and nothing at all on a corpus using
// none.** `finding` has two severities and neither is advisory, so every diagnostic
// here is a warning somebody must dismiss. The starter vocabulary ships six types and
// a new bundle uses one or two, which would make this the loudest check in the tool
// on the day a corpus is created — and a check that is loudest when there is least to
// say teaches its reader to skip it. Grouping keeps the cost at one line, and the
// zero-types guard keeps a bundle with no documents from being told its vocabulary is
// unused when what is missing is the corpus.
//
// A deprecated type nothing uses is excluded: that is the announcement working.
// Reporting it would ask somebody to delete the entry currently telling authors what
// to use instead.
func unusedTypes(snap *Snapshot, used map[gnosis.TypeKey]int) []finding.Diagnostic {
	if len(used) == 0 {
		return nil
	}
	var unused []string
	for i := range snap.Vocabulary.Types {
		declared := &snap.Vocabulary.Types[i]
		if used[declared.Key] == 0 && declared.Deprecated == "" {
			unused = append(unused, declared.Key.String())
		}
	}
	if len(unused) == 0 {
		return nil
	}
	return []finding.Diagnostic{{
		Severity: finding.SeverityWarning,
		Category: "type-unused",
		Message: "no document is of " + Noun(len(unused), "declared type") + " " +
			strings.Join(unused, ", ") +
			" — a vocabulary entry nothing exercises is one whose behaviour nobody" +
			" has observed; a corpus that has only just started is expected to be" +
			" in this state",
		Action: finding.ActionHuman,
	}}
}

// documentsCarrying opens a message with how many documents carry something.
//
// Requires: nothing.
// Ensures: the noun and its verb agree. Pure.
//
// It carries the verb because the noun alone does not settle it, and the first
// version of these messages read "1 document declare type" — visible the moment the
// binary was run over a real corpus and invisible in a test asserting on a substring.
func documentsCarrying(n int) string {
	if n == 1 {
		return "1 document carries"
	}
	return strconv.Itoa(n) + " documents carry"
}

// deprecatedTypesInUse reports documents still carrying an announced-away type.
//
// The announcement's own message is quoted rather than summarised: §5.8.1 makes the
// message *be* the announcement, and an author told only "deprecated" has no account
// of what to use instead.
func deprecatedTypesInUse(snap *Snapshot, used map[gnosis.TypeKey]int) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Vocabulary.Types {
		declared := &snap.Vocabulary.Types[i]
		if declared.Deprecated == "" || used[declared.Key] == 0 {
			continue
		}
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "type-deprecated",
			Message: documentsCarrying(used[declared.Key]) + " type " +
				excerpt(declared.Key.String()) + ", which is deprecated: " +
				declared.Deprecated,
			Action: finding.ActionHuman,
		})
	}
	return out
}

// sortedKeys orders a type-keyed map, so two runs over one corpus report alike.
func sortedKeys(m map[gnosis.TypeKey]int) []gnosis.TypeKey {
	out := make([]gnosis.TypeKey, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

// Noun renders a count inside a noun phrase, so no verb has to agree with it.
//
// Requires: word is the singular form.
// Ensures: "1 quotation" and "2 quotations". Pure.
//
// **It is §17.5's remedy and it is exported for one caller outside this package.** Three
// findings shipped saying "1 document declare", "1 claim name" and "1 command that do
// not resolve", each written by composing a number with a sentence built for the plural
// case, and each caught by running the binary rather than by a test. The rule applies to
// any count in any message, so the findings gate one layer up calls this rather than
// keeping a second copy: a rule spelled twice is a rule that can be fixed once.
func Noun(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
