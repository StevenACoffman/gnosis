package bundle

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/textnorm"
)

// RewriteOptions asks for a rewrite prompt for one concept.
type RewriteOptions struct {
	// Model is what will answer, and is part of every key.
	Model relay.Model

	// Path is the concept to rewrite, bundle-relative.
	Path string
}

// RewritePending is a rewrite prompt that was emitted, or found already answered.
type RewritePending struct {
	Key    string
	Prompt string
	Cached bool
}

// Synthesis is what rewriting a concept's body would do, and why it may not.
//
// §6.3 makes synthesis the *gated* half of accretion: rewriting a body "is where a
// model silently drops what it did not think important", so the gate is stated over
// **quotations** rather than over paragraphs. A rewrite may say anything it likes as
// long as it keeps the evidence the corpus already held.
type Synthesis struct {
	// Path is the document, bundle-relative.
	Path string `json:"path"`

	// Content is the document to write. Empty when the gate refused.
	Content []byte `json:"-"`

	// Dropped are quotations the document held and the rewrite does not.
	//
	// **The finding that justifies the gate.** A model rewriting a page will
	// paraphrase, reorder, and merge claims, all of which are fine; losing the
	// passage that made a claim checkable is not, and it is invisible in a diff a
	// reader skims because what went missing is in frontmatter rather than prose.
	Dropped []string `json:"dropped,omitempty"`

	// Unvalidated are quotations the rewrite offers that the archive does not
	// support. They are the ordinary evidence invariant, reported here so one
	// refusal reports both kinds rather than the first kind twice.
	Unvalidated []string `json:"unvalidated,omitempty"`

	// Diff is what would change, reported before any write (§6.3).
	Diff string `json:"diff,omitempty"`
}

// Approved reports whether the rewrite may be written.
func (s *Synthesis) Approved() bool {
	return len(s.Dropped) == 0 && len(s.Unvalidated) == 0 && len(s.Content) > 0
}

// Synthesize plans a concept's rewrite and applies §6.3's gate to it.
//
// Requires: existing is the parsed document; reply is the rewrite; supported names the
// quotations the archive was found to support, fold-normalised.
// Ensures: Content is empty unless every prior quotation survives and every offered
// quotation validates. Pure — no I/O, no clock.
//
// **The evidence comparison is a set, not a count.** A rewrite that dropped one
// quotation and added another would balance, and the arithmetic would approve a
// document that lost the passage a claim rested on.
func Synthesize(
	existing *okf.Document, id gnosis.ID, reply *relay.Reply, supported map[string]bool,
) (*Synthesis, error) {
	const op = "bundle.Synthesize"

	if reply.Title == "" || reply.Type == "" {
		return nil, &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": a rewrite must carry a title and a type",
		}
	}
	before := quoteSetOf(claimsOf(existing))
	out := &Synthesis{}

	after := map[string]string{}
	for i := range reply.Claims {
		for _, q := range reply.Claims[i].Quotes {
			folded := textnorm.Fold(q)
			after[folded] = q
			if !supported[folded] {
				out.Unvalidated = append(out.Unvalidated, q)
			}
		}
	}
	for folded, raw := range before {
		if _, kept := after[folded]; !kept {
			out.Dropped = append(out.Dropped, raw)
		}
	}
	sort.Strings(out.Dropped)
	sort.Strings(out.Unvalidated)

	content := renderRewrite(existing, id, reply)
	out.Diff = summariseRewrite(existing, reply)
	if len(out.Dropped) > 0 || len(out.Unvalidated) > 0 {
		// Content stays empty on a refusal, so a caller cannot write a document the
		// gate declined by reading past the verdict.
		return out, nil
	}
	out.Content = content
	return out, nil
}

// quoteSetOf indexes a document's existing quotations by their folded form.
//
// Folded because the evidence invariant is: a passage re-offered with different
// whitespace is the same passage, and a rewrite that reflowed one has not dropped it.
func quoteSetOf(claims []gate.Claim) map[string]string {
	out := map[string]string{}
	for i := range claims {
		for _, q := range claims[i].Quotes {
			out[textnorm.Fold(q)] = q
		}
	}
	return out
}

// renderRewrite builds the document a rewrite would produce.
//
// The identity, and the sources the document already declared, are carried forward:
// §5.1 assigns identity once and never rewrites it — "not on move, not on retitle, not
// when a body is replaced" — and a rewrite that minted a new id would make the corpus
// unable to answer what it believed last month.
func renderRewrite(existing *okf.Document, id gnosis.ID, reply *relay.Reply) []byte {
	doc := conceptDoc{
		Type: reply.Type, Title: reply.Title, ID: id,
		SourceURI: mergedSources(existing, reply.SourceURI),
		Claims:    make([]conceptClaim, 0, len(reply.Claims)),
	}
	for i := range reply.Claims {
		c := &reply.Claims[i]
		doc.Claims = append(doc.Claims, conceptClaim{
			ID: c.ID, Anchor: c.Text, Lead: c.Lead,
			Quotes: c.Quotes, ArchivePaths: c.ArchivePaths,
		})
		doc.Paragraphs = append(doc.Paragraphs, c.Text)
	}
	return renderConcept(&doc)
}

// summariseRewrite says what would change, for a reader deciding whether to apply it.
//
// Counts and titles rather than a line diff: §6.3 asks for "the diff being reported
// before the write", and what a reader of a rewritten concept needs first is whether
// the page still says the same number of things about the same subject. `git diff`
// answers the line question better than anything written here would.
func summariseRewrite(existing *okf.Document, reply *relay.Reply) string {
	title, _ := existing.Text("title")
	var b strings.Builder
	if title != reply.Title {
		b.WriteString("title: " + title + " → " + reply.Title + "\n")
	}
	b.WriteString("claims: " + strconv.Itoa(len(claimsOf(existing))) + " → " +
		strconv.Itoa(len(reply.Claims)) + "\n")
	return b.String()
}

// RewritePrompt emits a gated rewrite prompt for one concept (§6.3).
//
// Requires: the writer holds the lock; opts.Path names a concept in this bundle.
// Ensures: the document's hash is recorded with the prompt, so a reply computed against
// bytes that have since moved is refused rather than applied. A question already
// answered writes nothing — re-emitting it would invite an agent to answer it again,
// which is the cost the cache exists to avoid.
//
// **The prompt is built from the document and its archived sources**, never from the
// live web: §4.1's argument applies unchanged, and a rewrite quoting a page nobody kept
// would produce claims unverifiable by construction.
func (w *Writer) RewritePrompt(opts *RewriteOptions) (RewritePending, error) {
	const op = "bundle.Writer.RewritePrompt"

	if err := w.held(op); err != nil {
		return RewritePending{}, err
	}
	doc, hash, sources, err := w.rewritable(op, opts.Path)
	if err != nil {
		return RewritePending{}, err
	}

	archives, err := archivesFor(op, w.dir, sources)
	if err != nil {
		return RewritePending{}, err
	}
	prompt := relay.Render(&relay.Request{
		URI: sources[0], SourceHash: hash, Text: doc.Body, Model: opts.Model,
	})
	meta := PromptMeta{
		Key: prompt.Key, Kind: PromptSynthesize,
		URI: sources[0], SourceHash: hash, ArchivePaths: archives,
		DocumentPath: opts.Path, DocumentHash: hash,
	}
	_, cached, err := LoadCached(w.dir, prompt.Key)
	if err != nil {
		return RewritePending{}, err
	}
	if cached {
		return RewritePending{Key: prompt.Key, Cached: true}, nil
	}

	// Metadata before the prompt, as ingest does and for the same reason: a crash
	// between them leaves a meta file describing a prompt that is not there, which
	// is inert. The reverse leaves a prompt an agent can answer and admit cannot
	// accept.
	if err := w.StorePromptMeta(&meta); err != nil {
		return RewritePending{}, err
	}
	rel := promptPath(prompt.Key)
	full := filepath.Join(w.dir, filepath.FromSlash(rel))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return RewritePending{}, &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, []byte(prompt.Text), 0o640); wErr != nil {
		return RewritePending{}, &errs.Error{Op: op, Err: wErr}
	}
	return RewritePending{Key: prompt.Key, Prompt: rel}, nil
}

// rewritable reads a concept and reports why it cannot be rewritten, or its sources.
//
// Requires: rel is bundle-relative.
// Ensures: the hash is of the bytes read, so a reply can be refused when the document
// moves under it. A document declaring no sources is refused here rather than at admit
// time: a rewrite of it could be checked against nothing, and finding that out after a
// model has answered costs the answer.
func (w *Writer) rewritable(op, rel string) (*okf.Document, string, []string, error) {
	hash, err := w.conceptHash(op, rel)
	if err != nil {
		return nil, "", nil, err
	}
	raw, err := os.ReadFile(filepath.Join(w.dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, "", nil, &errs.Error{Op: op, Err: err}
	}
	doc, err := okf.Parse(raw)
	if err != nil {
		return nil, "", nil, &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	sources := existingSources(doc)
	if len(sources) == 0 {
		return nil, "", nil, &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": " + rel + " declares no sources, so a rewrite would " +
				"have nothing to keep its quotations against",
		}
	}
	return doc, hash, sources, nil
}

// archivesFor resolves a document's declared sources to the tier-0 files holding them.
//
// Requires: uris are `sources[].resource` values.
// Ensures: one archive path per source that has durable archived text, in the order the
// document declares them. A source with none is skipped rather than refused — §4.3
// permits a referenced source, and a document citing one is not thereby unrewritable —
// but a document with *no* archived source at all is refused by the caller, because its
// rewrite could be checked against nothing.
func archivesFor(op, dir string, uris []string) ([]string, error) {
	records, err := recordsByURI(op, dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(uris))
	for _, uri := range uris {
		rec, ok := records[uri]
		if ok && rec.Disposition.Durable() && rec.ArchivePath != "" {
			out = append(out, rec.ArchivePath)
		}
	}
	if len(out) == 0 {
		return nil, &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": none of this document's sources has archived text; " +
				"run `gnosis fetch` on them before rewriting it",
		}
	}
	return out, nil
}
