package bundle

import "time"

// This file holds the corpus's outward projection — what a concept looks like to a tool
// that is not gnosis. It is separate from `portable.go`, which answers a different
// question: which bytes may leave at all.

// ExportClaim is one claim as an exporting reader needs it: what it asserts and where
// the passages backing it live.
//
// The archive paths travel because they are what makes an exported claim checkable. A
// stream carrying assertions and no addresses would hand a receiver a set of things to
// believe, which is the shape §1.1 rejects.
type ExportClaim struct {
	ID           string   `json:"id"`
	Anchor       string   `json:"anchor"`
	Lead         string   `json:"lead,omitempty"`
	ArchivePaths []string `json:"archive_paths,omitempty"`
}

// ExportDoc is one concept in the JSONL export.
//
// **It is a projection and never a second copy of the corpus.** The markdown is the
// document; this is a rendering of it for a tool that does not want to parse OKF, and
// every field here is derived from bytes that ship alongside it in the same export. A
// receiver who distrusts the projection can read the file, which is the property that
// keeps this from becoming a representation that can silently drift.
type ExportDoc struct {
	Path   string `json:"path"`
	ID     string `json:"gnosis_id"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status string `json:"status,omitempty"`

	// Hash is the document's content hash, so a receiver can tell whether the file
	// beside this row is the one the row was made from.
	Hash  string `json:"sha256"`
	Bytes int    `json:"bytes"`

	Body        string        `json:"body"`
	Claims      []ExportClaim `json:"claims,omitempty"`
	Sources     []string      `json:"sources,omitempty"`
	Limitations []string      `json:"limitations,omitempty"`

	// StaleAfter is the date the author asked for this to be revisited by, in OKF's
	// form, or empty when they declared none. A receiver reading an exported corpus a
	// year later needs to know which parts were expected to have aged.
	StaleAfter string `json:"stale_after,omitempty"`

	// Invalid is why a document could not be read, when it could not be.
	//
	// **Exported rather than skipped**, which is the same rule `lint` follows: a
	// corpus that quietly dropped its unreadable documents would hand a receiver a
	// clean-looking export and no way to learn that something was missing from it.
	Invalid string `json:"invalid,omitempty"`
}

// ExportRow projects one loaded document for the JSONL export.
//
// Requires: doc came from Load.
// Ensures: a row carrying what a receiver needs to read the concept without parsing OKF.
// Pure — it reads no file and mutates nothing it was handed.
//
// **One document rather than the whole corpus**, so the caller streams: a row carries the
// body, the slice of them is a second copy of every document in memory, and a linter
// counting the bytes each loop iteration copied is what pointed at it.
func ExportRow(doc *Document) ExportDoc {
	row := ExportDoc{
		Path:        doc.Path,
		ID:          doc.ID.String(),
		Type:        string(doc.Type),
		Title:       doc.Title,
		Status:      doc.Status,
		Hash:        doc.Hash,
		Bytes:       doc.Bytes,
		Body:        doc.Body,
		Sources:     doc.Resources,
		Limitations: doc.Limitations,
	}
	if !doc.StaleAfter.IsZero() {
		row.StaleAfter = doc.StaleAfter.Format(time.DateOnly)
	}
	if doc.Invalid != nil {
		row.Invalid = doc.Invalid.Error()
	}
	for i := range doc.Claims {
		claim := &doc.Claims[i]
		row.Claims = append(row.Claims, ExportClaim{
			ID:           claim.ID,
			Anchor:       claim.Anchor,
			Lead:         claim.Lead,
			ArchivePaths: claim.ArchivePaths,
		})
	}
	return row
}
