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

	// PromptCritic asks a cold critic whether a claim's source supports it (§10.5).
	// The reply becomes findings a caller reads and a coverage row a later prompt
	// reads; nothing it says reaches the corpus.
	//
	// **Its own kind rather than a mode of the others**, for PromptSynthesize's
	// reason one constant up: the three that exist all end in a document, and this
	// one ends in an opinion. A single kind with a mode would give two contracts one
	// name, and the reply that arrives is the one thing neither can re-ask about.
	PromptCritic PromptKind = "critic"

	// PromptAsk asks for an answer to one question, assembled from the claims
	// retrieved for it (§8.3). The reply becomes a candidate concept that `gnosis
	// file` puts through the promote gate; nothing it says reaches the corpus
	// unreviewed.
	//
	// **It is the one kind that names no archived source, and that is what it means.**
	// The other four are about text somebody fetched; this one is about claims the
	// corpus already holds, whose own evidence was checked when they were admitted. A
	// caller validating it against an archive path would be asking the wrong question
	// of it.
	PromptAsk PromptKind = "ask"
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

	// Model is what the prompt was rendered for.
	//
	// It is already inside the key, and it is recorded here because a key is opaque:
	// a coverage row saying which model covered an angle is the difference between
	// "somebody looked" and "this model looked", and §10.5's ledger is most useful
	// exactly where two models covered different ground. `CachedReply` repeats it
	// for the same reason one field over.
	Model string `json:"model,omitempty"`

	// DocumentID is the identifier of the document a critic prompt asks about.
	//
	// **The identifier rather than the path**, because the coverage ledger is keyed
	// on it: a ledger keyed by path loses a claim's steering history the moment
	// somebody retitles the concept, since §5.1.1 changes the slug. It is §5.4's
	// rule about durable edges applied one tier down, to derived state that still
	// wants to survive a rename. DocumentPath is kept beside it because a reader
	// needs one, which §5.6 makes a view rather than a second address.
	DocumentID string `json:"document_id,omitempty"`

	// ClaimID is the claim a critic prompt asks about, within DocumentPath. Empty for
	// every other kind.
	//
	// A critique is about one assertion, not a page: §10.5 samples claims, and a
	// coverage row keyed by document would tell a later critic that a page had been
	// looked at when one sentence on it had.
	ClaimID string `json:"claim_id,omitempty"`

	// Cites are the claim references an ask prompt offered, and therefore the whole
	// set an answer to it may cite. Empty for every other kind.
	//
	// **Stored because the check cannot be made without it.** A citation naming a
	// claim the prompt never carried is the failure this relay is arranged to catch,
	// and by the time the reply arrives the retrieval that produced the prompt is
	// gone: re-running the query would compare the answer against a *different* set,
	// which is not the same check and would pass an answer resting on claims nobody
	// showed the model.
	Cites []string `json:"cites,omitempty"`

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
	case PromptCritic:
		bad = append(bad, m.criticProblems()...)
	case PromptAsk:
		bad = append(bad, m.askProblems()...)
	case PromptKindUnset:
		bad = append(bad, "kind is unset; a prompt nobody characterised cannot be "+
			"answered, and defaulting one would file a rewrite as a new document")
	default:
		bad = append(bad, "kind "+string(m.Kind)+
			" is not source, accrete, synthesize, critic or ask")
	}
	return bad
}

// askProblems names what an answer prompt is missing.
//
// Its own function rather than a sixth arm with a body, which is `criticProblems`'s
// reason and the same linter finding that produced it: a switch whose arms carry logic
// is one whose complexity grows with the vocabulary.
//
// **The claims are the whole requirement, and the absence of the others is the point.**
// An answer prompt rests on claims that were gated when they were admitted, so there is
// no archived source for this meta to point at — inventing a requirement to make this
// arm look like its neighbours would refuse a well-formed prompt.
func (m *PromptMeta) askProblems() []string {
	if len(m.Cites) == 0 {
		return []string{"an answer prompt names no claims, so every citation in a " +
			"reply to it would be one the prompt did not carry"}
	}
	return nil
}

// criticProblems names what a critic prompt is missing.
//
// Its own function rather than a fourth arm of the switch, which the complexity limit
// asked for and reading it agrees with: a critic prompt needs everything a concept
// prompt needs *and* the claim, and stating that as composition says it once.
func (m *PromptMeta) criticProblems() []string {
	var bad []string
	if len(m.ArchivePaths) == 0 && strings.TrimSpace(m.ArchivePath) == "" {
		bad = append(bad, "a critic prompt names no archived source, so the claim "+
			"would be judged against nothing")
	}
	if strings.TrimSpace(m.ClaimID) == "" {
		bad = append(bad, "a critic prompt names no claim; a verdict about a whole "+
			"page cannot be filed against the assertion it concerns")
	}
	if strings.TrimSpace(m.DocumentID) == "" {
		bad = append(bad, "a critic prompt names no document identifier, so its "+
			"coverage could only be keyed by a path that changes on a retitle")
	}
	return append(bad, m.documentProblems()...)
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
