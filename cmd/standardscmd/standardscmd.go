// Package standardscmd implements the "standards" CLI command.
package standardscmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// Config holds the configuration for the standards command.
type Config struct {
	*root.Config

	// Since is the revision to compare against. HEAD by default, which answers
	// "what have I loosened but not committed" — the question a person asks
	// before writing a commit message.
	Since string

	// Log files an entry in log.md for each loosening found.
	Log bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the standards command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("check").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Since, 's', "since", "HEAD", "the revision to compare against")
	cfg.Flags.BoolVar(&cfg.Log, 0, "log", "file an entry in log.md for each loosening")

	check := &ff.Command{
		Name:      "check",
		Usage:     "gnosis standards check [--since REV] [--log]",
		ShortHelp: "report thresholds that moved in the finding-reducing direction",
		LongHelp: `Compare this bundle's standards against a git revision and report what loosened.

SPEC §6.2 is the reason this exists. standards/ makes two runs over one corpus
agree, and that property is silent about whether the thresholds are any good: a
corpus can be made to lint clean by widening a cap, and every run afterwards will
be perfectly reproducible and perfectly quiet. So a value moved in the
finding-reducing direction has to be recorded, with what it cost.

Only loosenings are reported. A tightening needs no defence, and listing both
would bury the answer.

The previous values come from git, because standards/ is hand-edited and gnosis
does not own the writes. That means this needs a worktree and a revision that has
the files.

**The finding count is reported only where it exists.** §6.2 asks for the count
before and after, and for most thresholds there is no such number: the allowlist
and the size caps govern what enters tier 0 rather than what any check reports, and
the gate thresholds govern promotion. Where a loosening genuinely cannot change a
count, this says so rather than printing a zero delta — which would read as "it
cost nothing" when what happened is that nothing measured it.

--log files one line per loosening under today's date in log.md, which is the
committed record §6.2 asks for.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	cfg.Command = &ff.Command{
		Name:        "standards",
		Usage:       "gnosis standards <SUBCOMMAND>",
		ShortHelp:   "inspect the corpus's declared thresholds",
		Subcommands: []*ff.Command{check},
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec compares, reports, and optionally files.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) > 0 {
		return c.usage(errors.New("standards check takes no arguments"))
	}

	found, err := bundle.Loosenings(c.Bundle, c.Since)
	if err != nil {
		return c.fail(root.ReasonStandardsInvalid, err)
	}
	if c.Log && len(found) > 0 {
		if err := c.file(ctx, found); err != nil {
			return err
		}
	}
	return c.report(found)
}

// file appends one entry per loosening under today's date.
//
// It takes the writer lock: log.md is committed and at the bundle root, so two
// processes appending at once would interleave (§4.6).
func (c *Config) file(ctx context.Context, found []bundle.Loosened) error {
	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	notes := make([]string, 0, len(found))
	for i := range found {
		notes = append(notes, found[i].LogEntry())
	}
	if wErr := w.Log(time.Now().UTC(), notes...); wErr != nil {
		return c.fail(root.ReasonNoBundle, wErr)
	}
	return nil
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("standards check: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("standards check: %w", c.Usage(cause))
}
