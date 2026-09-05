package bundle

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// pairSide is one claim of a pair, with the archived text it rests on.
type pairSide struct {
	Ref         string
	Claim       relay.CriticClaim
	URI         string
	ArchivePath string
}

// PairPrompts emits one prompt per unseparated pair (§10.3).
//
// Requires: the writer holds the lock, because emitting a prompt writes one; the index
// and the corpus are readable.
// Ensures: one prompt per pair the residue reports and both of whose claims rest on
// archived text; pairs missing a source are counted as skipped rather than dropped
// silently.
//
// # This is the producer §10.3 was waiting for
//
// The section routes semantic contradiction to a cold critic, and Step 3.7 built the
// single-claim critique instead — "not because the prompt is hard but because nothing
// emits the pairs". The residue predicate now does, so this is the join: every pair it
// reports becomes a question a judge can answer.
//
// # A pair without two sources is skipped and counted
//
// Judging a claim against text nobody kept is what §4.1 refuses, so a claim citing no
// archived source cannot be one side of this. The count travels for `Critiqued.Skipped`'s
// reason: a corpus where most claims cite nothing would otherwise report a small number
// of prompts as a small number of disagreements.
func (w *Writer) PairPrompts(ctx context.Context, opts *CriticOptions) (*Critiqued, error) {
	const op = "bundle.Writer.PairPrompts"

	if err := w.held(op); err != nil {
		return nil, err
	}
	snap, err := w.snapshot(ctx, op)
	if err != nil {
		return nil, err
	}
	sources, err := sourceVersions(os.DirFS(w.dir))
	if err != nil {
		return nil, err
	}
	docs, err := Load(os.DirFS(w.dir))
	if err != nil {
		return nil, err
	}

	pairs := lint.Unseparated(snap)
	out := &Critiqued{Prompts: make([]Pending, 0, len(pairs)), Population: len(pairs)}
	for i := range pairs {
		sides, ok := pairSides(&pairs[i], docs, sources)
		if !ok {
			out.Skipped++
			continue
		}
		pending, pErr := w.pairPrompt(op, &pairs[i], &sides, opts)
		if pErr != nil {
			return nil, pErr
		}
		out.Prompts = append(out.Prompts, pending)
	}
	return out, nil
}

// snapshot builds the lint snapshot the residue predicate reads.
func (w *Writer) snapshot(ctx context.Context, op string) (*lint.Snapshot, error) {
	idx, err := LoadIndex(ctx, w.dir)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	fresh, err := LoadFreshness(w.dir)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	snap, err := Snapshot(os.DirFS(w.dir), idx, fresh)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return snap, nil
}

// pairSides gathers both claims and their sources, reporting whether both have one.
//
// The **first** archive path a claim names wins, which is `firstArchived`'s rule one file
// over: a claim resting on three sources could be judged three ways, and §10.3 asks about
// a pair of claims rather than a pair of claim-source combinations.
func pairSides(
	pair *lint.UnseparatedPair, docs []Document, sources map[string]lint.SourceVersion,
) ([2]pairSide, bool) {
	var out [2]pairSide
	for i, claim := range [2]lint.UnseparatedClaim{pair.First, pair.Second} {
		side, ok := sideFor(&claim, docs, sources)
		if !ok {
			return out, false
		}
		out[i] = side
	}
	return out, true
}

// sideFor finds one claim in the corpus and the archived text it rests on.
func sideFor(
	claim *lint.UnseparatedClaim, docs []Document, sources map[string]lint.SourceVersion,
) (pairSide, bool) {
	for i := range docs {
		if docs[i].ID != claim.DocID {
			continue
		}
		found := findClaim(&docs[i], claim.ClaimID)
		if found == nil {
			return pairSide{}, false
		}
		path, source, ok := firstArchived(found.ArchivePaths, sources)
		if !ok {
			return pairSide{}, false
		}
		return pairSide{
			Ref: claim.Ref(),
			Claim: relay.CriticClaim{
				Text: found.Anchor, Lead: found.Lead, Quotes: found.Quotes,
			},
			URI: source.URI, ArchivePath: path,
		}, true
	}
	return pairSide{}, false
}

// pairPrompt renders and stores one two-claim prompt.
func (w *Writer) pairPrompt(
	op string, pair *lint.UnseparatedPair, sides *[2]pairSide, opts *CriticOptions,
) (Pending, error) {
	var texts [2]string
	for i := range sides {
		raw, err := os.ReadFile(
			filepath.Join(w.dir, filepath.FromSlash(sides[i].ArchivePath)))
		if err != nil {
			return Pending{}, &errs.Error{Op: op, Err: err}
		}
		texts[i] = string(raw)
	}
	prompt := relay.RenderPairCritic(&relay.PairRequest{
		Claims:  [2]relay.CriticClaim{sides[0].Claim, sides[1].Claim},
		Sources: texts,
		URIs:    [2]string{sides[0].URI, sides[1].URI},
		Subject: string(pair.Subject),
		Model:   opts.Model,
	})

	_, cached, err := LoadCached(w.dir, prompt.Key)
	if err != nil {
		return Pending{}, err
	}
	if cached {
		return Pending{URI: prompt.URI, Key: prompt.Key, Cached: true}, nil
	}
	meta := PromptMeta{
		Key: prompt.Key, Kind: PromptCriticPair, URI: prompt.URI,
		ArchivePaths: []string{sides[0].ArchivePath, sides[1].ArchivePath},
		PairRefs:     []string{sides[0].Ref, sides[1].Ref},
		Model:        opts.Model.Name,
	}
	// Metadata before the prompt, as every other emitter does: a crash between them
	// leaves a meta describing a prompt that is not there, which is inert, where the
	// reverse leaves a prompt an agent can answer and nothing can accept.
	if mErr := w.StorePromptMeta(&meta); mErr != nil {
		return Pending{}, mErr
	}
	rel := promptPath(prompt.Key)
	full := filepath.Join(w.dir, filepath.FromSlash(rel))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, []byte(prompt.Text), 0o640); wErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: wErr}
	}
	// §6.4's miss, and this one is the clearest case in the log: §10.3 states outright
	// that no deterministic predicate can settle this, so the checks that ran and
	// decided nothing are named and the recurrence is not a backlog item.
	w.noteMiss(&Miss{
		Op: "pair-critique", Reason: gnosis.MissNoPredicate, Key: prompt.Key,
		ChecksRun: []string{"conflict"}, Candidate: pair.ID(), At: time.Now().UTC(),
	}, opts.Warn)
	return Pending{URI: prompt.URI, Key: prompt.Key, Path: rel}, nil
}

// filePairCritique records a verdict about two claims (§10.3).
//
// Requires: the writer holds the lock; meta is a validated pair prompt's metadata.
// Ensures: the reply is cached whatever happens next; **a coverage row is appended for
// each claim**; the findings name both. The prompt is spent.
//
// # Coverage for both, and the ledger's meaning is why
//
// §10.5's record says what was *looked at*. A pair critique looked at two claims, so a
// row for one of them would understate it — and the understatement is the expensive
// direction: a later single-claim critique would be told nobody had examined a claim this
// one read in full, and would re-ask a question already answered.
//
// The two rows carry the same key, which is what lets a reader see they came from one
// critique rather than from two.
func (w *Writer) filePairCritique(
	op, key string, src []byte, meta *PromptMeta, warn io.Writer,
) (*Verdict, error) {
	if sErr := w.StoreCached(&CachedReply{
		Key: key, URI: meta.URI, Model: meta.Model, Reply: string(src),
	}); sErr != nil {
		return nil, sErr
	}
	reply, err := relay.ParseCriticReply(src)
	if err != nil {
		// Wrapped so the operation is in the message; the EINVALID survives, which is
		// what lets the command report a malformed reply as the agent's to fix.
		return nil, &errs.Error{Op: op, Err: err}
	}

	for _, ref := range meta.PairRefs {
		if rErr := w.RecordCritique(&Critique{
			ClaimRef: ref, Key: key, Model: meta.Model, At: time.Now().UTC(),
			Examined: reply.Examined, NotExamined: reply.NotExamined,
		}); rErr != nil {
			return nil, rErr
		}
	}

	out := &Verdict{
		ClaimRef: strings.Join(meta.PairRefs, " vs "),
		Examined: reply.Examined, NotExamined: reply.NotExamined,
		Findings: make([]finding.Diagnostic, 0, len(reply.Findings)),
	}
	for _, f := range reply.Findings {
		out.Findings = append(out.Findings, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: f.Category,
			Message:  strings.Join(meta.PairRefs, " vs ") + ": " + f.Message,
			// A human's, because §10.3 routed this here precisely because no
			// predicate can settle it — and §10.2.2 keeps a derived comparison a
			// finding rather than a verdict.
			Action: finding.ActionHuman,
		})
	}
	w.spendAnswered(key, "verdict", warn)
	return out, nil
}
