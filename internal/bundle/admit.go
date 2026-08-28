package bundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/gnosis/internal/segment"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/identity"
	"github.com/StevenACoffman/skillet/quotecheck"
	"github.com/StevenACoffman/skillet/textnorm"
)

// admit consumes an agent's reply and writes the resulting document to quarantine.
//
// Requires: cmd has been validated; w still holds the lock.
// Ensures: the reply is cached whatever happens next, because a model call already
// happened and discarding the answer would make the caller pay for it again to
// learn the same thing. The document reaches quarantine only when every claim is
// supported.
//
// **The order is segment, then check, and it is not incidental** (§9.4). A
// sentence carrying two assertions gets one verdict otherwise, so a quotation
// validating only its first half reports the whole sentence supported — a silent
// false pass in the check the corpus most depends on.
func (c *Coordinator) admit(
	_ context.Context, w *Writer, cmd *command.Admit,
) (gnosis.Outcome, error) {
	const op = "bundle.Coordinator.admit"

	// What this key was a question about, before anything else. A key naming no
	// emitted prompt is a reply to a question nobody asked, and caching it would
	// leave an entry nothing will ever hit.
	meta, err := LoadPromptMeta(c.Dir, cmd.Key)
	if err != nil {
		if errs.ErrorCode(err) == errs.ENOTFOUND {
			return gnosis.Blocked(gnosis.ReasonNeedsHuman, err.Error(),
				map[string]any{"key": cmd.Key}), nil
		}
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}

	reply, parseErr := relay.ParseReply([]byte(cmd.Reply))
	if cErr := c.cacheReply(op, w, cmd, &meta); cErr != nil {
		return gnosis.Outcome{}, cErr
	}
	if parseErr != nil {
		// A malformed reply is a finding about the reply, not a broken tool. The
		// agent can be told exactly what to fix and asked again.
		return gnosis.Blocked(gnosis.ReasonUnparsable, parseErr.Error(),
			map[string]any{"key": cmd.Key}), nil
	}

	// The source and the archive it was checked against come from tier 0, keyed by
	// what the cached prompt was about — never from the reply. A model that could
	// name its own source could cite one it never read, and one that could nominate
	// its own archive could choose a file its quotations happen to appear in.
	reply.SourceURI = meta.URI
	checked, err := c.checkReply(op, &reply, &meta)
	if err != nil {
		return gnosis.Outcome{}, err
	}
	if len(checked.unsupported) > 0 || len(checked.unchecked) > 0 {
		return c.refuseReply(w, cmd, &meta, checked), nil
	}
	return c.applyReply(op, w, cmd, &reply, &meta, checked)
}

// applyReply routes a checked reply to what its prompt asked for.
//
// Requires: every quotation was supported; meta.Kind is set, which StorePromptMeta
// guarantees.
// Ensures: exactly one of the three operations runs. Named rather than defaulted, so a
// kind added later has to be classified deliberately.
func (c *Coordinator) applyReply(
	op string, w *Writer, cmd *command.Admit, reply *relay.Reply, meta *PromptMeta,
	k *checked,
) (gnosis.Outcome, error) {
	switch meta.Kind {
	case PromptAccrete:
		return c.accreteReply(op, w, cmd, reply, meta)
	case PromptSynthesize:
		return c.synthesizeReply(op, w, cmd, reply, meta, k)
	case PromptSource, PromptKindUnset:
	}
	if !cmd.Eff.Writes() {
		return gnosis.OK(map[string]any{
			"key": cmd.Key, "effect": cmd.Eff.String(),
			"claims": len(k.claims), "would_quarantine": true,
		}), nil
	}
	return c.quarantineReply(op, w, cmd, reply, k)
}

// refuseReply records a reply whose quotations the archive did not support.
//
// The archived text is named on the row, so it says *which* source the claims were not
// supported by. Without it the record would be "somebody once asserted this and it did
// not hold", which is a fact about nobody.
func (c *Coordinator) refuseReply(
	w *Writer, cmd *command.Admit, meta *PromptMeta, k *checked,
) gnosis.Outcome {
	outcome := k.outcome(cmd)
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpAdmit, Actor: string(cmd.Submitter),
		Paths:       meta.Archives(),
		Outcome:     string(outcome.Status),
		Detail:      outcome.Message,
		Unsupported: k.withheld,
	})
	return outcome
}

// accreteReply appends a concept-bound reply's evidence to the document it was about.
//
// Requires: meta.Kind is PromptConcept; the reply's quotations have been checked.
// Ensures: the document's body is unchanged or nothing is written (§6.3). A reply
// adding no new quotation is a no-op rather than a fresh commit, so re-ingesting an
// unchanged source costs nothing.
//
// **The document is re-read here and its hash compared**, because the prompt was
// computed against bytes that may since have moved. Applying an answer to a document
// that changed underneath it is §9.4's approved-diff window one level up, and the
// comparison is what closes it.
func (c *Coordinator) accreteReply(
	op string, w *Writer, cmd *command.Admit, reply *relay.Reply, meta *PromptMeta,
) (gnosis.Outcome, error) {
	doc, hash, err := c.readConcept(op, meta.DocumentPath)
	if err != nil {
		return gnosis.Outcome{}, err
	}
	if hash != meta.DocumentHash {
		return gnosis.Blocked(gnosis.ReasonNeedsHuman,
			meta.DocumentPath+" changed since this prompt was emitted, so the reply "+
				"was computed against bytes that are gone; re-run `gnosis ingest` for it",
			map[string]any{"key": cmd.Key, "path": meta.DocumentPath}), nil
	}

	id, _ := doc.Text(idKey)
	plan, err := Accrete(doc, gnosis.ID(id), reply)
	if err != nil {
		return gnosis.Outcome{}, err
	}
	data := map[string]any{
		"key": cmd.Key, "path": meta.DocumentPath,
		"added": plan.Added, "unmatched": plan.Unmatched,
	}
	if plan.Added == 0 || !cmd.Eff.Writes() {
		data["effect"] = cmd.Eff.String()
		return gnosis.OK(data), nil
	}
	if err := w.WriteConcept(meta.DocumentPath, plan.Content); err != nil {
		return gnosis.Outcome{}, err
	}
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpAdmit, Actor: string(cmd.Submitter),
		Paths:   []string{meta.DocumentPath},
		Outcome: string(gnosis.StatusOK),
		Detail:  "accreted " + strconv.Itoa(plan.Added) + " quotation(s)",
	})
	return gnosis.OK(data), nil
}

// synthesizeReply replaces a concept's body with a gated rewrite (§6.3).
//
// Requires: meta.Kind is PromptSynthesize; the reply's quotations have been checked.
// Ensures: nothing is written unless every quotation the document already held survives
// and every quotation the rewrite offers validates. The diff is reported either way,
// because a caller refusing a rewrite still needs to see what it proposed.
func (c *Coordinator) synthesizeReply(
	op string, w *Writer, cmd *command.Admit, reply *relay.Reply, meta *PromptMeta,
	k *checked,
) (gnosis.Outcome, error) {
	doc, hash, err := c.readConcept(op, meta.DocumentPath)
	if err != nil {
		return gnosis.Outcome{}, err
	}
	if hash != meta.DocumentHash {
		return gnosis.Blocked(gnosis.ReasonNeedsHuman,
			meta.DocumentPath+" changed since this prompt was emitted, so the rewrite "+
				"was computed against bytes that are gone; re-run `gnosis synthesize`",
			map[string]any{"key": cmd.Key, "path": meta.DocumentPath}), nil
	}

	id, _ := doc.Text(idKey)
	plan, err := Synthesize(doc, gnosis.ID(id), reply, k.supported)
	if err != nil {
		return gnosis.Outcome{}, err
	}
	plan.Path = meta.DocumentPath
	data := map[string]any{
		"key": cmd.Key, "path": plan.Path, "diff": plan.Diff,
		"dropped": plan.Dropped, "unvalidated": plan.Unvalidated,
	}
	if !plan.Approved() {
		return gnosis.Blocked(gnosis.ReasonRefused,
			"the rewrite would lose evidence the document already held", data), nil
	}
	if !cmd.Eff.Writes() {
		data["effect"] = cmd.Eff.String()
		return gnosis.OK(data), nil
	}
	if err := w.WriteConcept(plan.Path, plan.Content); err != nil {
		return gnosis.Outcome{}, err
	}
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpAdmit, Actor: string(cmd.Submitter),
		Paths: []string{plan.Path}, Outcome: string(gnosis.StatusOK),
		Detail: "synthesized " + plan.Path,
	})
	return gnosis.OK(data), nil
}

// recordSupported notes which whole quotations the archive supports.
//
// Requires: quotes are one claim's, as offered; sources are the archived files.
// Ensures: a quotation is recorded only when checking it alone finds support.
//
// **Per quotation, checked one at a time, and the reason is that `Check` answers a
// different question.** It reports per *passage* — a quotation is split, and each part
// gets its own finding — so folding a finding's passage yields fragments that no
// caller holding the original quotation can look up. The synthesis gate asks "does the
// document still hold this exact quotation", which only a per-quotation answer
// settles. The cost is one extra scan of the archive per quotation, which is bounded
// by the archive and paid only where the answer is needed.
//
// Fold-normalised, because that is the space the evidence invariant compares in: a
// passage re-offered with different whitespace is the same passage.
func recordSupported(k *checked, quotes []string, sources []quotecheck.Source) {
	if k.supported == nil {
		k.supported = map[string]bool{}
	}
	for _, q := range quotes {
		if quotecheck.Support(quotecheck.Check([]string{q}, sources)) > 0 {
			k.supported[textnorm.Fold(q)] = true
		}
	}
}

// readConcept loads a concept and the hash of the bytes it was read from.
func (c *Coordinator) readConcept(op, rel string) (*okf.Document, string, error) {
	raw, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, "", &errs.Error{Op: op, Err: err}
	}
	doc, err := okf.Parse(raw)
	if err != nil {
		return nil, "", &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	return doc, identity.Hash(string(raw)), nil
}

// cacheReply stores the answer under its key.
//
// It runs before the reply is even parsed, deliberately. The model call is already
// spent; caching only replies that turned out to be usable would make a caller pay
// again to receive the same unusable answer, and §6.1's promise is that a second
// run over unchanged inputs makes no model calls — not that it makes none when the
// first run went well.
func (c *Coordinator) cacheReply(
	op string, w *Writer, cmd *command.Admit, meta *PromptMeta,
) error {
	if err := w.StoreCached(&CachedReply{
		Key: cmd.Key, URI: meta.URI, Reply: cmd.Reply,
	}); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	return nil
}

// checkReply segments the reply's claims and validates each against the archive.
// Quotations are checked against **the one archived file the prompt was built
// from**, not against the whole archive. Checking against everything would let a
// reply about one source pass on a phrase that happens to appear in another, which
// is the evidence check answering a question nobody asked.
func (c *Coordinator) checkReply(
	op string, reply *relay.Reply, meta *PromptMeta,
) (*checked, error) {
	archives := meta.Archives()
	sources := make([]quotecheck.Source, 0, len(archives))
	for _, rel := range archives {
		body, rErr := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(rel)))
		if rErr != nil {
			return nil, &errs.Error{Op: op, Err: rErr}
		}
		sources = append(sources, quotecheck.Source{Name: rel, Text: string(body)})
	}

	// Read once, outside the loop: the words are the same for every claim in the
	// reply, and reading them per claim would put file I/O inside a fold.
	markers := dependentMarkers(c.Dir)

	out := &checked{}
	for i := range reply.Claims {
		rc := &reply.Claims[i]
		rc.ID = "claim-" + strconv.Itoa(i+1)
		// Segment first. A reply's "one claim" is whatever the model decided one
		// claim was, and §9.4 does not take its word for it.
		parts := segment.Claims(rc.Text, markers)
		out.claims = append(out.claims, parts...)

		findings := quotecheck.Check(rc.Quotes, sources)
		rc.ArchivePaths = foundIn(findings)
		recordSupported(out, rc.Quotes, sources)
		switch {
		case quotecheck.Support(findings) > 0:
		case allUnchecked(findings):
			out.unchecked = append(out.unchecked, describe(i, rc.Text))
		default:
			out.unsupported = append(out.unsupported, describe(i, rc.Text))
			out.withheld = append(out.withheld, rc.Text)
		}
	}
	return out, nil
}

// foundIn names the archived files a claim's quotations were actually located in,
// deduplicated and sorted.
//
// It is derived from the check rather than declared, which is the point: the
// document records where the evidence *was found*, so a later reader validating it
// looks in the same place this run did. A path nobody found anything in is not
// recorded, because it would read as evidence that was never there.
func foundIn(findings []quotecheck.Finding) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range findings {
		if f.Status == quotecheck.Found && f.FoundIn != "" && !seen[f.FoundIn] {
			seen[f.FoundIn] = true
			out = append(out, f.FoundIn)
		}
	}
	sort.Strings(out)
	return out
}

// allUnchecked reports whether nothing was actually looked for.
//
// The distinction quotecheck draws and this must not collapse: Missing means the
// passage was sought in the archive and is not there, and Unchecked means nobody
// looked — because every passage was too short to be evidence, or because there
// was no source to look in. Reporting the second as the first would accuse an
// agent of fabricating a quotation that may well be accurate.
func allUnchecked(findings []quotecheck.Finding) bool {
	if len(findings) == 0 {
		return true
	}
	for _, f := range findings {
		if f.Status != quotecheck.Unchecked {
			return false
		}
	}
	return true
}

// quarantineReply renders the document and writes it to tier 1.
func (c *Coordinator) quarantineReply(
	op string, w *Writer, cmd *command.Admit, reply *relay.Reply, k *checked,
) (gnosis.Outcome, error) {
	id, err := gnosis.NewID()
	if err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	rel := "c/" + id.String() + "-" + gnosis.SlugFrom(reply.Title).String() + ".md"

	content := renderQuarantined(id, reply, k)
	if _, err = w.Quarantine(rel, content); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	c.record(w, &audit.Row{
		At: c.now(), Op: audit.OpAdmit, Actor: string(cmd.Submitter),
		Paths:     []string{rel},
		HashAfter: hashOrEmpty(content),
		Outcome:   string(gnosis.StatusOK),
		Detail:    "quarantined from reply " + cmd.Key,
	})
	c.spend(w, cmd.Key)
	return gnosis.OK(map[string]any{
		"key": cmd.Key, "effect": cmd.Eff.String(),
		"path": rel, "claims": len(k.claims), "quarantined": true,
	}), nil
}

// spend removes the prompt this reply answered.
//
// **Here rather than where the reply is cached, and the entry's own wording is what
// this corrects.** "Once the reply is cached" is the wrong trigger: caching happens
// before the reply is even parsed, and a malformed or unsupported reply leaves the
// agent expected to submit another one under the same key — which `admit` can only
// accept while the metadata is still there. Removing it then would turn "fix the YAML
// and re-run" into "this key no longer exists". So the prompt is spent when the
// document is filed, which is the one outcome after which nothing more will be
// admitted under that key.
//
// A preview keeps its prompt for the same reason, structurally: it never reaches here.
//
// Best-effort, and it does not become the operation's failure. The reply is cached and
// the document is quarantined; telling a caller to retry that because a file could not
// be unlinked would be a worse report than a note. Warn is where it goes, because a
// note that exists only in a JSON field is one nobody in a terminal reads.
func (c *Coordinator) spend(w *Writer, key string) {
	if err := w.SpendPrompt(key); err != nil && c.Warn != nil {
		_, _ = fmt.Fprintf(c.Warn,
			"warning: the reply was filed but its prompt was not removed: %v\n", err)
	}
}

// describe names a claim in a message, truncated so a report stays readable.
func describe(i int, text string) string {
	const width = 60
	if len(text) > width {
		text = text[:width] + "…"
	}
	return "claim " + strconv.Itoa(i+1) + ": " + text
}
