// Package lintcmd implements the "lint" CLI command.
//
// The package is named lintcmd rather than lint so it does not shadow
// internal/lint, which it imports.
package lintcmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// Config holds the configuration for the lint command.
type Config struct {
	*root.Config
	Check   string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the lint command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("lint").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Check, 0, "check", "",
		"run only the named check; the default runs every applicable one")
	cfg.Command = &ff.Command{
		Name:      "lint",
		Usage:     "gnosis lint [--check NAME]",
		ShortHelp: "report corpus health",
		LongHelp: `Report the health of the knowledge base.

Every check is reported, and so is every check that did NOT run. A check whose
convention the corpus does not yet exhibit — an orphan check on a corpus with no
links, a log-format check on a bundle with no log — is skipped with its reason
stated, because a silent skip is indistinguishable from a clean result.

Findings are not failures. A corpus with blocking findings exits ` +
			strconv.Itoa(int(root.CodeFindings)) + `, distinct
from the ` + strconv.Itoa(int(root.CodeError)) + ` a broken tool exits with, so a CI job can tell
"the corpus has problems" from "gnosis could not run".`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: load, run the pure checks, render.
func (c *Config) exec(ctx context.Context, _ []string) error {
	// Read-only: a bundle with no index is reported as such rather than given
	// one, so linting never leaves state behind.
	idx, err := bundle.LoadIndex(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}
	snap, err := bundle.Snapshot(os.DirFS(c.Bundle), idx)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}

	checks, err := c.selectChecks()
	if err != nil {
		return c.usage(err)
	}

	report := lint.Run(snap, checks)
	if c.JSONL {
		return c.emit(report)
	}
	return c.render(report)
}

// selectChecks narrows the registry to --check when given.
func (c *Config) selectChecks() ([]lint.Check, error) {
	all := lint.Checks()
	if c.Check == "" {
		return all, nil
	}
	for _, ch := range all {
		if ch.Name == c.Check {
			return []lint.Check{ch}, nil
		}
	}
	names := make([]string, 0, len(all))
	for _, ch := range all {
		names = append(names, ch.Name)
	}
	return nil, fmt.Errorf("unknown check %q; known checks are %s",
		c.Check, strings.Join(names, ", "))
}

// emit writes the machine envelope and returns the matching exit code.
func (c *Config) emit(report lint.Report) error {
	blocking := hasBlocking(report.Diagnostics)
	if !blocking {
		if err := c.EmitOK(report); err != nil {
			return fmt.Errorf("lint: %w", err)
		}
		return nil
	}
	if err := c.EmitFindings(reasonFor(report.Diagnostics),
		"the corpus has blocking findings", report); err != nil {
		return fmt.Errorf("lint: %w", err)
	}
	return root.ExitError(root.CodeFindings)
}

// render writes the human form: findings first, then what was skipped.
func (c *Config) render(report lint.Report) error {
	for _, d := range report.Diagnostics {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\t%s\n", d.Severity, d.Category, d.Path, d.Message)
	}
	for _, s := range report.Skipped {
		_, _ = fmt.Fprintf(c.Stderr, "skipped %s: %s\n", s.Check, s.Reason)
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d finding(s), %d check(s) skipped\n",
		len(report.Diagnostics), len(report.Skipped))
	if hasBlocking(report.Diagnostics) {
		return root.ExitError(root.CodeFindings)
	}
	return nil
}

// fail and usage adapt root's reporting to this command's name, so a diagnostic
// says which command produced it.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("lint: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("lint: %w", c.Usage(cause))
}

// hasBlocking reports whether any diagnostic is error-severity.
func hasBlocking(ds []finding.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity.Blocking() {
			return true
		}
	}
	return false
}

// reasonFor picks the machine token for the most significant finding, so an
// agent branches on a token rather than reading messages.
func reasonFor(ds []finding.Diagnostic) string {
	for _, d := range ds {
		if !d.Severity.Blocking() {
			continue
		}
		switch d.Category {
		case "identity":
			return root.ReasonDuplicateIdentity
		case "index-drift":
			return root.ReasonIndexDrift
		case "conformance":
			return root.ReasonUnparsable
		}
	}
	return root.ReasonNeedsHuman
}
