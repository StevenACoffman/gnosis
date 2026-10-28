package web

import (
	"sort"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Term is a vocabulary entry the corpus declares, as a page surfaces it.
//
// The description is the ontology's own `desc`, which is where a corpus already writes
// what a subject means (§5.8) — so this surfaces a definition somebody wrote rather than
// asking for a second one that could disagree with it.
type Term struct {
	Key  string `json:"key"`
	Desc string `json:"desc"`
}

// definedTerms are the declared terms a body actually uses.
//
// Requires: body is the concept's markdown; declared maps a surface phrase to what it
// means.
// Ensures: one entry per declared term the body mentions, ordered by the phrase so two
// loads present the same panel. Pure.
//
// # Why the panel lists only what the page uses
//
// `glossary-18F`'s point, and TODO:1172's: "a glossary nobody opens is not an ontology".
// A page carrying the whole vocabulary is a page whose glossary is scrolled past; a page
// carrying the four terms it actually uses is one where the definition is where the term
// is.
//
// The match is on the folded surface, which is the corpus's own comparison — the same
// fold `subject` resolution and the duplication signal use, rather than a second reading
// of when two phrases are the same phrase.
func definedTerms(body string, declared map[string]string) []Term {
	if len(declared) == 0 {
		return nil
	}
	folded := gnosis.Surface(body).Fold()

	out := make([]Term, 0, len(declared))
	for surface, desc := range declared {
		if strings.Contains(folded, gnosis.Surface(surface).Fold()) {
			out = append(out, Term{Key: surface, Desc: desc})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	if len(out) == 0 {
		return nil
	}
	return out
}
