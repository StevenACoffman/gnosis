// Package searchcmd implements the "search" CLI command.
package searchcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

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

// longHelp is the command's prose, extracted from New because a page of help text
// inside a constructor puts the function over the length limit — and the limit is
// measuring the wrong thing here, since none of it is logic. A const rather than a
// var: nothing may mutate it, and rules.md §1 forbids package-level mutable state.
const longHelp = `Find documents whose text matches an FTS5 query, best first.

Ranking weights the title above the body: a document *about* a term is more often
what a reader wants than one mentioning it in passing.

Each result carries its outbound links already resolved, so following one costs no
second query. That is a requirement rather than a convenience — a result that
forces a fresh search to follow a link reproduces the defect associative indexing
was invented to fix.

--claims queries claim leads instead of document bodies. A lead is a claim's
conclusion stated first (§17.4), which is why it is the searchable half: a result a
reader can judge without opening the document.

It reports how many claims carry no lead beside the results, because those claims
cannot match at any ranking. Extraction fills leads a document at a time, so for as
long as a corpus is part-way through, claim search answers from part of it — and a
result set that did not say so would present that part as the whole.

--provable returns only documents whose evidence can still be checked offline
(§14.4): every source they rest on was archived or extracted, rather than referenced
by URI and hash alone. It is the query the durability signal exists for — a reader
assembling an argument wants what can still be checked — and it says how many matches
it removed, because "no results" and "no results, and four rest on sources nobody can
check" send a reader to opposite conclusions.

It costs a read of tier 0's records, which a plain search does not do, so it is paid
only when the flag is passed. Partly provable documents are excluded: a page half of
whose sources are a hash and a URI can be partly checked, and §14.4 keeps that a
separate state precisely so a filter does not have to guess which half was wanted.

--type, --status, --under, --trust and --fresh restrict what comes back, and combine
as a conjunction — naming two means both. --under is a path prefix and matches whole
segments, so c/retry never admits c/retry-budget.md: a query restricted to a subtree can
never return a concept outside it. --status compares against OKF's own vocabulary rather
than a list gnosis holds, and a document declaring none is stable. --trust admits a tier
and everything above it, because a reader asking for machine-confirmed wants what has
been confirmed at least that far. --fresh means exactly fresh — a document citing no
source is not-applicable, which is a different state and does not pass.

Five of the six need the corpus rather than the index: it holds no document type, no
trust fold and no freshness. They are read once when a filter is named and not at all
when none is, so a plain search still touches only the index.

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
than reporting success.`

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

	// Claims queries claim leads instead of document bodies (§11).
	//
	// A flag rather than a command for the same reason --cases is one: it is the same
	// index, the same query language, and the same bound. What differs is the grain,
	// and a reader who wants a claim wants it from the tool that already found the
	// document.
	Claims bool

	// Provable restricts results to documents whose evidence can still be checked
	// offline (§14.4).
	//
	// It is the query the durability signal exists for. §14.4 states it plainly: a
	// reader assembling an argument wants to restrict to what can still be checked,
	// and without the filter the signal is decoration.
	//
	// **It costs a bundle read that a search otherwise does not do**, because
	// durability is derived from tier 0's records and never stored (§14) — so it is
	// paid only when the flag is passed, and a plain search still touches nothing but
	// the index.
	Provable bool

	// The §11.3 scoping filters. Each is unrestricted when empty or false, and they
	// combine as a conjunction — a caller naming two means both.
	//
	// **They go through one value rather than one branch each.** "Restrict the results
	// to documents with property P" is a single decision with six inputs, and five of
	// them need the bundle rather than the index; six branches here would each pay for
	// their own corpus read and could come to disagree about what a restriction means.
	// `--provable` folds in with them for that reason, having shipped as its own filter
	// one day earlier.
	DocType string
	Status  string
	Under   string
	Trust   string
	Fresh   bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: the query as asked, and what matched.
type Result struct {
	Query string      `json:"query"`
	Hits  []index.Hit `json:"hits"`

	// Filtered is how many matches --provable removed, and it is not omitempty for
	// the reason ClaimResult.Unextracted is not: "no results" and "no results, and
	// the filter removed four" send a reader to opposite conclusions, and on the
	// wire an absent field and a zero one are the same thing.
	Filtered int `json:"filtered"`
}

// ClaimResult is the claim-grain payload.
//
// **It carries what the query could not reach**, and the field is not omitempty: a
// corpus that is fully extracted must report zero rather than say nothing, because
// absence and zero are the same on the wire and only one of them is an answer.
type ClaimResult struct {
	Query       string           `json:"query"`
	Hits        []index.ClaimHit `json:"hits"`
	Unextracted int              `json:"unextracted"`

	// Filtered is how many matches --provable removed. See Result.Filtered.
	Filtered int `json:"filtered"`
}

// New registers the search command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("search").SetParent(parent.Flags)
	cfg.Flags.IntVar(&cfg.Limit, 'n', "limit", defaultLimit, "maximum results to return")
	cfg.Flags.BoolVar(&cfg.Cases, 0, "cases",
		"grade the corpus against standards/retrieval-cases.toml instead of querying")
	cfg.Flags.BoolVar(&cfg.Claims, 0, "claims",
		"query claim leads instead of document bodies")
	cfg.Flags.BoolVar(&cfg.Provable, 0, "provable",
		"return only documents whose evidence can still be checked offline")
	cfg.Flags.StringVar(&cfg.DocType, 0, "type", "", "restrict to one OKF type")
	cfg.Flags.StringVar(&cfg.Status, 0, "status", "",
		"restrict to one OKF status; a document declaring none is stable")
	cfg.Flags.StringVar(&cfg.Under, 0, "under", "", "restrict to a path prefix")
	cfg.Flags.StringVar(&cfg.Trust, 0, "trust", "",
		"restrict to documents at or above a trust tier (§14.1)")
	cfg.Flags.BoolVar(&cfg.Fresh, 0, "fresh",
		"restrict to documents whose sources are all verified and unexpired")
	cfg.Command = &ff.Command{
		Name:      "search",
		Usage:     "gnosis search <QUERY> | gnosis search --cases",
		ShortHelp: "find documents by full text",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: open the index, query, render.
func (c *Config) exec(ctx context.Context, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if err := c.validate(query); err != nil {
		return c.usage(err)
	}
	if c.Cases {
		return c.cases(ctx)
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

	if c.Claims {
		return c.queryClaims(ctx, db, query)
	}
	return c.queryDocuments(ctx, db, query)
}

// validate settles the flag combinations before anything is opened, so a mistake in
// the invocation is reported as one rather than after a bundle load that was never
// going to be used.
func (c *Config) validate(query string) error {
	if c.Cases && c.Claims {
		return errors.New("--cases grades the document suite; it cannot be combined with --claims")
	}
	if c.Cases && c.Provable {
		return errors.New(
			"--cases grades the suite as authored; it cannot be combined with --provable")
	}
	if c.Cases && query != "" {
		return errors.New("--cases grades the whole suite and takes no query")
	}
	if !c.Cases && query == "" {
		return errors.New("search needs a query; try `gnosis search retry budget`")
	}
	if c.Limit < 1 {
		return fmt.Errorf("--limit must be positive, got %d", c.Limit)
	}
	return nil
}

// queryDocuments answers at document grain.
func (c *Config) queryDocuments(ctx context.Context, db *index.DB, query string) error {
	hits, err := db.Search(ctx, query, c.Limit)
	if err != nil {
		return c.queryFailed(err)
	}
	scope, err := c.scope()
	if err != nil {
		return c.usage(err)
	}
	kept := make([]index.Hit, 0, len(hits))
	for _, h := range hits {
		if scope.Admits(h.Path) {
			kept = append(kept, h)
		}
	}
	return c.report(&Result{
		Query: query, Hits: kept, Filtered: len(hits) - len(kept),
	})
}

// queryClaims answers at claim grain.
func (c *Config) queryClaims(ctx context.Context, db *index.DB, query string) error {
	got, err := db.SearchClaims(ctx, query, c.Limit)
	if err != nil {
		return c.queryFailed(err)
	}
	scope, err := c.scope()
	if err != nil {
		return c.usage(err)
	}
	hits := make([]index.ClaimHit, 0, len(got.Hits))
	for _, h := range got.Hits {
		if scope.Admits(h.Path) {
			hits = append(hits, h)
		}
	}
	filtered := len(got.Hits) - len(hits)
	// Traced before the filter would be wrong and after it is right: §12.2's reach
	// report answers "did anybody ever use this claim", and a claim the reader was
	// never shown was not used.
	c.trace(hits)
	return c.reportClaims(&ClaimResult{
		Query:       query,
		Hits:        hits,
		Unextracted: got.Unextracted,
		Filtered:    filtered,
	})
}

// scope assembles §11.3's restriction from the flags.
//
// Requires: nothing.
// Ensures: a scope admitting everything when no filter was named, and then it reads
// nothing — a plain search still touches only the index. EINVALID naming the accepted
// values when a flag names something the vocabulary does not.
//
// **`--provable` is one of the six now.** It shipped a day earlier as its own filter
// with its own generic helper, and folding it in was the change worth making: two
// mechanisms answering "restrict to documents with property P" is one decision in two
// places, and the second would have been the one somebody forgot to extend.
func (c *Config) scope() (*bundle.SearchScope, error) {
	query, err := bundle.ParseScopeQuery(
		c.DocType, c.Status, c.Under, c.Trust, c.Fresh, c.Provable)
	if err != nil {
		// Wrapped so the message says where it happened; the EINVALID survives,
		// because `errs.ErrorCode` walks the chain and the caller reports a bad flag
		// value as a usage error rather than a corpus problem.
		return nil, fmt.Errorf("reading the scope flags: %w", err)
	}
	scope, err := bundle.LoadScope(c.Bundle, query)
	if err != nil {
		return nil, fmt.Errorf("reading what the corpus holds: %w", err)
	}
	return scope, nil
}

// trace records which claims this search returned, for §12.2's reach report.
//
// **Best-effort, and a failure is a warning rather than an error.** A search that
// answered correctly has answered; turning an observation into a failure would make the
// tracer the most fragile part of retrieval, and the thing it measures — reach — is not
// worth a wrong exit code. §4.3.1's rule that decisions are committed and observations
// are cached is the same argument one level down: nothing here is a fact about the
// corpus.
//
// It takes no lock, for the reason RecordRetrievals gives: `search` is a read command,
// and putting a retrieval behind the write coordinator would serialise reads behind
// every writer.
//
// **It is called after the scope filter**, because §12.2's question is whether anybody
// ever *used* a claim — and a claim the reader was never shown was not used.
func (c *Config) trace(hits []index.ClaimHit) {
	if len(hits) == 0 {
		return
	}
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	if err := bundle.RecordRetrievals(c.Bundle, time.Now().UTC(), ids); err != nil {
		_, _ = fmt.Fprintf(c.Stderr,
			"warning: this search was not recorded, so `gnosis audit --unretrieved` "+
				"will not count it: %v\n", err)
	}
}

// queryFailed classifies one query error for both grains, so the two cannot come to
// disagree about whose mistake a malformed FTS5 expression is.
func (c *Config) queryFailed(err error) error {
	// A malformed FTS5 query is the caller's syntax, not a broken tool.
	if errs.ErrorCode(err) == errs.EINVALID {
		return c.usage(err)
	}
	return c.fail(root.ReasonIndexDrift, err)
}

// reportClaims renders the claim grain.
//
// **The shortfall is unconditional on the wire and conditional in prose.** A JSON reader
// cannot tell an absent field from a zero one, so Unextracted is always emitted; a person
// reading "0 claim(s) carry no lead" on every query of a fully extracted corpus learns to
// skip the line, and the one time it matters it reads as furniture. The clause appears
// when it applies, which is what makes its absence informative.
//
// It is not a health number either way (§17): it is the size of the corpus this query
// could not see, and it falls as extraction proceeds rather than as anything improves.
func (c *Config) reportClaims(result *ClaimResult) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("search: %w", err)
		}
		return nil
	}
	for _, h := range result.Hits {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\n", h.Path, h.Lead)
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d claim(s) for %q\n", len(result.Hits), result.Query)
	writeFiltered(c.Stderr, result.Filtered)
	if result.Unextracted > 0 {
		// The command named here has to exist. The first version advised
		// `gnosis extract`, which does not — the very finding `lint`'s `command`
		// check reports, written into the tool that ships it.
		_, _ = fmt.Fprintf(c.Stderr,
			"%d claim(s) carry no lead and were not searched; a lead is written when "+
				"a reply is admitted, so `gnosis ingest` then `gnosis admit` is what "+
				"covers them\n", result.Unextracted)
	}
	return nil
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
	writeFiltered(c.Stderr, result.Filtered)
	return nil
}

// writeFiltered says how many matches the scope removed, and says nothing when it
// removed none.
//
// **Without it, a restricted query returning nothing is indistinguishable from the
// query matching nothing**, and those send a reader to opposite conclusions: one means
// the corpus is silent on the subject, the other means it holds matches the reader
// asked not to see. That is the defect `ClaimResults.Unextracted` was added for, one
// flag over.
//
// **It names no particular flag**, which the first version did — it said `--provable`
// back when that was the only filter, and after five more arrived it was telling a
// reader who had typed `--type Rule` that their evidence could not be checked offline.
// Found by running the command, which is the eleventh time in this repository's log.
//
// Silent at zero, for the reason the durability check's suppression line is: a line
// that appears on every run is one a reader stops seeing, and its absence is what makes
// its presence mean something.
func writeFiltered(w io.Writer, filtered int) {
	if filtered <= 0 {
		return
	}
	_, _ = fmt.Fprintf(w,
		"the scope removed %d match(es) the query found; drop a filter to see them\n",
		filtered)
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("search: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("search: %w", c.Usage(cause))
}
