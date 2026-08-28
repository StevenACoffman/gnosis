package bundle

import (
	"os"

	"github.com/StevenACoffman/gnosis/internal/schema"
)

// IndexFile is OKF §3.1's entry point, at the bundle root and committed.
const IndexFile = "index.md"

// IndexSibling is where the generated listing goes when the committed file carries no
// markers.
//
// **This is the common path rather than the edge**, and that is worth stating where
// somebody will read it. `init` has seeded `index.md` with curated prose since the
// beginning, so every bundle that exists has a hand-written one. §5.7.1's fail-closed
// rule means none of them is converted silently: the listing lands beside the file and
// the owner decides whether to paste the markers in.
const IndexSibling = "index.generated.md"

// indexPreamble is the document's opening, written once when there is no file at all.
//
// It is outside every marker, so the first thing a reader is told is that the rest is
// theirs — and so this sentence is the first thing gnosis stops maintaining.
//
// **It says "map, not mirror" because the seeded prose did**, and that instruction is
// still right about the part a person writes. What changed is the reason the derived
// list belongs in the file rather than only in a command: §5.7's argument is that an
// agent reads *files*, and a listing reachable only by running `gnosis search` is not
// reachable by the reader this document exists for. OKF §8's progressive disclosure is
// the same argument.
const indexPreamble = "# Index\n" +
	"\n" +
	"The entry point to this knowledge base, for a reader — or an agent — arriving " +
	"with a question and no idea where to look.\n" +
	"\n" +
	"Everything outside the `gnosis:begin`/`gnosis:end` markers is yours and is " +
	"preserved byte for byte. Keep it a **map, not a mirror**: the handful of paths " +
	"through the corpus a newcomer actually needs. The generated region below is the " +
	"mirror, so your prose does not have to be."

// PlanIndexDoc works out what the index document should contain.
//
// Requires: bundleDir is a bundle root, which need not exist.
// Ensures: writes nothing. A bundle whose concepts will not load yields an error,
// unlike an absent ontology in PlanSchemaDoc — a listing built from a partial read
// would silently omit documents, and an index that quietly loses a page is worse than
// one that is not written.
func PlanIndexDoc(bundleDir string) (MarkedDoc, error) {
	const op = "bundle.PlanIndexDoc"

	docs, err := Load(os.DirFS(bundleDir))
	if err != nil {
		return MarkedDoc{}, err
	}
	return planMarked(op, bundleDir, IndexFile, IndexSibling, indexPreamble,
		schema.IndexRegions(indexEntries(docs)))
}

// indexEntries projects loaded documents into the listing's shape.
//
// Requires: nothing.
// Ensures: one entry per document, in load order — the region sorts. Pure.
//
// Title falls back to the path because a document with no title still has to be
// findable: omitting it would make the listing disagree with the corpus, and the
// missing title is `conformance`'s finding rather than this listing's to hide.
func indexEntries(docs []Document) []schema.DocEntry {
	out := make([]schema.DocEntry, 0, len(docs))
	for i := range docs {
		d := &docs[i]
		title := d.Title
		if title == "" {
			title = d.Path
		}
		out = append(out, schema.DocEntry{
			Type: d.Type.String(), Title: title, Path: d.Path,
		})
	}
	return out
}
