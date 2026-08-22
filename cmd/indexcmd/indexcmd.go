// Package indexcmd implements the "index" CLI command group.
package indexcmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/index"
)

// Config holds the configuration for the index command group.
type Config struct {
	*root.Config
	CheckOnly bool

	// Force proceeds past the rebuild floor. It exists because a real deletion is
	// legitimate and the floor cannot tell one from an accident; what the floor
	// buys is that the accident takes a second command rather than none.
	Force   bool
	Flags   *ff.FlagSet
	Command *ff.Command
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
	cfg.Flags.BoolVar(&cfg.Force, 0, "force",
		"rebuild even when the document count collapsed")

	rebuild := &ff.Command{
		Name:      "rebuild",
		Usage:     "gnosis index rebuild [--check]",
		ShortHelp: "rebuild the derived index from the bundle",
		LongHelp: `Rebuild the derived index from the bundle and the evidence archive.

The index is a cache, not a store: every fact in it comes from a document that
carries its own identifier, so deleting the database and rebuilding is always
safe and always produces the same result.

With --check nothing is written. The command reports whether the index still
describes the bundle, which is what a CI job runs.

A rebuild that finds far fewer documents than the index already held **refuses**,
naming both counts. Being a cache is what makes the index safe to destroy, and it
is also what makes destroying it unnoticeable: a wrong --bundle, a partial clone,
or an unstaged c/ produces a rebuild that does exactly what it was told and leaves
an index describing nothing, over the only artifact that showed what was there.
--force is for the case where the corpus really did shrink.`,
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
//
// The writer lock is taken even under --check, because opening the index applies
// any pending migration and that is a write (SPEC §4.6: the writer owns the
// bundle, not merely the database). A --check that raced a rebuild would compare
// against a schema being changed underneath it.
func (c *Config) exec(ctx context.Context, _ []string) error {
	lock, err := bundle.AcquireWriterLock(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer lock.Release()

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

	// The previously indexed count is the last count this corpus verified, so the
	// floor needs no separate bookkeeping: it is len(indexed), already loaded.
	if !c.CheckOnly && !c.Force {
		floor, ferr := c.floorFraction()
		if ferr != nil {
			return c.fail(root.ReasonStandardsInvalid, ferr)
		}
		if index.FloorBreached(len(indexed), len(docs), floor) {
			return c.refuse(len(indexed), len(docs))
		}
	}

	if !c.CheckOnly {
		if err := db.Replace(ctx, bundle.Rows(docs), bundle.LinkRows(docs)); err != nil {
			return c.fail(root.ReasonIndexDrift, err)
		}
		result.Wrote = true
		result.Drifted = 0
	}
	return c.report(result, drift)
}

// floorFraction reads the declared floor.
//
// A bundle with no standards file gets the seed, as everywhere else: a floor that
// only applied once somebody wrote a config would protect the corpora least likely
// to have one.
func (c *Config) floorFraction() (float64, error) {
	std, err := bundle.LoadArchiveStandards(c.Bundle)
	if err != nil {
		return 0, fmt.Errorf("index rebuild: %w", err)
	}
	return std.RebuildFloorFraction.Value, nil
}

// refuse reports a breached floor.
//
// Findings rather than an error, and blocked rather than either: the tool worked,
// the corpus is in a state a person must look at, and the repair is a decision
// rather than a retry. Both counts are named because the number that matters is the
// one the reader did not expect.
func (c *Config) refuse(previous, current int) error {
	message := "rebuild would drop from " + strconv.Itoa(previous) + " documents to " +
		strconv.Itoa(current) + "; refusing. Check --bundle and that c/ is present, " +
		"then use --force if the corpus really shrank"

	if c.JSONL {
		if err := c.EmitBlocked(root.ReasonNeedsHuman, message,
			Result{Documents: current, Drifted: previous}); err != nil {
			return fmt.Errorf("index rebuild: %w", err)
		}
		return root.ExitError(root.CodeBlocked)
	}
	_, _ = fmt.Fprintf(c.Stderr, "error: %s\n", message)
	return root.ExitError(root.CodeBlocked)
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
