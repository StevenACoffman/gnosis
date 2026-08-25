// Package indexcmd implements the "index" CLI command group.
package indexcmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/audit"
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
	Documents int `json:"documents"`
	Drifted   int `json:"drifted"`

	// Sources is how many tier-0 records the projection now holds. Reported
	// separately from Documents because the two answer different questions —
	// how much the corpus says, and how much evidence it holds to say it with.
	Sources int  `json:"sources"`
	Wrote   bool `json:"wrote"`

	// Digest is a content hash of every row the index holds (SPEC §18.3).
	//
	// It is reported because §4.6 makes the index **per-user**: two colleagues at
	// one commit hold different files and must hold the same answers, and until
	// this existed that was a claim with no way to check it. Comparing two digests
	// settles whether a disagreement is about the corpus or about somebody's cache.
	//
	// It is a hash of content rather than of the file, because a SQLite file is not
	// byte-stable — two builds of identical rows differ in page allocation — so a
	// file hash would differ for reasons that say nothing about the corpus.
	Digest string `json:"digest"`
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
	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

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

	if refusal := c.checkFloor(len(indexed), len(docs)); refusal != nil {
		return refusal
	}

	if !c.CheckOnly {
		if err := c.write(ctx, w, db, docs, &result); err != nil {
			return err
		}
	}

	// Reported under --check too. A caller comparing their index against a
	// colleague's is asking a read-only question, and making them write to get the
	// answer would be the opposite of §4.5.
	if result.Digest, err = db.Digest(ctx); err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}
	return c.report(result, drift)
}

// checkFloor refuses a rebuild whose document count collapsed, or reports nil.
//
// Requires: was is the previously indexed count and now is what was found on disk.
// Ensures: nil when the rebuild may proceed, and the refusal to return otherwise.
//
// The previously indexed count needs no separate bookkeeping: it is len(indexed),
// which exec already loaded to compute drift.
//
// Extracted from exec because the linter reported the complexity when the digest
// was added, and it was right — exec had come to hold lock acquisition, opening,
// loading, drift, the floor, the write, the digest, and the report. This is the one
// of those that is a policy decision rather than a step.
func (c *Config) checkFloor(was, now int) error {
	if c.CheckOnly || c.Force {
		return nil
	}
	floor, err := c.floorFraction()
	if err != nil {
		return c.fail(root.ReasonStandardsInvalid, err)
	}
	if index.FloorBreached(was, now, floor) {
		return c.refuse(was, now)
	}
	return nil
}

// write rebuilds both derived tables.
//
// Extracted from exec because the linter reported the complexity, and it was
// right: exec had come to do loading, the floor check, drift, and two writes, and
// only the last of those is what --check turns off.
//
// The documents and the tier-0 projection are rebuilt in one pass but not one
// transaction. That is acceptable because both are derived (§4.5): an interruption
// between them leaves an index that disagrees with the bundle, which is exactly the
// state `--check` reports and a second rebuild repairs.
func (c *Config) write(
	ctx context.Context, w *bundle.Writer, db *index.DB, docs []bundle.Document,
	result *Result,
) error {
	if err := db.Replace(ctx, bundle.Rows(docs), bundle.LinkRows(docs)); err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}
	// Rebuilt from the committed records rather than merged into what was there
	// (§4.3.1): a record deleted from tier 0 must disappear here too.
	sources, err := bundle.SourceRows(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}
	if err = db.ReplaceSources(ctx, sources); err != nil {
		return c.fail(root.ReasonIndexDrift, err)
	}
	result.Sources = len(sources)
	result.Wrote = true
	result.Drifted = 0

	// §15 audits every mutation, and a rebuild is one: it replaces the derived
	// tables wholesale. The actor is a check rather than a person because the tool
	// caused it — `gnosis.KindCheck` exists for exactly this, and §5.5 gives the
	// reason where `findings.opened_by` names one: "a check name is as much an
	// answer as an actor is". Attributing it to whoever typed the command would be
	// less true, and the trail is per-user anyway, so the file is the person.
	//
	// Best-effort, like every other audit row: the rebuild happened, and reporting
	// a bookkeeping failure as the operation's would tell a caller to retry
	// something that succeeded.
	if aErr := w.Audit(&audit.Row{
		At: time.Now().UTC(), Op: audit.OpRebuild, Actor: "check:index-rebuild",
		Paths:   []string{".gnosis/index.db"},
		Outcome: string(root.StatusOK),
		Detail: strconv.Itoa(result.Documents) + " documents, " +
			strconv.Itoa(result.Sources) + " sources",
	}); aErr != nil {
		_, _ = fmt.Fprintf(c.Stderr, "warning: the rebuild was not audited: %v\n", aErr)
		if bundle.AuditLost(aErr) {
			// The append reported success and the row is not on disk, which no
			// other signal reveals. Best-effort covers a *known* gap; it must not
			// cover a trail that lied about writing (§15).
			return root.ExitError(root.CodeError)
		}
	}
	return nil
}

// floorFraction reads the declared floor.
//
// A bundle with no standards file gets the seed, as everywhere else: a floor that
// only applied once somebody wrote a config would protect the corpora least likely
// to have one.
func (c *Config) floorFraction() (float64, error) {
	std, err := bundle.LoadPromoteStandards(c.Bundle)
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

	// The digest goes to stderr, where a person reads it, and to Data for a machine.
	// It is the answer to "do we hold the same index", and a field only `--jsonl`
	// showed would be one nobody comparing notes over a terminal could use.
	_, _ = fmt.Fprintf(c.Stderr, "digest %s\n", result.Digest)
	return nil
}

// fail adapts root's reporting to this command's name, so a diagnostic says
// which command produced it.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("index rebuild: %w", c.Fail(reason, cause))
}
