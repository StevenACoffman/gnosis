package ontology

import (
	_ "embed"
)

// FileName is where a bundle's vocabulary lives, relative to its root.
const FileName = "ontology.toml"

// starter is the seed vocabulary, embedded rather than generated from Go values
// because its comments are the part a reviewer reads. Encoding an Ontology back
// to TOML would drop every one of them.
//
//go:embed starter.toml
var starter []byte

// Starter returns the vocabulary a new bundle begins with: the five types of
// SPEC §5.8 and no subjects.
//
// Requires: nothing.
// Ensures: the result is accepted by Load — pinned by a test, because a seed its
// own loader rejects would break every bundle created from it. The returned
// slice is a copy, so a caller cannot corrupt the seed for the next one.
func Starter() []byte {
	out := make([]byte, len(starter))
	copy(out, starter)
	return out
}
