package bundle

import (
	"io/fs"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/ontology"
)

// vocabulary flattens a bundle's ontology into the value the checks compare
// against.
//
// Requires: fsys is rooted at the bundle.
// Ensures: an absent ontology yields the undeclared zero value and no error, and a
// malformed one yields the same. Never nil-maps a declared vocabulary.
//
// **A file that will not parse reads here as no vocabulary at all**, and that is
// deliberate rather than lazy. `doctor` already reports the parse failure with the
// message, so returning an error would report one fault twice and leave `lint` with
// neither a vocabulary nor a diagnosis. What the checks then see is `Declared:
// false`, which skips them with a reason — the honest state, since a vocabulary that
// did not load cannot say whether a key is declared.
func vocabulary(fsys fs.FS) lint.Vocabulary {
	raw, err := fs.ReadFile(fsys, ontology.FileName)
	if err != nil {
		return lint.Vocabulary{}
	}
	o, err := ontology.Load(raw)
	if err != nil {
		return lint.Vocabulary{}
	}

	out := lint.Vocabulary{
		Declared:  true,
		Types:     make([]lint.VocabType, 0, len(o.Types)),
		SubjectOf: make(map[gnosis.Surface]gnosis.SubjectKey, len(o.Subjects)),
	}
	for i := range o.Types {
		t := &o.Types[i]
		entry := lint.VocabType{
			Key:            t.Key,
			ExpectsSubject: t.ExpectsSubject,
			Episodic:       t.Episodic,
		}
		if t.Deprecated != nil {
			entry.Deprecated = t.Deprecated.Message
		}
		out.Types = append(out.Types, entry)
	}
	for i := range o.Subjects {
		s := &o.Subjects[i]
		// The key resolves to itself, then every alias. ResolveSubject is what
		// §5.8.2.1's exclusivity rule is enforced through at load, so asking it
		// rather than re-deriving the fold keeps one implementation of "what does
		// this phrase mean" rather than two that can disagree.
		for _, surface := range append([]string{string(s.Key)}, s.Aliases...) {
			if key, ok := o.ResolveSubject(gnosis.Surface(surface)); ok {
				out.SubjectOf[gnosis.Surface(surface)] = key
			}
		}
	}
	return out
}
