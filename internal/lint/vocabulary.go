package lint

import (
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Vocabulary is `ontology.toml` flattened to what the checks ask of it.
//
// It is a value the shell gathers, like every other Snapshot field, and that is
// what keeps this package's only internal import `internal/gnosis`. A check that
// imported the ontology parser could ask it anything; one handed this can ask only
// the three questions below, which is the whole of what §5.8's checks need.
//
// It is not a second name for an ontology. An `ontology.Ontology` is a parsed file
// with declaration rules, deprecation announcements, and rejection records; this is
// the subset a check compares against, with aliases already resolved.
type Vocabulary struct {
	// Declared reports whether the bundle has an ontology at all.
	//
	// Separate from `len(Types) > 0` because the two states must not render alike:
	// a corpus with no `ontology.toml` has nothing to check against, and reporting
	// every type in it as undeclared would be reporting the absence of the ruler as
	// a fault in the thing measured. HasLog is beside LogLines for the same reason.
	Declared bool

	// Types are the declared types in declaration order, which is the order
	// `ontology.toml` lists them and therefore the order a person edits.
	Types []VocabType

	// SubjectOf resolves every declared subject surface — key and alias alike — to
	// the key it stands for.
	//
	// A map rather than a set because §5.8.2.1 makes an alias exclusive: the phrase
	// an author writes is often not the key it resolves to, and the population
	// report groups by the key. Nil for a corpus declaring no subjects, which reads
	// correctly — no surface resolves.
	SubjectOf map[gnosis.Surface]gnosis.SubjectKey
}

// VocabType is one declared type, down to the facts a check compares against.
type VocabType struct {
	Key gnosis.TypeKey

	// ExpectsSubject reports whether a claim of this type is expected to name what
	// it is about (§5.8.2). It drives a review signal and never a refusal.
	ExpectsSubject bool

	// Episodic reports whether this type's claims assert what happened at a moment
	// rather than what holds in general (§5.8.3.1).
	Episodic bool

	// Deprecated is the announcement's message, and is empty when the type is
	// current. The message rather than a bare flag: §5.8.1 makes the announcement
	// *be* the message, and an author told only "deprecated" has no account of what
	// to use instead.
	Deprecated string
}

// ResolvesSubject reports whether a surface phrase names a declared subject.
//
// Requires: nothing; the zero Vocabulary resolves nothing.
// Ensures: false for the empty phrase, so a claim declaring no subject is never
// reported as declaring an unknown one — those are different findings with
// different remedies. Pure.
func (v *Vocabulary) ResolvesSubject(surface string) (gnosis.SubjectKey, bool) {
	if surface == "" {
		return "", false
	}
	key, ok := v.SubjectOf[gnosis.Surface(surface)]
	return key, ok
}

// TypeNamed returns the declared type with this key.
//
// Requires: nothing.
// Ensures: reports false for an undeclared key, which is itself a finding rather
// than an error. Pure.
//
// A linear scan because a vocabulary holds a handful of types and a map would be a
// second representation of the same list to keep in step.
func (v *Vocabulary) TypeNamed(key gnosis.TypeKey) (*VocabType, bool) {
	for i := range v.Types {
		if v.Types[i].Key == key {
			return &v.Types[i], true
		}
	}
	return nil, false
}
