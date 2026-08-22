// Package quarantinecmd implements the "quarantine" CLI command.
package quarantinecmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// Config holds the configuration for the quarantine command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: what is waiting, and what the gate makes of each.
type Result struct {
	Waiting []bundle.Waiting `json:"waiting"`
}

// New registers the quarantine command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("quarantine").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "quarantine",
		Usage:     "gnosis quarantine",
		ShortHelp: "list the documents waiting to be promoted",
		LongHelp: `Show tier 1: documents that were admitted and have not entered the corpus.

Quarantine lives under ` + "`.gnosis/`" + `, not beside the corpus, and that is a
decided constraint rather than a default. **Unvetted text is text an agent will
obey.** A coding agent browsing the repository does not know which directories to
skip, so putting unreviewed content in the working tree would undercut §9.3
entirely. This command is how you see what is there.

Each entry carries the gate's decision, because a list of paths says what is
waiting and not why any of it is stuck — which is the question a reader actually
has. Running the gate per entry is what makes that possible, so this is slower
than a directory listing and worth it.

- **approved** — every signal passed. ` + "`gnosis promote --apply`" + ` writes it.
- **needs_human** — the signals that could run passed and some could not. A person
  may carry it; see ` + "`gnosis promote --help`" + `.
- **refused** — a signal failed. Fix the document and re-admit it; nothing
  promotes a refused candidate.

Reads only. It takes no lock and never writes, so it is safe to run against a
bundle somebody else is ingesting into.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: gather the queue, render it.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New("quarantine takes no arguments; " +
			"use `gnosis promote <path>` to inspect one document"))
	}

	waiting, err := bundle.Review(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(&Result{Waiting: waiting})
}

// report renders the queue.
//
// An empty queue is `ok` and not a finding. Nothing waiting is the state a
// healthy corpus is in most of the time, and reporting it as a finding would make
// the ordinary case look like a problem.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("quarantine: %w", err)
		}
		return nil
	}
	for _, w := range result.Waiting {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s%s\n", w.Decision, w.Path, why(&w))
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d document(s) waiting\n", len(result.Waiting))
	return nil
}

// why renders the signals behind a decision, or nothing for an approved one.
//
// Failures and unrun checks are labelled separately even in one line, because the
// reader's next action differs: one is a document to fix and the other is a
// signature to give.
func why(w *bundle.Waiting) string {
	out := ""
	if len(w.Failed) > 0 {
		out += "\tfailed: " + join(w.Failed)
	}
	if len(w.Unchecked) > 0 {
		out += "\tunrun: " + join(w.Unchecked)
	}
	return out
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("quarantine: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("quarantine: %w", c.Usage(cause))
}
