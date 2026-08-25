// Package logcmd implements the "log" CLI command.
package logcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/okflog"
)

// Config holds the configuration for the log command.
type Config struct {
	*root.Config

	// Add is a note to file under today's date. Empty reads instead of writing.
	Add string

	// Since filters to entries on or after a date, in YYYY-MM-DD form.
	Since string

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the log command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("log").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Add, 'a', "add", "", "file a note under today's date")
	cfg.Flags.StringVar(&cfg.Since, 's', "since", "", "only entries on or after YYYY-MM-DD")
	cfg.Command = &ff.Command{
		Name:      "log",
		Usage:     "gnosis log [--since YYYY-MM-DD] | gnosis log --add \"note\"",
		ShortHelp: "read or append the corpus history",
		LongHelp: `Read log.md, or file a note in it.

This is the corpus's own history and it is **committed**, which distinguishes it
from the audit trail. The trail in .gnosis/audit.jsonl records mechanically what
each process did, per user and gitignored; this records what the corpus decided,
in prose, for a colleague reading in six months. Merging the trail into git would
conflict on every pull and tell nobody anything.

The case that makes this load-bearing is a threshold change. SPEC §6.2 requires
that a value in standards/ moved in the finding-reducing direction be recorded
here with the finding count before and after. Git already has the diff; what git
cannot show is whether a threshold was wrong or merely inconvenient, and that is
exactly what nobody reconstructs a year later.

Entries are OKF §9 date headings, newest first. A note filed today joins today's
section or starts it.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec reads or appends.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) > 0 {
		return c.usage(errors.New("log takes no arguments; use --add to file a note"))
	}
	if c.Add != "" {
		return c.add(ctx)
	}
	return c.read()
}

// read renders the log, or the part of it the caller asked for.
func (c *Config) read() error {
	src, err := os.ReadFile(filepath.Join(c.Bundle, bundle.LogFile))
	if err != nil {
		if os.IsNotExist(err) {
			// Not an error. OKF §9 makes log.md optional, and a corpus that has
			// not needed one yet is in a normal state.
			return c.report(nil)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	_, entries := okflog.Parse(string(src))
	return c.report(okflog.Since(entries, c.Since))
}

// add files a note under today's date.
//
// It takes the writer lock. log.md is committed and at the bundle root, so two
// processes appending at once would interleave — and SPEC §4.6 states the rule
// this follows from: the writer owns the bundle, not merely the database.
func (c *Config) add(ctx context.Context) error {
	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	if wErr := w.Log(time.Now().UTC(), c.Add); wErr != nil {
		return c.fail(root.ReasonNoBundle, wErr)
	}
	if c.JSONL {
		if eErr := c.EmitOK(map[string]any{"added": c.Add}); eErr != nil {
			return fmt.Errorf("log: %w", eErr)
		}
		return nil
	}
	_, _ = fmt.Fprintf(c.Stdout, "filed under %s\n", time.Now().UTC().Format(time.DateOnly))
	return nil
}

// report renders entries.
func (c *Config) report(entries []okflog.Entry) error {
	if c.JSONL {
		if err := c.EmitOK(map[string]any{"entries": entries}); err != nil {
			return fmt.Errorf("log: %w", err)
		}
		return nil
	}
	if len(entries) == 0 {
		// Said rather than left blank: silence would be ambiguous between "no log"
		// and "nothing since that date".
		_, _ = fmt.Fprintf(c.Stdout, "no entries%s\n", sinceSuffix(c.Since))
		return nil
	}
	for i := range entries {
		_, _ = fmt.Fprintf(c.Stdout, "## %s\n", entries[i].Date)
		for _, line := range entries[i].Lines {
			_, _ = fmt.Fprintf(c.Stdout, "%s\n", line)
		}
		_, _ = fmt.Fprintln(c.Stdout)
	}
	return nil
}

func sinceSuffix(since string) string {
	if strings.TrimSpace(since) == "" {
		return ""
	}
	return " on or after " + since
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("log: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("log: %w", c.Usage(cause))
}
