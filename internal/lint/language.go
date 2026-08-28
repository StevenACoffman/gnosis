package lint

import (
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/textnorm"
)

// LanguageMarker is one phrase the language check looks for.
type LanguageMarker struct {
	Phrase string
	Role   string
}

// languageCheck reports hedging, weasel words, meaningless comparisons and assuring
// expressions in a document's prose.
//
// **Lexical only, and §10.3 draws that line rather than this check.** Language is
// detectable — "a word list plus a test corpus finds 'industry-leading', 'significantly
// better' with no comparison class, and 'studies show' with no citation" — and reasoning
// is not: "nothing lexical finds a post hoc inference, and a check that claimed to would
// be a classifier wearing a rule's name badge." The distinction is not about difficulty.
// One class is a property of the words and the other is a property of the inference, and
// only the first is present in the artifact.
//
// **This is not the promote gate's hedging signal.** That one refuses an *admission* past
// `hedging_max` softening phrases (§9.5). This reports what the corpus already holds,
// including every page written by hand that no gate ever saw and every page admitted
// before a phrase was added to the list. Same words, two moments, two mechanisms.
//
// **One finding per document, naming the phrases.** A word list applied to every body is
// the loudest thing this tool could do, and §17.3.1's `coverage` already showed what
// per-occurrence reporting costs. Naming them lets a reader judge the register of a page
// in one line; counting them would invite a target, which §17 forbids.
func languageCheck() Check {
	return Check{
		Name:       "language",
		Categories: []string{"language"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    languageMarkersDeclared,
		Run:        markedLanguage,
	}
}

// languageMarkersDeclared reports whether there is a list to check against.
func languageMarkersDeclared(snap *Snapshot) (bool, string) {
	if len(snap.LanguageMarkers) == 0 {
		return false, "standards/language.toml declares no markers"
	}
	return true, ""
}

// markedLanguage reports each document whose prose carries marked phrases.
//
// Requires: the markers are lower-cased and sorted longest first.
// Ensures: one diagnostic per document, phrases named in the order the list declares
// them. A phrase inside a longer marked phrase is not reported twice. Pure.
func markedLanguage(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		found := markersIn(doc.Body, snap.LanguageMarkers)
		if len(found) == 0 {
			continue
		}
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: "language",
			Path:     doc.Path,
			Message: "carries " + noun(len(found), "marked phrase") + ": " +
				strings.Join(found, ", ") +
				" — hedges are often right and the others rarely are;" +
				" a comparison names what it is against, and an attribution names who." +
				" This is the corpus you already have, not an admission being refused",
			Action: finding.ActionHuman,
		})
	}
	return out
}

// markersIn returns the marked phrases present in a body, described by role.
//
// Requires: markers are lower-cased, longest first.
// Ensures: each phrase once, sorted; a phrase contained in a longer match already found
// is skipped, so "clearly" inside a longer marked phrase is not counted twice. Pure.
func markersIn(body string, markers []LanguageMarker) []string {
	folded := " " + strings.ToLower(textnorm.Fold(body)) + " "
	matched := map[string]string{}
	consumed := folded
	for _, m := range markers {
		if !containsPhrase(consumed, m.Phrase) {
			continue
		}
		matched[m.Phrase] = m.Role
		// Remove what matched, so a shorter marker inside it does not match the same
		// words a second time. Markers arrive longest-first, which is what makes this
		// remove the longer phrase before the shorter one is tried.
		consumed = strings.ReplaceAll(consumed, m.Phrase, " ")
	}

	out := make([]string, 0, len(matched))
	for phrase, role := range matched {
		out = append(out, strconv.Quote(phrase)+" ("+role+")")
	}
	sort.Strings(out)
	return out
}

// containsPhrase reports whether a folded body carries a phrase on word boundaries.
func containsPhrase(folded, phrase string) bool {
	for _, form := range []string{" " + phrase + " ", " " + phrase + ",", " " + phrase + "."} {
		if strings.Contains(folded, form) {
			return true
		}
	}
	return false
}
