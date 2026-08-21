// Package indexcmd implements the "index" CLI command group.
package indexcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// Config holds the configuration for the index command group.
type Config struct {
	*root.Config
	CheckOnly bool
	Flags     *ff.FlagSet
	Command   *ff.Command
}

// Result is the payload for a rebuild or a check.
type Result struct {
	Documents int  `json:"documents"`
	Drifted   int  `json:"drifted"`
	Wrote     bool `json:"wrote"`
}

// New registers the index command group under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("rebuild").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.CheckOnly, 0, "check",
		"report whether the index matches the bundle; write nothing")

	rebuild := &ff.Command{
		Name:      "rebuild",
		Usage:     "gnosis index rebuild [--check]",
		ShortHelp: "rebuild the derived index from the bundle",
		LongHelp: `Rebuild the derived index from the bundle and the evidence archive.

The index is a cache, not a store: every fact in it comes from a document that
carries its own identifier, so deleting the database and rebuilding is always
safe and always produces the same result.

With --check nothing is written. The command reports whether the index still
describes the bundle, which is what a CI job runs.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	cfg.Command = &ff.Command{
		Name:        "index",
		Usage:       "gnosis index <SUBCOMMAND>",
		ShortHelp:   "inspect and rebuild the derived index",
		Subcommands: []*ff.Command{rebuild},
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: load the bundle, compare, write unless --check.
func (c *Config) exec(ctx context.Context, _ []string) error {
	db, err := bundle.OpenIndex(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	defer func() { _ = db.Close() }()

	docs, err := bundle.Load(os.DirFS(c.Bundle))
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	indexed, err := db.Indexed(ctx)
	if err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}

	drift := len(bundle.Reconciled(docs, indexed))
	result := Result{Documents: len(docs), Drifted: drift}

	if !c.CheckOnly {
		if err := db.Replace(ctx, bundle.Rows(docs), bundle.LinkRows(docs)); err != nil {
			return c.fail(root.ReasonIndexDrift, err)
		}
		result.Wrote = true
		result.Drifted = 0
	}
	return c.report(result, drift)
}

// report renders the outcome. Under --check, drift is a finding rather than an
// error: the index being stale is a fact about the corpus, not a tool failure,
// and a CI job needs to tell those apart.
func (c *Config) report(result Result, drift int) error {
	driftedAndChecking := c.CheckOnly && drift > 0

	if c.JSONL {
		if !driftedAndChecking {
			if err := c.EmitOK(result); err != nil {
				return fmt.Errorf("index rebuild: %w", err)
			}
			return nil
		}
		if err := c.EmitFindings(root.ReasonIndexDrift,
			"the index does not describe the bundle", result); err != nil {
			return fmt.Errorf("index rebuild: %w", err)
		}
		return root.ExitError(root.CodeFindings)
	}

	switch {
	case result.Wrote:
		_, _ = fmt.Fprintf(c.Stdout, "indexed %d document(s)\n", result.Documents)
	case driftedAndChecking:
		_, _ = fmt.Fprintf(c.Stderr, "index is stale: %d document(s) differ\n", drift)
		return root.ExitError(root.CodeFindings)
	default:
		_, _ = fmt.Fprintf(c.Stdout, "index matches the bundle (%d document(s))\n",
			result.Documents)
	}
	return nil
}

// fail adapts root's reporting to this command's name, so a diagnostic says
// which command produced it.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("index rebuild: %w", c.Fail(reason, cause))
}
