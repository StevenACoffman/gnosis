package bundle

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// CriticOptions asks for cold-critic prompts (§10.5).
type CriticOptions struct {
	// Model is what will answer, and is part of every key.
	Model relay.Model

	// Path restricts the critique to one document's claims, bundle-relative. Empty
	// critiques a sample drawn from the whole corpus.
	Path string

	// SampleN is how many claims to draw. Zero takes the default from
	// standards/sample.toml, which is where §10.5's five and its reasoning live.
	//
	// It is ignored when Path is set: a caller naming a document has already chosen
	// the population, and sampling within it would answer a question nobody asked.
	SampleN int

	// Warn is where a note that is not a failure goes, or nil to discard one.
	//
	// The same channel `Coordinator.Warn` is, for the same reason: a note that exists
	// only in a JSON field is one nobody in a terminal reads.
	Warn io.Writer
}

// Critiqued is what a critic run emitted, and what it could not.
//
// **Skipped is part of the answer rather than a statistic beside it**, which is the rule
// `search --provable` and `durability` already follow: a corpus where most claims cite
// nothing would otherwise report a small sample as a small corpus. A caller told "5
// prompts" and not told "40 claims cite no archived source" would conclude the critic
// had covered the place.
type Critiqued struct {
	// Prompts are the prompts written, or found already answered.
	//
	// The field is `prompts` on the wire because `ingest`'s payload already calls
	// this list that, and a reader of the envelope should not have to learn a second
	// word for one thing — nor should a test helper that walks it.
	Prompts []Pending `json:"prompts"`

	// Skipped is how many claims were excluded because they cite no archived source.
	Skipped int `json:"skipped"`

	// Population is how many claims could have been drawn from, so a sample can be
	// read as a fraction of something rather than as a number.
	Population int `json:"population"`

	// Seed is the draw's seed, present only for a sample. §6.2.1 requires the
	// specific draw to be inspectable, and a sample whose seed is not reported is
	// reproducible in principle and not in practice.
	Seed uint64 `json:"seed,omitempty"`
}

// Verdict is one filed critique: what the critic found, and what it covered.
type Verdict struct {
	// ClaimRef is the claim judged, by identifier — the address the coverage ledger is
	// keyed on, which survives a retitle.
	ClaimRef string `json:"claim_ref"`

	// Path is the document as a reader navigates to it, which §5.6 makes a view
	// computed from the corpus rather than a second address. It is carried so a person
	// reading a verdict is not handed a UUID and asked to find the page.
	Path string `json:"path"`

	// Findings are the critic's verdicts, every one a warning.
	//
	// **The severity is gnosis's and not the model's** (§10.5). A critic that could
	// return an error severity would be a model gating the corpus, which §9.5.1
	// refuses on the promotion path for the same reason — and `relay.CriticFinding`
	// has no severity field, so there is nowhere for a reply to ask.
	Findings []finding.Diagnostic `json:"findings"`

	// Examined and NotExamined are what this critique covered, as recorded — the
	// second carrying the reason that says whether a better excerpt would close the
	// gap (§16.1's family type).
	Examined    []string             `json:"examined"`
	NotExamined []finding.Unexamined `json:"not_examined,omitempty"`

	// Moved reports that the document changed after the prompt was emitted.
	//
	// A warning rather than a refusal, and the contrast with a rewrite is the reason:
	// `synthesize` applies a reply to the bytes it was computed against, so stale
	// bytes corrupt the result. A critique is applied to nothing — it is an opinion
	// about text that has since changed, which is worth saying and not worth
	// discarding.
	Moved bool `json:"moved,omitempty"`
}

// criticTarget is one claim a cold critic could judge, with everything the prompt needs.
//
// It is assembled once and carried, rather than each field being looked up again at
// render time: the population is walked to decide what is critiquable, and re-deriving
// the same joins per drawn claim would be two answers to one question.
type criticTarget struct {
	// Ref addresses the claim across the corpus (§10.5's sample is over claims, not
	// documents).
	Ref string

	// ID and Path are the document's durable address and the view of it (§5.6): the
	// coverage ledger is keyed on the first and a reader is shown the second.
	ID           gnosis.ID
	Path         string
	DocumentHash string
	ClaimID      string

	// Text, Lead and Quotes are the blinded view — exactly what relay.CriticClaim
	// holds, and nothing a §10.3 verdict could echo.
	Text   string
	Lead   string
	Quotes []string

	// URI, SourceHash and ArchivePath name the archived source the claim is judged
	// against.
	URI         string
	SourceHash  string
	ArchivePath string
}

// CriticPrompts emits cold-critic prompts for a claim, a document, or a sample (§10.5).
//
// Requires: the writer holds the lock; opts.Model names what will answer.
// Ensures: one prompt per drawn claim, with its metadata beside it, and a count of the
// claims excluded for citing no archived source. A question already answered writes
// nothing — re-emitting it would invite an agent to answer it again, which is the cost
// the cache exists to avoid.
//
// **The population is claims carrying a quotation whose archive path resolves.** A
// critic judges whether the source supports the claim (§17.1), and a claim with no
// source has nothing to be judged against; asking anyway would produce an opinion about
// the claim's plausibility, which is the reasoning-without-evidence this corpus is built
// to refuse.
func (w *Writer) CriticPrompts(opts *CriticOptions) (*Critiqued, error) {
	const op = "bundle.Writer.CriticPrompts"

	if err := w.held(op); err != nil {
		return nil, err
	}
	docs, err := Load(os.DirFS(w.dir))
	if err != nil {
		return nil, err
	}
	sources, err := sourceVersions(os.DirFS(w.dir))
	if err != nil {
		return nil, err
	}

	population, skipped := critiquable(docs, opts.Path, sources)
	drawn, seed, err := w.draw(op, population, opts)
	if err != nil {
		return nil, err
	}

	out := &Critiqued{
		Prompts: make([]Pending, 0, len(drawn)),
		Skipped: skipped, Population: len(population), Seed: seed,
	}
	critiques, err := LoadCritiques(w.dir)
	if err != nil {
		return nil, err
	}
	for i := range drawn {
		pending, pErr := w.criticPrompt(op, &drawn[i], critiques, opts)
		if pErr != nil {
			return nil, pErr
		}
		out.Prompts = append(out.Prompts, pending)
	}
	return out, nil
}

// FileCritique records a cold critic's verdict (§10.5).
//
// Requires: the writer holds the lock; key names an emitted critic prompt.
// Ensures: the reply is cached whatever happens next, because a model call already
// happened; the coverage row is appended; the prompt and its metadata are spent. The
// findings are returned and **not stored** — nothing reads a stored critic verdict, and
// §10.5 names a reader only for the coverage.
//
// ENOTFOUND when the key names no emitted prompt, EINVALID when it names one of another
// kind: a reply filed against an extraction key would record coverage for a claim
// nobody critiqued.
func (w *Writer) FileCritique(key string, src []byte, warn io.Writer) (*Verdict, error) {
	const op = "bundle.Writer.FileCritique"

	if err := w.held(op); err != nil {
		return nil, err
	}
	meta, err := LoadPromptMeta(w.dir, key)
	if err != nil {
		return nil, err
	}
	if meta.Kind != PromptCritic {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": key " + key + " is a " + string(meta.Kind) +
				" prompt; a critic verdict is filed only against a critic prompt",
		}
	}
	// **Checked on the way out as well as on the way in**, which is the exception to
	// StorePromptMeta's rule and earns it: the coverage ledger is keyed on this field,
	// so a meta written by a build that had no such field would key a row on `#claim`
	// — a reference nothing can parse, in a file whose only job is to be matched
	// against later. A refusal here costs one re-emission; the row costs a claim's
	// steering history, silently.
	if strings.TrimSpace(meta.DocumentID) == "" {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": prompt " + key + " was emitted before critic prompts" +
				" recorded a document identifier, and its coverage would be keyed on" +
				" a reference nothing can resolve; re-run `gnosis critic` to emit it" +
				" again",
		}
	}

	// Cached before parsed, as admit does: the model call has happened, and throwing
	// the answer away would make the caller pay again to learn the same thing.
	if sErr := w.StoreCached(&CachedReply{
		Key: key, URI: meta.URI, Model: meta.Model, Reply: string(src),
	}); sErr != nil {
		return nil, sErr
	}
	reply, err := relay.ParseCriticReply(src)
	if err != nil {
		// Wrapped rather than returned bare, so the message carries where it
		// happened. The EINVALID survives: `errs.ErrorCode` walks the chain, so a
		// caller can still tell "the agent replied badly" from "the disk failed",
		// which is the distinction the command layer renders differently.
		return nil, &errs.Error{Op: op, Err: err}
	}
	return w.recordVerdict(key, &meta, &reply, warn)
}

// recordVerdict writes what a parsed verdict leaves behind.
//
// Split from FileCritique because the two answer different questions — that one decides
// whether the reply may be filed, this one files it — and because the parse is the point
// at which a caller's mistake stops being possible.
func (w *Writer) recordVerdict(
	key string, meta *PromptMeta, reply *relay.CriticReply, warn io.Writer,
) (*Verdict, error) {
	ref := gnosis.ClaimRef(gnosis.ID(meta.DocumentID), meta.ClaimID)
	if rErr := w.RecordCritique(&Critique{
		ClaimRef: ref, Key: key, Model: meta.Model, At: time.Now().UTC(),
		Examined: reply.Examined, NotExamined: reply.NotExamined,
	}); rErr != nil {
		return nil, rErr
	}

	out := &Verdict{
		ClaimRef: ref, Path: meta.DocumentPath,
		Examined: reply.Examined, NotExamined: reply.NotExamined,
		Findings: make([]finding.Diagnostic, 0, len(reply.Findings)),
		Moved:    w.documentMoved(meta),
	}
	for _, f := range reply.Findings {
		out.Findings = append(out.Findings, finding.Diagnostic{
			Severity: finding.SeverityWarning,
			Category: f.Category,
			Path:     meta.DocumentPath,
			Message:  "claim " + meta.ClaimID + ": " + f.Message,
			Action:   finding.ActionHuman,
		})
	}
	w.spendAnswered(key, "verdict", warn)
	return out, nil
}

// spendAnswered removes a prompt whose reply has landed, and reports a failure as a
// note. `what` names the thing that was filed, for the message.
//
// **Best-effort, and it does not become the operation's failure** — `Coordinator.spend`
// makes the same call one command over and gives the reason: the row is written, so
// telling a caller to retry because a file could not be unlinked would be a worse report
// than a note. Retrying is also the wrong advice here specifically, because a second run
// would append a second coverage row for one critique and make the ledger say the claim
// was looked at twice.
//
// It serves two commands, which is why `what` is a parameter rather than the word
// "verdict" written into the string: `gnosis file` lands an answer through the same
// step, and a note telling that caller their *verdict* was filed would name an artifact
// they never produced.
func (w *Writer) spendAnswered(key, what string, warn io.Writer) {
	if err := w.SpendPrompt(key); err != nil && warn != nil {
		_, _ = fmt.Fprintf(warn,
			"warning: the "+what+" was filed but its prompt was not removed: %v\n", err)
	}
}

// documentMoved reports whether the document changed after the prompt was emitted.
//
// A failure to read it is reported as *not moved*, which is the quiet direction and the
// right one here: the alternative is a note saying the text may have changed, on a
// corpus where the only thing that changed is that a file could not be opened.
func (w *Writer) documentMoved(meta *PromptMeta) bool {
	if meta.DocumentPath == "" || meta.DocumentHash == "" {
		return false
	}
	hash, err := w.conceptHash("bundle.Writer.documentMoved", meta.DocumentPath)
	if err != nil {
		return false
	}
	return hash != meta.DocumentHash
}

// draw picks the claims to critique.
//
// Requires: population is every critiquable claim, in corpus order.
// Ensures: the whole population when a path was named, otherwise a reproducible sample
// and the seed that produced it. §18.3 requires the draw to be reproducible and §6.2.1
// requires the specific draw to be inspectable, which is why the seed comes back.
func (w *Writer) draw(
	op string, population []criticTarget, opts *CriticOptions,
) ([]criticTarget, uint64, error) {
	if opts.Path != "" {
		return population, 0, nil
	}
	std, err := LoadSampleStandards(w.dir)
	if err != nil {
		return nil, 0, &errs.Error{Op: op, Err: err}
	}
	n := opts.SampleN
	if n <= 0 {
		n = std.CriticDefault.Value
	}

	byRef := make(map[string]criticTarget, len(population))
	refs := make([]string, 0, len(population))
	for i := range population {
		byRef[population[i].Ref] = population[i]
		refs = append(refs, population[i].Ref)
	}
	out := make([]criticTarget, 0, n)
	for _, ref := range gnosis.Sample(std.Seed.Value, n, refs) {
		out = append(out, byRef[ref])
	}
	// Restored to corpus order, so two runs with one seed emit prompts in one order
	// and a reader comparing them is diffing content rather than sequence.
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out, std.Seed.Value, nil
}

// criticPrompt renders and writes one claim's prompt, or reports it already answered.
func (w *Writer) criticPrompt(
	op string, target *criticTarget, critiques []Critique, opts *CriticOptions,
) (Pending, error) {
	text, err := os.ReadFile(filepath.Join(w.dir, filepath.FromSlash(target.ArchivePath)))
	if err != nil {
		return Pending{}, &errs.Error{Op: op, Err: err}
	}
	examined, notExamined := CoveredAngles(critiques, target.Ref)
	prompt := relay.RenderCritic(&relay.CriticRequest{
		URI: target.URI, SourceHash: target.SourceHash, Text: string(text),
		Claim: relay.CriticClaim{
			Text: target.Text, Lead: target.Lead, Quotes: target.Quotes,
		},
		Examined: examined, NotExamined: notExamined, Model: opts.Model,
	})

	_, cached, err := LoadCached(w.dir, prompt.Key)
	if err != nil {
		return Pending{}, err
	}
	if cached {
		return Pending{URI: target.URI, Key: prompt.Key, Cached: true}, nil
	}
	meta := PromptMeta{
		Key: prompt.Key, Kind: PromptCritic, URI: target.URI,
		SourceHash: target.SourceHash, ArchivePath: target.ArchivePath,
		DocumentPath: target.Path, DocumentHash: target.DocumentHash,
		DocumentID: target.ID.String(), ClaimID: target.ClaimID,
		Model: opts.Model.Name,
	}
	// Metadata before the prompt, as ingest does and for the same reason: a crash
	// between them leaves a meta file describing a prompt that is not there, which is
	// inert. The reverse leaves a prompt an agent can answer and nothing can accept.
	if err := w.StorePromptMeta(&meta); err != nil {
		return Pending{}, err
	}
	rel := promptPath(prompt.Key)
	full := filepath.Join(w.dir, filepath.FromSlash(rel))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, []byte(prompt.Text), 0o640); wErr != nil {
		return Pending{}, &errs.Error{Op: op, Err: wErr}
	}
	// **The one miss this corpus can act on** (§6.4). A critique is asked because no
	// deterministic check decides whether a quotation *bears on* the claim it is
	// offered for (§17.1), so `checks_run` names the checks that did look at this
	// claim and found nothing to say about that question — which is the shape §6.4's
	// own example has, and the shape that makes a recurrence a backlog item.
	w.noteMiss(&Miss{
		Op: "critique", Reason: gnosis.MissNoPredicate, Key: prompt.Key,
		ChecksRun: criticChecksRun(), Candidate: target.Ref, At: time.Now().UTC(),
	}, opts.Warn)
	return Pending{URI: target.URI, Key: prompt.Key, Path: rel}, nil
}

// critiquable is every claim a critic could judge, and how many were excluded.
//
// Requires: docs came from Load; sources maps an archive path to the source it holds;
// path restricts to one document when non-empty.
// Ensures: targets in corpus order, and a count of the claims skipped for citing no
// archived source. Pure.
//
// **The first resolving archive path wins, and the claim is judged against that one.**
// A claim resting on three sources could be critiqued three times, and §10.5's sample is
// over claims rather than over claim-source pairs — so one prompt per claim is the unit
// the seed draws from, and the source is the first the claim itself names.
func critiquable(
	docs []Document, path string, sources map[string]lint.SourceVersion,
) ([]criticTarget, int) {
	var (
		out     []criticTarget
		skipped int
	)
	for i := range docs {
		doc := &docs[i]
		if path != "" && doc.Path != path {
			continue
		}
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			archive, source, ok := firstArchived(claim.ArchivePaths, sources)
			if !ok || len(claim.Quotes) == 0 {
				skipped++
				continue
			}
			out = append(out, criticTarget{
				Ref: gnosis.ClaimRef(doc.ID, claim.ID),
				ID:  doc.ID, Path: doc.Path, DocumentHash: doc.Hash,
				ClaimID: claim.ID,
				Text:    claim.Anchor, Lead: claim.Lead, Quotes: claim.Quotes,
				URI: source.URI, SourceHash: source.SHA256, ArchivePath: archive,
			})
		}
	}
	return out, skipped
}

// firstArchived is the first of a claim's paths that tier 0 has a record for.
//
// Requires: sources came from sourceVersions.
// Ensures: comma-ok, so a claim whose every path is dangling is skipped rather than
// critiqued against nothing. Pure.
func firstArchived(
	paths []string, sources map[string]lint.SourceVersion,
) (string, lint.SourceVersion, bool) {
	for _, p := range paths {
		if source, ok := sources[p]; ok {
			return p, source, true
		}
	}
	return "", lint.SourceVersion{}, false
}

// criticChecksRun names the deterministic checks that examine a claim's evidence and
// still cannot answer the critic's question.
//
// Requires: nothing.
// Ensures: a freshly built slice, so a caller cannot reorder the list another caller
// will see. Pure.
//
// **It is a list rather than a count, and it is the honest content of a miss row.**
// §17.1's Gettier argument is that these checks establish a claim is supported *in the
// way it says it is* and cannot establish that the quotation **bears on** the claim.
// Naming them is what makes a recurrence readable: a reader of `gnosis miss report` sees
// which mechanisms were already tried, and therefore what a new predicate would have to
// do that these do not.
//
// A hand-maintained list is a second place to remember, and this one is allowed for the
// reason §17.5's verb list is: it can only under-report. A check added and not listed
// here makes a miss row say less than it could; nothing here can claim a check ran that
// did not, because these are the checks that run over every claim carrying evidence.
func criticChecksRun() []string {
	return []string{"coverage", "evidence", "rung"}
}
