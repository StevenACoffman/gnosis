package bundle

import (
	"io/fs"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// DurabilityByPath reports, per document, how far what it says can still be checked
// offline (§14.4).
//
// Requires: fsys is rooted at the bundle.
// Ensures: one entry per document the bundle holds, keyed by bundle-relative path;
// empty rather than nil for a corpus with no concepts. A document resting on nothing
// is DurabilityNotApplicable, which is the zero value, so a caller reading a missing
// key gets the same answer as one reading a present one.
//
// **It exists so `search --provable` is a query rather than decoration**, which is
// §14.4's own argument for that flag: a signal a reader cannot filter on is a label.
// The fold is the domain's and the record reading is the shell's, and this function is
// only the join between them.
func DurabilityByPath(fsys fs.FS) (map[string]gnosis.Durability, error) {
	docs, err := Load(fsys)
	if err != nil {
		return nil, err
	}
	support, err := sourceSupport(fsys)
	if err != nil {
		return nil, err
	}
	out := make(map[string]gnosis.Durability, len(docs))
	for i := range docs {
		doc := &docs[i]
		out[doc.Path] = foldEvidence(evidenceOf(doc, support))
	}
	return out, nil
}

// foldEvidence is the projection between the shell's Evidence and the domain's fold.
//
// Requires: nothing.
// Ensures: DurabilityNotApplicable for an empty list. Pure.
//
// `lint`'s `durability` check does the same three lines on its own side of the layer
// boundary, because a check may not import the shell. That is duplication of a
// projection and not of a decision: both call `gnosis.FoldDurability`, which is the
// single authority on what the states mean, so the two cannot come to disagree about
// a document — only about which sources they were handed.
func foldEvidence(evidence []lint.Evidence) gnosis.Durability {
	support := make([]gnosis.Support, 0, len(evidence))
	for _, e := range evidence {
		support = append(support, e.Support)
	}
	return gnosis.FoldDurability(support)
}
