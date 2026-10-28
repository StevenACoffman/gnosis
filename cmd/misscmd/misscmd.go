// Package misscmd implements the "miss" CLI command.
package misscmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// longHelp is the command's prose, extracted from New because a page of help text inside
// a constructor puts the function over the length limit and none of it is logic.
const longHelp = `Aggregate the miss log: how often a model was consulted, and why.

gnosis writes a row to .gnosis/miss.jsonl every time it emits a prompt. A miss is a
non-event — nothing happened, deterministically, and then a model was asked — and
without the log that is invisible: a claim about how rarely the model is consulted needs
the consultations counted.

Rows are grouped by reason, and the grouping is the report. A reason that means "no
deterministic path exists for this operation" recurs for as long as the corpus ingests
anything and is not work anybody can do; a reason that means "a deterministic path ran
and decided nothing" is a check waiting to be written. The actionable groups come first
for that reason — sorting by count alone would put the line nobody can act on at the top
of every run.

There is no hit rate, and its absence is deliberate. This log measures **reach**, never
correctness: a retrieval path that confidently returns the wrong concept every time has
a perfect miss-log record, because a wrong answer produces no fallback, no row and no
trace. "Ninety percent of queries were answered before step 5" is a true statement about
how rarely the model was asked and not a statement about accuracy, and a percentage here
would be the most target-shaped number this corpus could produce — it improves when
somebody stops asking.

Every count is a floor. Recording is best-effort, so a lost row makes the corpus look as
though the model was consulted less often than it was.

Reads only. It takes no lock and never writes, so it is safe to run against a bundle
somebody else is ingesting into.`

// Config holds the configuration for the miss command.
type Config struct {
	*root.Config

	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: what each reason accounts for.
type Result struct {
	Groups []bundle.MissGroup `json:"groups"`

	// Rows is how many emissions the log holds, so a reader can see that the groups
	// account for all of them rather than for a filtered subset.
	//
	// **Not omitempty.** An absent field and a zero one are the same on the wire, and
	// "no prompt has ever been emitted here" is an answer a caller may need to act on.
	Rows int `json:"rows"`
}

// New registers the miss command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("miss").SetParent(parent.Flags)
	report := &ff.Command{
		Name:      "report",
		Usage:     "gnosis miss report",
		ShortHelp: "aggregate the miss log by reason",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	cfg.Command = &ff.Command{
		Name:        "miss",
		Usage:       "gnosis miss <SUBCOMMAND>",
		ShortHelp:   "read the miss log: how often a model was consulted, and why",
		Subcommands: []*ff.Command{report},
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: read the log, group, render.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New("miss report takes no arguments"))
	}
	misses, err := bundle.LoadMisses(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(&Result{Groups: bundle.MissReport(misses), Rows: len(misses)})
}

// report renders the outcome.
//
// **Nothing exits non-zero**, including a corpus with a large actionable group. §17
// forbids presenting a count as health, and a command that failed on one would make the
// backlog signal into a build failure — at which point the cheapest fix is to stop
// emitting prompts, which is the opposite of what the log is for.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("miss: %w", err)
		}
		return nil
	}
	for i := range result.Groups {
		g := &result.Groups[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\n",
			g.Reason, questions(g), strings.Join(g.Ops, ", "))
		writeChecks(c, g)
	}
	// On stderr with the total, so a reader piping stdout keeps the scope — and the
	// word "at least", because a lost row undercounts and the report must not read as
	// a census.
	_, _ = fmt.Fprintf(c.Stderr, "at least %d emission(s) in %d group(s)\n",
		result.Rows, len(result.Groups))
	if result.Rows == 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"no prompt has been emitted from this bundle by this user; the log is "+
				"per-user and derived, so a colleague's consultations are not here\n")
	}
	return nil
}

// questions renders a group's count, saying so when a prompt was written more than once.
//
// **The two numbers are different facts and the line says which.** A count of distinct
// questions is how often a model was consulted; the emission count is how often a prompt
// reached disk, and the gap is the times somebody re-ran one nobody had answered. Showing
// only the larger would overstate the consultations, and showing only the smaller would
// hide that the relay is being re-run.
func questions(g *bundle.MissGroup) string {
	if g.Emissions == g.Count {
		return strconv.Itoa(g.Count)
	}
	return strconv.Itoa(g.Count) + " (" + strconv.Itoa(g.Emissions) + " emissions)"
}

// writeChecks names what an actionable group already tried, which is what makes a
// recurrence readable: a reader learns what a new predicate would have to do that these
// do not.
func writeChecks(c *Config, g *bundle.MissGroup) {
	if !g.Actionable {
		return
	}
	if len(g.ChecksRun) == 0 {
		_, _ = fmt.Fprintf(c.Stdout,
			"\ta deterministic path declined and named no checks\n")
		return
	}
	_, _ = fmt.Fprintf(c.Stdout,
		"\tchecks that ran and decided nothing: %s — a predicate that answered this"+
			" would shrink the model's surface area\n", strings.Join(g.ChecksRun, ", "))
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("miss: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("miss: %w", c.Usage(cause))
}
