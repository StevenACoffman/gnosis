// Package searchcmd implements the "search" CLI command.
package searchcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/skillet/errs"
)

// defaultLimit bounds a result set to what a person will actually read. The
// number is not sacred; the bound is, because an unbounded ranked list is the
// exploratory output SPEC §5.6 says an explanatory surface must not produce.
const defaultLimit = 20

// Config holds the configuration for the search command.
type Config struct {
	*root.Config
	Limit int

	// Cases grades the corpus against standards/retrieval-cases.toml instead of
	// running one query (§11.0.2).
	//
	// A flag on `search` rather than its own command, because it *is* a search — the
	// same index, the same ranking, the same limit. A separate command would be a
	// second path to the same query, and the two could then disagree about what the
	// corpus answers, which is the one thing a retrieval suite must not do.
	Cases bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: the query as asked, and what matched.
type Result struct {
	Query string      `json:"query"`
	Hits  []index.Hit `json:"hits"`
}

// New registers the search command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("search").SetParent(parent.Flags)
	cfg.Flags.IntVar(&cfg.Limit, 'n', "limit", defaultLimit, "maximum results to return")
	cfg.Flags.BoolVar(&cfg.Cases, 0, "cases",
		"grade the corpus against standards/retrieval-cases.toml instead of querying")
	cfg.Command = &ff.Command{
		Name:      "search",
		Usage:     "gnosis search <QUERY> | gnosis search --cases",
		ShortHelp: "find documents by full text",
		LongHelp: `Find documents whose text matches an FTS5 query, best first.

Ranking weights the title above the body: a document *about* a term is more often
what a reader wants than one mentioning it in passing.

Each result carries its outbound links already resolved, so following one costs no
second query. That is a requirement rather than a convenience — a result that
forces a fresh search to follow a link reproduces the defect associative indexing
was invented to fix.

Phase 1 searches documents. Claim-level search arrives with extraction, because
identifying a claim needs more than the text can supply on its own.

--cases grades the corpus against standards/retrieval-cases.toml: labelled queries
with the titles that must come back, **including cases whose correct answer is that
the corpus holds nothing**. A corpus that answers every query with its best guess
cannot say "we do not know", which is the answer §14.3's whole vocabulary exists to
make expressible.

There is no pass rate. A case holds or it does not, and §17 forbids presenting a
count as health — a retrieval percentage is the most tempting such number there is,
because it looks like progress and rises when a failing case is deleted.

The file ships empty. §11.0.2 says cases are authored when a real query disappoints,
never invented up front, so an empty suite reports that it examined nothing rather
than reporting success.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: open the index, query, render.
func (c *Config) exec(ctx context.Context, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if c.Cases {
		if query != "" {
			return c.usage(errors.New(
				"--cases grades the whole suite and takes no query"))
		}
		if c.Limit < 1 {
			return c.usage(fmt.Errorf("--limit must be positive, got %d", c.Limit))
		}
		return c.cases(ctx)
	}
	if query == "" {
		return c.usage(errors.New("search needs a query; try `gnosis search retry budget`"))
	}
	if c.Limit < 1 {
		return c.usage(fmt.Errorf("--limit must be positive, got %d", c.Limit))
	}

	idx, err := bundle.LoadIndex(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}
	if !idx.Present {
		return c.fail(root.ReasonNoBundle,
			errors.New("no index; run `gnosis index rebuild` first"))
	}

	db, err := bundle.OpenIndexForRead(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	defer func() { _ = db.Close() }()

	hits, err := db.Search(ctx, query, c.Limit)
	if err != nil {
		// A malformed FTS5 query is the caller's syntax, not a broken tool.
		if errs.ErrorCode(err) == errs.EINVALID {
			return c.usage(err)
		}
		return c.fail(root.ReasonIndexDrift, err)
	}
	return c.report(&Result{Query: query, Hits: hits})
}

// report renders the outcome. No matches is a valid answer, not a finding.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("search: %w", err)
		}
		return nil
	}
	for _, h := range result.Hits {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\n", h.Path, h.Title)
		if h.Snippet != "" {
			_, _ = fmt.Fprintf(c.Stdout, "\t%s\n", h.Snippet)
		}
		for _, l := range h.Outbound {
			_, _ = fmt.Fprintf(c.Stdout, "\t→ %s\n", l)
		}
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d result(s) for %q\n", len(result.Hits), result.Query)
	return nil
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("search: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("search: %w", c.Usage(cause))
}
