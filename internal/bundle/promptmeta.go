package bundle

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// PromptMeta is what a key was a question about.
//
// It is written beside the prompt when one is emitted, and read by admit. Without
// it a reply carries no way to say which source it concerns, and the alternatives
// were both worse: taking the model's word for its own source would let a reply
// cite a document it never read, and re-deriving the key from every record would
// need admit to be told the model the prompt was rendered for.
//
// It also settles a question admit could not otherwise answer: **whether the key
// names a prompt that was actually emitted.** A key with no meta is a reply to a
// question nobody asked, and caching one would leave an entry nothing will ever hit.
type PromptMeta struct {
	Key string `json:"key"`

	// URI is the source, and becomes the document's `sources[].resource`.
	URI string `json:"uri"`

	// SourceHash is the archived text's content hash.
	SourceHash string `json:"source_hash"`

	// ArchivePath is the tier-0 file the reply's quotations must be found in. It
	// is recorded here rather than searched for at admit time so the check runs
	// against the text the prompt was built from, not against whatever the archive
	// happens to hold later.
	ArchivePath string `json:"archive_path"`
}

// promptMetaPath is where a key's metadata lives, beside its prompt.
func promptMetaPath(key string) string {
	return filepath.ToSlash(filepath.Join(stateDir, promptDir, key+".json"))
}

// StorePromptMeta records what a prompt was about.
//
// Requires: meta.Key is set; the writer lock is held.
// Ensures: written atomically, so an interrupted ingest never leaves a meta file
// that describes half a prompt.
func StorePromptMeta(bundleDir string, meta *PromptMeta) error {
	const op = "bundle.StorePromptMeta"

	data, err := json.Marshal(meta)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	full := filepath.Join(bundleDir, filepath.FromSlash(promptMetaPath(meta.Key)))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, append(data, '\n'), 0o640); wErr != nil {
		return &errs.Error{Op: op, Err: wErr}
	}
	return nil
}

// LoadPromptMeta reads what a key was a question about.
//
// Requires: nothing.
// Ensures: ENOTFOUND when the key names no emitted prompt. That is a refusal
// rather than a default, and it is the check that stops a mistyped key filing a
// reply against a question nobody asked.
func LoadPromptMeta(bundleDir, key string) (PromptMeta, error) {
	const op = "bundle.LoadPromptMeta"

	data, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(promptMetaPath(key))))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return PromptMeta{}, &errs.Error{
				Code:    errs.ENOTFOUND,
				Message: op + ": no prompt was emitted for key " + key,
			}
		}
		return PromptMeta{}, &errs.Error{Op: op, Err: err}
	}
	var out PromptMeta
	if uErr := json.Unmarshal(data, &out); uErr != nil {
		return PromptMeta{}, &errs.Error{Code: errs.EINVALID, Op: op, Err: uErr}
	}
	return out, nil
}
