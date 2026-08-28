// Package quarantinecmd implements the "quarantine" CLI command.
package quarantinecmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Config holds the configuration for the quarantine command.
type Config struct {
	*root.Config

	// DiscardPath names a draft to drop. Empty lists the queue instead.
	//
	// It is a flag on this command rather than a verb of its own because listing
	// what is waiting and dropping one of the things waiting are the same job from
	// the reader's side: you run `quarantine`, you see a refused entry, you drop
	// it. A separate `gnosis discard` would be a second place to look for tier 1.
	DiscardPath string

	// By is who is dropping it, and Reason is why. Both required for a discard,
	// and neither is defaulted: a discard whose actor gnosis supplied would be a
	// trail row nobody can be asked about, which is the event §15 exists to
	// prevent.
	By     string
	Reason string

	// Apply performs the discard. Without it the command reports what it would
	// drop and drops nothing — the same preview-and-apply shape every other write
	// here has (§4.6.2), and it matters more for this one: tier 1 is not committed,
	// so there is no `git checkout` after a mistake.
	Apply bool

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
	cfg.Flags.StringVar(&cfg.DiscardPath, 0, "discard", "",
		"drop this draft instead of listing the queue")
	cfg.Flags.StringVar(&cfg.By, 0, "by", "",
		"who is discarding, as <kind>:<id> — required with --discard")
	cfg.Flags.StringVar(&cfg.Reason, 0, "reason", "",
		"why the draft is being dropped — required with --discard")
	cfg.Flags.BoolVar(&cfg.Apply, 0, "apply",
		"perform the discard; without it the command reports what it would drop")
	cfg.Command = &ff.Command{
		Name:      "quarantine",
		Usage:     "gnosis quarantine [--discard PATH --by ACTOR --reason WHY [--apply]]",
		ShortHelp: "list the documents waiting to be promoted, or drop one",
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
- **refused** — a signal failed. Nothing promotes a refused candidate, at any
  actor, with any phrase. Fix the *input* and re-admit: correct the source, the
  prompt, or the model, and run the relay again. Then drop the old draft with
  --discard.

**There is no --edit, and that is a decision rather than a gap.** Editing
quarantined content by hand is how unvetted text acquires a human's authority
without review: a person who opens a refused draft, changes the sentence the gate
objected to, and promotes the result has produced a document that passed the gate
and was checked against nothing — the quotation validates because they made it
validate. Tier 1 exists to keep model-written text out of the working tree until
something has looked at it, and hand-editing is that requirement arrived at through
the front door. What is re-checked here is always a reply, by the same gate, from
the same evidence.

--discard drops a draft. It needs --by and --reason, because a discard nobody can
be asked about is exactly what the write trail exists to prevent, and a trail of
discards with no account of what was wrong cannot say whether the drafts were junk
or somebody was clearing their queue. Without --apply it reports what it would drop
and drops nothing; tier 1 is not committed, so there is no undo afterwards.

Listing reads only: it takes no lock and never writes, so it is safe to run against
a bundle somebody else is ingesting into. A discard takes the writer lock.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: either drop one draft, or gather the queue and
// render it.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New("quarantine takes no arguments; " +
			"use --discard <path> to drop one, or `gnosis promote <path>` to inspect it"))
	}
	if c.DiscardPath != "" {
		return c.discard(ctx)
	}

	waiting, err := bundle.Review(c.Bundle, c.Rules)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(&Result{Waiting: waiting})
}

// discard drops one draft through the coordinator.
//
// The actor and the reason are not defaulted and not inferred. A discard whose
// actor gnosis supplied would be a trail row nobody can be asked about, which is
// the event §15 exists to prevent; the usage error naming both is cheaper than a
// row that says a check dropped somebody's draft.
func (c *Config) discard(ctx context.Context) error {
	by, ok := gnosis.ParseActor(c.By)
	if !ok {
		return c.usage(errors.New(
			"--by must be <kind>:<id>, kind one of human, agent, check"))
	}
	if strings.TrimSpace(c.Reason) == "" {
		return c.usage(errors.New("--reason is required; a discard with no account " +
			"of what was wrong leaves a trail nobody can read"))
	}

	coordinator := bundle.Coordinator{Dir: c.Bundle, Warn: c.Stderr, Rules: c.Rules}
	outcome, err := coordinator.Execute(ctx, &command.Discard{
		Path:   c.DiscardPath,
		Eff:    effect(c.Apply),
		By:     by,
		Reason: c.Reason,
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.reportDiscard(outcome)
}

// effect maps the flag to the command's field. Preview is the default for the
// reason §4.6.2 gives: a write is a command differing in one field, and the field's
// zero value must not be the one that writes.
func effect(apply bool) command.Effect {
	if apply {
		return command.EffectApply
	}
	return command.EffectPreview
}

// reportDiscard renders the outcome of a drop.
//
// A preview is `ok`: asking what would be dropped is a legitimate question with a
// successful answer, and reporting it as blocked would make the safe path look like
// the failing one.
func (c *Config) reportDiscard(outcome gnosis.Outcome) error {
	if c.JSONL {
		if err := c.EmitOutcome(outcome); err != nil {
			return fmt.Errorf("quarantine: %w", err)
		}
		return exitFor(outcome)
	}
	if outcome.Status != gnosis.StatusOK {
		_, _ = fmt.Fprintf(c.Stderr, "%s: %s\n", outcome.Reason, outcome.Message)
		return exitFor(outcome)
	}
	if c.Apply {
		_, _ = fmt.Fprintf(c.Stdout, "discarded %s\n", c.DiscardPath)
		// Which record it went in is the part worth saying, and it depends on who
		// declined: a person's decline is a decision and lands in the committed
		// log; an agent's is housekeeping and stays in the per-user trail. Saying
		// only "the audit trail" was true before the log entry existed and is now
		// the less important half.
		_, _ = fmt.Fprintf(c.Stderr, "%s Re-admit a corrected reply to replace it.\n",
			recordedIn(c.By))
		return nil
	}
	_, _ = fmt.Fprintf(c.Stdout, "would discard %s\n", c.DiscardPath)
	_, _ = fmt.Fprintf(c.Stderr, "Nothing was removed. Add --apply to drop it.\n")
	return nil
}

// exitFor turns a non-ok envelope into the exit code it names, so a machine caller
// branching on the status and one branching on the code agree.
func exitFor(outcome gnosis.Outcome) error {
	if outcome.Code == root.CodeOK {
		return nil
	}
	return root.ExitError(outcome.Code)
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

// recordedIn names where a discard was recorded, which depends on who made it.
//
// §10.7.4 puts decisions in the committed tier and observations in the per-user
// cache, and a discard is either depending on the actor: a person looked at a draft
// and dropped it, which no re-computation recovers; an agent cleared a reply its own
// gate refused, which is housekeeping. Telling the caller which happened is what
// makes the distinction visible rather than merely implemented.
func recordedIn(by string) string {
	if actor, ok := gnosis.ParseActor(by); ok && actor.IsHuman() {
		return "Recorded in " + bundle.LogFile + " and in the audit trail."
	}
	return "Recorded in the audit trail."
}
