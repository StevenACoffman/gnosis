// Package showcmd implements the "show" CLI command.
package showcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/identity"
)

// Config holds the configuration for the show command.
type Config struct {
	*root.Config
	Body    bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: the document, its links, its freshness, and optionally
// its text.
type Result struct {
	*index.Detail
	Body string `json:"body,omitempty"`

	// Reindex is true when the file on disk differs from what the index holds.
	//
	// Only computed with --body, because that is the case where the divergence is
	// visible: the text on screen came from the file and the snippet a `search` would
	// show came from the index. Without the body there is nothing on screen for the
	// warning to be about, and a warning attached to nothing is noise.
	Reindex bool `json:"reindex,omitempty"`

	// Freshness is how current the sources under this document are (§14.3).
	//
	// Rendered on the read path because that is the only place it reaches the
	// person who is about to rely on the claim. The index has known this since
	// checked.jsonl existed and nothing showed it, which made staleness a fact
	// about the corpus that the corpus did not tell anyone.
	Freshness bundle.DocFreshness `json:"freshness"`
}

// New registers the show command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("show").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Body, 'b', "body", "include the document text")
	cfg.Command = &ff.Command{
		Name:      "show",
		Usage:     "gnosis show <PATH|ID>",
		ShortHelp: "render a document with its links resolved",
		LongHelp: `Render one document: its title, its outbound links with each target
resolved, and the documents linking back to it.

A reference may be a bare identifier, a current path, or a **stale** path whose
slug no longer matches the title. All three resolve, because the identifier is
parsed out of the filename and matched on rather than the path being looked up.
That is what lets a link written before a retitle keep working with no mapping
table and no redirect.

Links are rendered inline, including the ones that lead nowhere. An unresolved
link is not an error — it is knowledge somebody intended and nobody has written
yet — and hiding it would remove the only record that the author meant to point
somewhere.

The text is read from the file rather than the index, because the file is the
source of truth and the index is a cache of it.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: resolve the reference, gather, render.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New(
			"show needs exactly one path or identifier; try `gnosis show c/<id>-<slug>.md`"))
	}

	db, err := bundle.OpenIndexForRead(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	defer func() { _ = db.Close() }()

	detail, err := db.Find(ctx, args[0])
	if err != nil {
		switch errs.ErrorCode(err) {
		case errs.EINVALID:
			return c.usage(err)
		case errs.ENOTFOUND:
			// Not a usage error and not a tool failure: the reference was
			// well-formed and the corpus does not hold it. A caller that asked
			// for something absent needs a distinct code from one that asked
			// wrongly.
			return c.fail(root.ReasonNeedsHuman, err)
		}
		return c.fail(root.ReasonIndexDrift, err)
	}

	result := Result{Detail: detail}
	// A freshness that cannot be computed is not an error: the document renders,
	// and its state stays the zero value, which is `unknown` — the honest answer
	// when the lookup itself did not happen.
	if fresh, fErr := bundle.FreshnessFor(c.Bundle, detail.Path, time.Now().UTC()); fErr == nil {
		result.Freshness = fresh
	}
	if c.Body {
		if result.Body, err = c.readBody(detail.Path); err != nil {
			return c.fail(root.ReasonNoBundle, err)
		}
		result.Reindex = staleIndex(result.Body, detail.ContentHash)
	}
	return c.report(&result)
}

// readBody reads the document from disk. The index holds a searchable copy; this
// reads the original, so what is shown is what is committed.
func (c *Config) readBody(rel string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(c.Bundle, filepath.FromSlash(rel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf(
				"%s is in the index but not on disk; run `gnosis index rebuild`", rel)
		}
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	return string(raw), nil
}

// report renders the outcome.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("show: %w", err)
		}
		return nil
	}

	_, _ = fmt.Fprintf(c.Stdout, "%s\n%s\n%s\n%s: %s\n",
		result.Title, result.Path, result.ID,
		result.Freshness.State, result.Freshness.Why)
	// Beside the freshness and not folded into it. §14.3.2 keeps the two signals
	// apart because they answer different questions, and the pair a reader most
	// needs is a document that was checked yesterday and has lost its support.
	if d := result.Freshness.Drift; d != "" {
		_, _ = fmt.Fprintf(c.Stdout, "upstream: %s\n", d)
	}
	writeClaims(c.Stdout, result.Freshness.Claims)
	if result.Reindex {
		// To stderr: the document rendered, and this is a note about the cache
		// rather than part of the answer. A reader piping stdout to a file still
		// sees it.
		_, _ = fmt.Fprintf(c.Stderr,
			"note: this file has changed since the last index rebuild, so `gnosis "+
				"search` still matches the old text; run `gnosis index rebuild`\n")
	}
	writeLinks(c.Stdout, "links to", result.Outbound)
	writeLinks(c.Stdout, "linked from", result.Inbound)
	if result.Body != "" {
		_, _ = fmt.Fprintf(c.Stdout, "\n%s", result.Body)
	}
	return nil
}

// writeClaims renders each claim's own freshness under the document's.
//
// Nothing at all for a document declaring no claims, which most hand-written Phase 2
// documents do: a heading over an empty list would say "this page has no claims" to a
// reader who asked about freshness, which is a different question answered by the
// `not_applicable` line above.
//
// The anchor is printed rather than the id where there is one, because the anchor is
// the sentence and the id is a name for it — and the point of reporting per claim is
// to put the reader in front of the sentence that rests on the unverified source.
func writeClaims(w io.Writer, claims []bundle.ClaimFreshness) {
	if len(claims) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\nclaims:\n")
	for i := range claims {
		cl := &claims[i]
		what := cl.Anchor
		if what == "" {
			what = cl.ID
		}
		_, _ = fmt.Fprintf(w, "  %s: %s\n    %s\n", cl.State, what, cl.Why)
		if cl.Drift != "" {
			_, _ = fmt.Fprintf(w, "    upstream: %s\n", cl.Drift)
		}
		// One line per source, never a count. A claim resting on four independent
		// sources and one resting on one look identical in frontmatter, and the
		// honest fix is to say which — a number would be the inheritance §1.1's
		// local reductionism refuses, and a reader could not tell four versions of
		// one page from four pages.
		for _, src := range cl.Sources {
			_, _ = fmt.Fprintf(w, "    source: %s\n", src)
		}
	}
}

// writeLinks renders one direction of the link graph, or says it is empty.
// Silence would be ambiguous between "no links" and "not looked up".
func writeLinks(w io.Writer, label string, links []index.Resolved) {
	if len(links) == 0 {
		_, _ = fmt.Fprintf(w, "\n%s: none\n", label)
		return
	}
	_, _ = fmt.Fprintf(w, "\n%s:\n", label)
	for _, l := range links {
		_, _ = fmt.Fprintf(w, "  %s\n", l)
	}
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("show: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("show: %w", c.Usage(cause))
}

// staleIndex reports whether the file differs from what the index recorded.
//
// Requires: body is the file's text as just read; indexed is the hash the index
// holds, which is empty for a document indexed before the column was populated.
// Ensures: false when the hashes agree and false when there is nothing to compare —
// an absent hash is not evidence of divergence, and reporting one would put a
// "rebuild the index" note on every document in a bundle indexed by an older build.
// Pure.
//
// The comparison goes through `identity.Hash`, which is what the indexer used. A
// second hashing function here would report every document as diverged, which is the
// failure mode of comparing two things normalised differently.
func staleIndex(body, indexed string) bool {
	if indexed == "" {
		return false
	}
	return identity.Hash(body) != indexed
}
