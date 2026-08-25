// Package debtcmd implements the "debt" CLI command.
package debtcmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Config holds the configuration for the debt command.
type Config struct {
	*root.Config

	// SampleN reports a reproducible sample of the documents carrying debt rather
	// than all of them. Zero reports everything.
	SampleN int

	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: what the corpus was admitted over.
type Result struct {
	*bundle.Debt

	// Sampled is how many documents were drawn, or zero when everything is
	// reported. It is separate from the debt itself because a reader must be able
	// to tell a sample from a census — a count that might be either is a count
	// nobody can act on.
	Sampled int `json:"sampled,omitempty"`

	// Seed is the draw's seed, present only for a sample. §6.2.1 requires the
	// specific draw to be inspectable, and a sample whose seed is not reported is
	// reproducible in principle and not in practice.
	Seed uint64 `json:"seed,omitempty"`
}

// New registers the debt command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("debt").SetParent(parent.Flags)
	cfg.Flags.IntVar(&cfg.SampleN, 0, "sample", 0,
		"report a reproducible sample of this many documents instead of all of them")
	cfg.Command = &ff.Command{
		Name:      "debt",
		Usage:     "gnosis debt [--sample N]",
		ShortHelp: "list the documents admitted over checks that could not run",
		LongHelp: `Report the corpus's accumulated debt: every document promoted over a gate
signal that could not run, and who carried it.

This is what separates ` + "`gnosis promote`" + `'s human path from a ` + "`--force`" + `. A
promotion carried by a person over an unrun check is defensible only if the corpus
can later find every claim admitted that way — otherwise "a human approved it" is
indistinguishable from a bypass, and the unrun check quietly becomes a check that
never runs. When the subsystem behind a signal lands, this is the query that says
what to re-examine.

The count is a **count**, not a score. SPEC §12 forbids presenting a finding count
as corpus health, and this is the same rule: a corpus with 34 documents carrying an
unrun conflict check is not 34 units unhealthy, it is a corpus with 34 documents to
look at when §10 exists.

--sample N draws a reproducible subset, seeded from standards/sample.toml, for the
case where the list is longer than anybody will start. The seed is reported with the
sample so the specific draw can be repeated or deliberately changed.

Reads only. It takes no lock and never writes, so it is safe to run against a
bundle somebody else is ingesting into.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: read the trail, compute, render.
//
// The trail is read rather than the corpus, and that is the point of the design it
// rests on: `audit.Row.Signals` records what each promotion was carried over at the
// moment it happened, so this answer does not depend on re-deriving a gate verdict
// against today's build. A document admitted over a conflict check stays in this
// report after §10 lands, which is exactly when somebody needs it.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New("debt takes no arguments; use --sample N to shorten the list"))
	}

	trail, err := bundle.AuditTrail(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	result := &Result{Debt: bundle.Owed(&trail)}

	if c.SampleN > 0 {
		seed, sErr := bundle.LoadSampleStandards(c.Bundle)
		if sErr != nil {
			return c.fail(root.ReasonStandardsInvalid, sErr)
		}
		drawn := gnosis.Sample(seed.Seed.Value, c.SampleN, result.Paths())
		result.Debt = result.Restricted(drawn)
		result.Sampled = len(drawn)
		result.Seed = seed.Seed.Value
	}
	return c.report(result)
}

// report renders the register.
//
// **No debt is `ok` and not a finding**, and neither is debt. A corpus carrying
// documents over unrun checks is the expected state of this build — `conflict`
// cannot run at all until §10 exists — so exiting non-zero would make the ordinary
// case look like a failure and teach a reader to ignore the command.
//
// A damaged trail is a **warning and not an error**, and the count is reported as a
// floor. That is the one thing this command must not get wrong: a debt total
// computed from a trail with unreadable lines is smaller than the truth, and
// reporting it as a total would be the flattering direction.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("debt: %w", err)
		}
		return c.warnIfPartial(result)
	}

	// Only the entries go to stdout. The per-signal counts are a summary of what
	// follows, and printing them above it put two different kinds of line in one
	// undifferentiated block — `conflict\t1` and `conflict\tc/a.md\thuman:priya`
	// read as the same shape, so a reader could not tell the total from a row.
	// Found by running the command, which is now the fifth time.
	for i := range result.Carried {
		e := &result.Carried[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\n", e.Signal, e.Path, e.Actor)
	}

	_, _ = fmt.Fprintf(c.Stderr, "%s over %s\n",
		countOf(len(result.Carried), "carried admission"),
		countOf(result.Rows, "promotion"))
	for _, signal := range result.Signals {
		_, _ = fmt.Fprintf(c.Stderr, "  %s: %d\n", signal, result.BySignal[signal])
	}
	if result.Sampled > 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"a reproducible sample of %d document(s), seed %d; "+
				"re-run without --sample for all of them\n",
			result.Sampled, result.Seed)
	}
	return c.warnIfPartial(result)
}

// warnIfPartial says so when the number is a floor.
//
// It writes to stderr in both output modes, because the JSON carries
// `unreadable_lines` and a person reading a terminal does not read the JSON. The
// exit status is unchanged: the trail is damaged, which is a fact about this
// machine's cache and not about the corpus, and `doctor` is what reports it as a
// finding.
func (c *Config) warnIfPartial(result *Result) error {
	if result.Complete() {
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr,
		"warning: %d unreadable line(s) in the write trail, so this count is a "+
			"floor rather than a total; run `gnosis doctor` for the damage\n",
		result.Unreadable)
	return nil
}

// countOf renders "1 promotion" and "3 promotions", because a report that says
// "1 promotions" reads as a placeholder somebody forgot.
func countOf(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("debt: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("debt: %w", c.Usage(cause))
}
