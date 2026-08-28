package bundle

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// The prompt kinds. PromptKindUnset is the zero value and names nothing, so a meta
// file nobody characterised is refused rather than read as the commoner case.
const (
	PromptKindUnset PromptKind = ""

	// PromptSource asks a model to extract claims from an archived source. The
	// reply becomes a new quarantined document.
	PromptSource PromptKind = "source"

	// PromptAccrete asks for more evidence for a concept that already exists. The
	// reply's quotations are appended to the claims that document already makes,
	// and its body does not change (§6.3).
	PromptAccrete PromptKind = "accrete"

	// PromptSynthesize asks for a concept's body to be rewritten. The reply
	// replaces the document, and is gated on every prior quotation still
	// validating and no evidence entry being dropped (§6.3).
	//
	// **Separate from PromptAccrete rather than a flag beside it.** Both ask about
	// a document and there the resemblance stops: one may not change a body and the
	// other exists to. A single kind with a mode would give two contracts one name,
	// and the reply that arrives is the one thing neither can re-ask about.
	PromptSynthesize PromptKind = "synthesize"
)

// PromptKind is what a prompt asks about.
type PromptKind string

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

	// Kind is what this prompt is a question about.
	//
	// The zero value is **rejected** rather than defaulted to a source. A meta file
	// written before this field existed, or by a caller that forgot it, describes a
	// prompt nobody characterised — and defaulting would let a reply about an
	// existing concept be admitted as a new document, which is the one mistake this
	// field exists to prevent.
	Kind PromptKind `json:"kind"`

	// URI is the source, and becomes the document's `sources[].resource`.
	URI string `json:"uri"`

	// SourceHash is the archived text's content hash.
	SourceHash string `json:"source_hash"`

	// ArchivePath is the tier-0 file the reply's quotations must be found in. It
	// is recorded here rather than searched for at admit time so the check runs
	// against the text the prompt was built from, not against whatever the archive
	// happens to hold later.
	ArchivePath string `json:"archive_path"`

	// ArchivePaths are the tier-0 files a rewrite's quotations must be found in.
	//
	// **A list because a concept rests on more than one source**, which a source
	// prompt never does. A rewrite of a page citing three documents must be allowed
	// to quote any of them, and checking against only the first would refuse
	// evidence the corpus already holds — the gate reporting a loss it caused.
	ArchivePaths []string `json:"archive_paths,omitempty"`

	// DocumentPath is the concept this prompt asks about, bundle-relative. Empty
	// for a source prompt.
	DocumentPath string `json:"document_path,omitempty"`

	// DocumentHash is that document's content hash when the prompt was emitted.
	//
	// **It is why this is not merely a path.** Between emitting a prompt about a
	// concept and admitting the reply, the concept can change — somebody edits it,
	// or another reply lands. Applying an answer computed against bytes that are
	// gone is §9.4's approved-diff window one level up, and comparing the hash is
	// what closes it: the same expected-revision check §4.6.2 requires of any write
	// that spans two round trips.
	DocumentHash string `json:"document_hash,omitempty"`
}

// Valid reports why this meta is not usable as written, or nil.
//
// Requires: nothing; a zero PromptMeta is valid input and is rejected.
// Ensures: every problem is named at once. Pure.
func (m *PromptMeta) Valid() error {
	const op = "bundle.PromptMeta.Valid"

	var bad []string
	if strings.TrimSpace(m.Key) == "" {
		bad = append(bad, "key is empty")
	}
	bad = append(bad, m.kindProblems()...)
	if len(bad) == 0 {
		return nil
	}
	return &errs.Error{
		Code: errs.EINVALID, Op: op, Message: op + ": " + strings.Join(bad, "; "),
	}
}

// Archives are the tier-0 files this prompt's quotations must be found in.
//
// Requires: nothing.
// Ensures: never nil for a valid meta. A source prompt has exactly one; a rewrite has
// however many the document cites. Pure.
func (m *PromptMeta) Archives() []string {
	if len(m.ArchivePaths) > 0 {
		return m.ArchivePaths
	}
	if m.ArchivePath == "" {
		return nil
	}
	return []string{m.ArchivePath}
}

// kindProblems names what this kind of prompt is missing.
//
// Requires: nothing.
// Ensures: every problem at once, so one round trip tells a caller everything. Pure.
func (m *PromptMeta) kindProblems() []string {
	var bad []string
	switch m.Kind {
	case PromptSource:
		if strings.TrimSpace(m.ArchivePath) == "" {
			bad = append(bad, "a source prompt names no archive path")
		}
	case PromptSynthesize:
		if len(m.ArchivePaths) == 0 {
			bad = append(bad, "a rewrite prompt names no archived sources, so its "+
				"quotations could be checked against nothing")
		}
		bad = append(bad, m.documentProblems()...)
	case PromptAccrete:
		bad = append(bad, m.documentProblems()...)
	case PromptKindUnset:
		bad = append(bad, "kind is unset; a prompt nobody characterised cannot be "+
			"answered, and defaulting one would file a rewrite as a new document")
	default:
		bad = append(bad, "kind "+string(m.Kind)+" is not source, accrete or synthesize")
	}
	return bad
}

// documentProblems names what a concept-bound prompt is missing.
func (m *PromptMeta) documentProblems() []string {
	var bad []string
	if strings.TrimSpace(m.DocumentPath) == "" {
		bad = append(bad, "a concept prompt names no document")
	}
	if strings.TrimSpace(m.DocumentHash) == "" {
		bad = append(bad, "a concept prompt records no document hash, so a reply "+
			"could be applied to bytes it was not computed against")
	}
	return bad
}

// promptMetaPath is where a key's metadata lives, beside its prompt.
func promptMetaPath(key string) string {
	return filepath.ToSlash(filepath.Join(stateDir, promptDir, key+".json"))
}

// StorePromptMeta records what a prompt was about.
//
// Requires: meta.Key is set.
// Ensures: written atomically, so an interrupted ingest never leaves a meta file
// that describes half a prompt.
func (w *Writer) StorePromptMeta(meta *PromptMeta) error {
	const op = "bundle.Writer.StorePromptMeta"

	if err := w.held(op); err != nil {
		return err
	}
	// Validated on the way in rather than on the way out. A meta file that cannot be
	// answered is a prompt whose reply has nowhere to go, and finding that at admit
	// time means the model has already been paid to answer it.
	if err := meta.Valid(); err != nil {
		return err
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	full := filepath.Join(w.dir, filepath.FromSlash(promptMetaPath(meta.Key)))
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
