package bundle

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/relay"
	"github.com/StevenACoffman/gnosis/internal/segment"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/quotecheck"
)

// admit consumes an agent's reply and writes the resulting document to quarantine.
//
// Requires: the writer lock is held; cmd has been validated.
// Ensures: the reply is cached whatever happens next, because a model call already
// happened and discarding the answer would make the caller pay for it again to
// learn the same thing. The document reaches quarantine only when every claim is
// supported.
//
// **The order is segment, then check, and it is not incidental** (§9.4). A
// sentence carrying two assertions gets one verdict otherwise, so a quotation
// validating only its first half reports the whole sentence supported — a silent
// false pass in the check the corpus most depends on.
func (c *Coordinator) admit(_ context.Context, cmd *command.Admit) (gnosis.Outcome, error) {
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
	if cErr := c.cacheReply(op, cmd, &meta); cErr != nil {
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
		outcome := checked.outcome(cmd)
		c.record(&audit.Row{
			At: c.now(), Op: audit.OpAdmit, Actor: string(cmd.Submitter),
			Outcome: string(outcome.Status), Detail: outcome.Message,
		})
		return outcome, nil
	}
	if !cmd.Eff.Writes() {
		return gnosis.OK(map[string]any{
			"key": cmd.Key, "effect": cmd.Eff.String(),
			"claims": len(checked.claims), "would_quarantine": true,
		}), nil
	}
	return c.quarantineReply(op, cmd, &reply, checked)
}

// cacheReply stores the answer under its key.
//
// It runs before the reply is even parsed, deliberately. The model call is already
// spent; caching only replies that turned out to be usable would make a caller pay
// again to receive the same unusable answer, and §6.1's promise is that a second
// run over unchanged inputs makes no model calls — not that it makes none when the
// first run went well.
func (c *Coordinator) cacheReply(op string, cmd *command.Admit, meta *PromptMeta) error {
	if err := StoreCached(c.Dir, &CachedReply{
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
	body, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(meta.ArchivePath)))
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	sources := []quotecheck.Source{{Name: meta.ArchivePath, Text: string(body)}}

	out := &checked{}
	for i := range reply.Claims {
		rc := &reply.Claims[i]
		rc.ID = "claim-" + strconv.Itoa(i+1)
		// Segment first. A reply's "one claim" is whatever the model decided one
		// claim was, and §9.4 does not take its word for it.
		parts := segment.Claims(rc.Text)
		out.claims = append(out.claims, parts...)

		findings := quotecheck.Check(rc.Quotes, sources)
		rc.ArchivePaths = foundIn(findings)
		switch {
		case quotecheck.Support(findings) > 0:
		case allUnchecked(findings):
			out.unchecked = append(out.unchecked, describe(i, rc.Text))
		default:
			out.unsupported = append(out.unsupported, describe(i, rc.Text))
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
	op string, cmd *command.Admit, reply *relay.Reply, k *checked,
) (gnosis.Outcome, error) {
	id, err := gnosis.NewID()
	if err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	rel := "c/" + id.String() + "-" + gnosis.SlugFrom(reply.Title).String() + ".md"

	content := renderQuarantined(id, reply, k)
	if _, err = Quarantine(c.Dir, rel, content); err != nil {
		return gnosis.Outcome{}, &errs.Error{Op: op, Err: err}
	}
	c.record(&audit.Row{
		At: c.now(), Op: audit.OpAdmit, Actor: string(cmd.Submitter),
		Paths:     []string{rel},
		HashAfter: hashOrEmpty(content),
		Outcome:   string(gnosis.StatusOK),
		Detail:    "quarantined from reply " + cmd.Key,
	})
	return gnosis.OK(map[string]any{
		"key": cmd.Key, "effect": cmd.Eff.String(),
		"path": rel, "claims": len(k.claims), "quarantined": true,
	}), nil
}

// describe names a claim in a message, truncated so a report stays readable.
func describe(i int, text string) string {
	const width = 60
	if len(text) > width {
		text = text[:width] + "…"
	}
	return "claim " + strconv.Itoa(i+1) + ": " + text
}
