package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"

	"github.com/StevenACoffman/skillet/errs"
)

// FetchDir is where fetch records live, relative to the bundle root.
const FetchDir = "evidence/fetch"

// TextDir is where archived text lives, relative to the bundle root.
const TextDir = "evidence/text"

// Record is one immutable ledger entry: what was fetched, and what became of it.
//
// There is deliberately no timestamp. See the package comment — content-addressing
// over the source bytes rather than over the fetch event is what makes a re-fetch
// of unchanged bytes a no-op on disk, and it is why a weekly staleness sweep does
// not deposit tens of thousands of near-identical records a year in the tier whose
// purpose is evidence.
//
// Field order is part of the format: it fixes the canonical encoding, and the
// canonical encoding fixes the filename. Reordering these fields renames every
// record in every corpus.
type Record struct {
	URI          string      `json:"uri"`
	SourceSHA256 string      `json:"source_sha256"`
	ByteSize     int64       `json:"byte_size"`
	MediaType    string      `json:"media_type,omitempty"`
	Disposition  Disposition `json:"disposition"`

	// ArchivePath is where the stored text lives, empty for Referenced.
	ArchivePath string `json:"archive_path,omitempty"`

	// Extractor and ExtractorVersion identify what produced the stored text, and
	// are set only for Extracted.
	Extractor        string `json:"extractor,omitempty"`
	ExtractorVersion string `json:"extractor_version,omitempty"`

	// ExtractedFrom is the source hash the extraction came from, so the chain
	// from stored text back to fetched bytes is in the record rather than
	// inferred.
	ExtractedFrom string `json:"extracted_from,omitempty"`

	// RejectReason says why the source was not archived directly, and is set
	// exactly for Referenced.
	RejectReason RejectReason `json:"reject_reason,omitempty"`
}

// Canonical is the record's bytes: compact JSON with a trailing newline.
//
// Requires: nothing.
// Ensures: deterministic — the same record yields the same bytes on any machine,
// which is what lets two users independently fetch one source and produce a record
// that merges rather than conflicts (§4.6.1). These are both the bytes hashed and
// the bytes written, so sha256 of the file on disk equals its own filename and a
// reader can check a record with shasum and nothing else.
//
// It is one line rather than indented. Diffability was the competing concern and
// it does not apply: one file per record means a change is a whole-file add, never
// a hunk.
func (r *Record) Canonical() ([]byte, error) {
	const op = "archive.Record.Canonical"

	// A struct with only strings, ints, and a named string type cannot fail to
	// marshal; the error is returned rather than dropped because a silent empty
	// record would be indistinguishable from an empty source.
	b, err := json.Marshal(r)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return append(b, '\n'), nil
}

// Hash is the record's identity: sha256 over its canonical bytes.
func (r *Record) Hash() (string, error) {
	b, err := r.Canonical()
	if err != nil {
		return "", err
	}
	return hashHex(b), nil
}

// Path is where the record belongs, relative to the bundle root.
//
// Requires: nothing.
// Ensures: evidence/fetch/<h[:2]>/<h>.json. The two-character shard keeps any one
// directory small enough that a corpus of a hundred thousand records still lists.
func (r *Record) Path() (string, error) {
	h, err := r.Hash()
	if err != nil {
		return "", err
	}
	return path.Join(FetchDir, h[:2], h+".json"), nil
}

// TextPath is where archived text with the given content hash and extension
// belongs, relative to the bundle root.
//
// Requires: sha is a full hex sha256; ext includes its leading dot or is empty.
// Ensures: evidence/text/<sha[:2]>/<sha><ext>, sharded as fetch records are.
func TextPath(sha, ext string) string {
	return path.Join(TextDir, sha[:2], sha+ext)
}

// hashHex is the one hash used for content addressing here, so no two call sites
// can disagree about what a content hash is.
func hashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
