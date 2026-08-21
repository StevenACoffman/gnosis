package gnosis

import "sort"

// The reconciliation outcomes of SPEC §5.1.2.
const (
	// KindIndex is a document carrying an identifier the index has never seen:
	// newly created, or an index rebuilt from scratch.
	KindIndex Kind = "index"

	// KindUpdatePath is one identifier at a path the index did not expect. The
	// document moved or was renamed; nothing about its content changed.
	KindUpdatePath Kind = "update-path"

	// KindTombstone is an indexed identifier no longer present anywhere. The
	// document was deleted outside gnosis. It is reported, never silently
	// dropped, because a deletion the tool did not perform is worth seeing.
	KindTombstone Kind = "tombstone"

	// KindDuplicate is one identifier carried by two or more documents: a copy,
	// or a bad merge. No winner is chosen — see Reconcile's contract.
	KindDuplicate Kind = "duplicate"

	// KindQuarantine is a document with no identifier at all, created outside
	// gnosis. An identifier is never assigned silently, because doing so would
	// adopt an unvetted file into the corpus by accident.
	KindQuarantine Kind = "quarantine"

	// KindConflict is a path whose document and whose index row disagree about
	// which identifier belongs there. Both changed independently.
	KindConflict Kind = "conflict"
)

// Kind names what a reconciliation found. The set is closed: every discrepancy
// between the filesystem and the index is exactly one of these, and a new
// observation that fits none of them is a gap in the model rather than a case
// to fold into the nearest neighbour.
type Kind string

// Observed is a document as found on disk.
type Observed struct {
	Path string
	// ID is empty when the document carries no gnosis_id.
	ID ID
}

// Indexed is a row as recorded in the derived index.
type Indexed struct {
	Path string
	ID   ID
}

// Resolution is one finding about one discrepancy.
//
// Paths carries every path involved: one for most kinds, the old and new for
// KindUpdatePath, and all of them for KindDuplicate. Other is populated only
// for KindConflict, where it holds the identifier the index expected.
type Resolution struct {
	Kind  Kind
	ID    ID
	Paths []string
	Other ID
}

// Reconcile compares what is on disk against what the index believes.
//
// Requires: observed lists every document found, with an empty ID where the
// document carries none; indexed lists every current row. Neither need be sorted.
// Ensures: returns one Resolution per discrepancy and never two for the same
// discrepancy; a document whose path and identifier both match the index yields
// nothing. Output is sorted, so a rebuild is comparable across runs. A
// KindDuplicate resolution carries every conflicting path and **chooses no
// winner** — picking one would silently discard whichever copy holds a
// colleague's work, which is the failure this whole identity model exists to
// prevent.
func Reconcile(observed []Observed, indexed []Indexed) []Resolution {
	byObservedID := map[ID][]string{}
	var out []Resolution

	for _, o := range observed {
		if o.ID == "" {
			out = append(out, Resolution{Kind: KindQuarantine, Paths: []string{o.Path}})
			continue
		}
		byObservedID[o.ID] = append(byObservedID[o.ID], o.Path)
	}

	byIndexedID := make(map[ID]string, len(indexed))
	byIndexedPath := make(map[string]ID, len(indexed))
	for _, i := range indexed {
		byIndexedID[i.ID] = i.Path
		byIndexedPath[i.Path] = i.ID
	}

	// displaced collects identifiers already accounted for by a conflict, so
	// the tombstone pass does not report the same fact a second time.
	displaced := map[ID]bool{}
	out = append(out, resolveObserved(byObservedID, byIndexedID, byIndexedPath, displaced)...)
	out = append(out, resolveMissing(byObservedID, indexed, displaced)...)

	sortResolutions(out)
	return out
}

// resolveObserved walks what is on disk and classifies each identifier.
func resolveObserved(
	byObservedID map[ID][]string,
	byIndexedID map[ID]string,
	byIndexedPath map[string]ID,
	displaced map[ID]bool,
) []Resolution {
	out := make([]Resolution, 0, len(byObservedID))
	for id, paths := range byObservedID {
		if len(paths) > 1 {
			sort.Strings(paths)
			out = append(out, Resolution{Kind: KindDuplicate, ID: id, Paths: paths})
			continue
		}
		path := paths[0]

		indexedPath, known := byIndexedID[id]
		switch {
		case known && indexedPath == path:
			// Path and identifier agree; nothing to report.
		case known:
			out = append(out, Resolution{
				Kind: KindUpdatePath, ID: id, Paths: []string{indexedPath, path},
			})
		default:
			// Unknown identifier. If the index has a different one recorded at
			// this path, the two disagree about what belongs here; reporting
			// that is more useful than reporting an unrelated add and delete.
			if prior, occupied := byIndexedPath[path]; occupied && prior != id {
				displaced[prior] = true
				out = append(out, Resolution{
					Kind: KindConflict, ID: id, Paths: []string{path}, Other: prior,
				})
				continue
			}
			out = append(out, Resolution{Kind: KindIndex, ID: id, Paths: []string{path}})
		}
	}
	return out
}

// resolveMissing reports indexed identifiers that are no longer on disk.
func resolveMissing(
	byObservedID map[ID][]string,
	indexed []Indexed,
	displaced map[ID]bool,
) []Resolution {
	out := make([]Resolution, 0)
	for _, i := range indexed {
		if _, stillPresent := byObservedID[i.ID]; stillPresent || displaced[i.ID] {
			continue
		}
		out = append(out, Resolution{Kind: KindTombstone, ID: i.ID, Paths: []string{i.Path}})
	}
	return out
}

// sortResolutions imposes a total order so two runs over the same corpus
// produce identical output. Map iteration is randomised, so without this a
// rebuild could not be compared against its predecessor.
func sortResolutions(rs []Resolution) {
	sort.Slice(rs, func(a, b int) bool {
		x, y := rs[a], rs[b]
		if x.Kind != y.Kind {
			return x.Kind < y.Kind
		}
		if x.ID != y.ID {
			return x.ID < y.ID
		}
		return firstPath(x) < firstPath(y)
	})
}

// firstPath returns a resolution's leading path, or "" when it has none.
func firstPath(r Resolution) string {
	if len(r.Paths) == 0 {
		return ""
	}
	return r.Paths[0]
}
