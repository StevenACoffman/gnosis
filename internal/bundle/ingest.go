package bundle

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// promptDir is where emitted prompts land, inside stateDir.
//
// Tier 3 like the cache, and for the same reason: a prompt is a question this
// user is asking, not a decision the corpus has made. It is written to disk rather
// than to stdout because ingest emits many at once and an agent needs to answer
// them one at a time.
const promptDir = "prompts"

// Pending is one prompt and whether its answer is already known.
type Pending struct {
	// Key is the §6.1 cache key.
	Key string `json:"key"`

	// URI is the source the prompt asks about.
	URI string `json:"uri"`

	// Path is where the prompt was written, or empty when it was not written
	// because the answer was already cached.
	Path string `json:"path,omitempty"`

	// Cached reports whether a reply is already stored for this key. A run in
	// which every prompt is cached makes no model calls at all, which is §6.1's
	// determinism win stated as a field a caller can count.
	Cached bool `json:"cached"`
}

// PromptOptions is what a caller is asking PromptsFor to do.
type PromptOptions struct {
	// Model is what will answer, and is part of every key.
	Model relay.Model

	// URIs are the sources to ask about.
	URIs []string

	// CacheOnly suppresses writing. §6.1 says --cache-only "refuses to emit any
	// prompt whose reply is not already cached", and refusing has to happen here
	// rather than in the report: a caller that learned about the misses only after
	// the prompts were on disk would find them already emitted.
	CacheOnly bool
}

// PromptsFor renders one extraction prompt per archived source and reports which
// already have answers.
//
// Requires: the writer lock is held, since prompts are written under .gnosis/.
// Ensures: one Pending per source, ordered by URI so two runs over one corpus
// produce the same sequence. A prompt whose reply is cached is **not written**:
// re-emitting a question that is already answered would invite an agent to answer
// it again, which is precisely the cost the cache exists to avoid. Under CacheOnly
// nothing is written at all, and the Pending values report what would have been.
//
// The prompt is built from the archived text, never from the live source. §4.1's
// argument applies directly: a prompt built from a page nobody kept would produce
// quotations nobody can verify, so every claim it yielded would be unverifiable by
// construction rather than by accident.
func PromptsFor(bundleDir string, opts *PromptOptions) ([]Pending, error) {
	const op = "bundle.PromptsFor"

	records, err := recordsByURI(op, bundleDir)
	if err != nil {
		return nil, err
	}

	out := make([]Pending, 0, len(opts.URIs))
	for _, uri := range opts.URIs {
		rec, ok := records[uri]
		if !ok {
			return nil, &errs.Error{
				Code:    errs.ENOTFOUND,
				Message: op + ": " + uri + " has no fetch record; run `gnosis fetch` first",
			}
		}
		pending, pErr := renderPending(op, bundleDir, opts, &rec)
		if pErr != nil {
			return nil, pErr
		}
		out = append(out, pending)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URI < out[j].URI })
	return out, nil
}

// renderPending renders and, when the answer is not already known, writes one
// prompt.
func renderPending(
	op, bundleDir string,
	opts *PromptOptions,
	rec *archive.Record,
) (Pending, error) {
	if !rec.Disposition.Durable() {
		// A referenced source has no archived text, so there is nothing to build a
		// prompt from. Refusing is honest: asking a model about a URI it cannot
		// read produces claims quoted from nothing.
		return Pending{}, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": " + rec.URI + " is " + string(rec.Disposition) +
				"; there is no archived text to extract from",
		}
	}
	text, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(rec.ArchivePath)))
	if err != nil {
		return Pending{}, &errs.Error{Op: op, Err: err}
	}

	prompt := relay.Render(&relay.Request{
		URI:        rec.URI,
		SourceHash: rec.SourceSHA256,
		Text:       string(text),
		Model:      opts.Model,
	})

	_, cached, err := LoadCached(bundleDir, prompt.Key)
	if err != nil {
		return Pending{}, err
	}
	pending := Pending{Key: prompt.Key, URI: prompt.URI, Cached: cached}
	if cached || opts.CacheOnly {
		return pending, nil
	}

	// The metadata is written before the prompt, so a crash between them leaves a
	// meta file describing a prompt that is not there — inert, and re-emitting
	// fixes it. The reverse would leave a prompt an agent could answer and admit
	// could not accept.
	meta := PromptMeta{
		Key:         prompt.Key,
		URI:         rec.URI,
		SourceHash:  rec.SourceSHA256,
		ArchivePath: rec.ArchivePath,
	}
	if err := StorePromptMeta(bundleDir, &meta); err != nil {
		return Pending{}, err
	}

	rel := filepath.ToSlash(filepath.Join(stateDir, promptDir, prompt.Key+".md"))
	full := filepath.Join(bundleDir, filepath.FromSlash(rel))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, []byte(prompt.Text), 0o640); wErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: wErr}
	}
	pending.Path = rel
	return pending, nil
}

// recordsByURI indexes tier 0 by source URI, keeping the durable record when a
// source has several.
//
// A source fetched twice has two records — one per version — and the caller wants
// the one with archived text. Where both are durable the choice is arbitrary and
// the effect is not: the prompt would differ, so the key would differ, so the
// cache would miss. The record with the lexically greatest hash wins, which is
// stable and is the whole requirement.
func recordsByURI(op, bundleDir string) (map[string]archive.Record, error) {
	out := map[string]archive.Record{}
	fsys := os.DirFS(bundleDir)

	err := walkRecords(op, fsys, func(rec *archive.Record) {
		prev, seen := out[rec.URI]
		switch {
		case !seen:
			out[rec.URI] = *rec
		case rec.Disposition.Durable() && !prev.Disposition.Durable():
			out[rec.URI] = *rec
		case rec.Disposition.Durable() == prev.Disposition.Durable() &&
			rec.SourceSHA256 > prev.SourceSHA256:
			out[rec.URI] = *rec
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
