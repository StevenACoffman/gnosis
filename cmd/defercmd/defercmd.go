// Package defercmd implements the "defer" CLI command.
package defercmd

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

// longHelp is the command's prose, extracted from New for `misscmd`'s reason.
const longHelp = `Record that you saw a contradiction and are not acting on it yet.

SPEC §17.0 spends a state on this because the common failure of a findings system is not
that problems go undetected but that detected problems go unanswered — and silence is
indistinguishable from nobody having looked. A deferral turns that silence into a
record: who saw it, when, and why they are not acting yet.

It resolves nothing. An adjudication decides a contradiction and writes a warrant
(` + "`gnosis adjudicate`" + `); this says only that somebody read this one and chose to
live with it for now. Nothing about either claim is asserted to be right.

The entry lands in the document's ` + "`gnosis_conflicts`" + ` frontmatter, so it travels
to every colleague through the same git pull that carries the claim, and it arrives as a
diff on a page a reviewer is already looking at. §10.7.4's rule is that decisions are
committed and observations are cached: an open conflict is what the check reports freshly
every run, and a decision to live with one is the state no rebuild can re-derive.

  --finding   the contradiction's id, which ` + "`gnosis lint --check conflict`" + ` prints
  --concept   the other document, by identifier — §5.4 names edges by id, never by path
  --by        who is deferring; a human, because deferring suppresses a finding
  --reason    why you are not acting yet

The review queue stops showing a deferred conflict. ` + "`gnosis lint`" + ` does not:
§17.0 makes reviewing the deferred set a different activity from reviewing the open set,
and it is the one that tells a team what it has decided to live with.

Takes the lock, because it writes a document.`

// Config holds the configuration for the defer command.
type Config struct {
	*root.Config

	// Finding, Concept, By and Reason are §17.0's record. All four are required and
	// none has a defensible default.
	Finding string
	Concept string
	By      string
	Reason  string

	// Apply writes. A preview is the default for §4.6.2's reason: the flag's zero
	// value must not be the one that mutates the corpus.
	Apply bool

	// Flags and Command are this command's own — see proofcmd for why declaring them
	// is not boilerplate.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the defer command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("defer").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Finding, 0, "finding", "",
		"the contradiction's id, as `gnosis lint --check conflict` prints it")
	cfg.Flags.StringVar(&cfg.Concept, 0, "concept", "",
		"the other document's identifier")
	cfg.Flags.StringVar(&cfg.By, 0, "by", "", "who is deferring, as human:<id>")
	cfg.Flags.StringVar(&cfg.Reason, 0, "reason", "",
		"why you are not acting on this yet")
	cfg.Flags.BoolVar(&cfg.Apply, 0, "apply",
		"write the deferral; without this the outcome says what would be recorded")
	cfg.Command = &ff.Command{
		Name: "defer",
		Usage: "gnosis defer --finding ID --concept ID --by ACTOR --reason R" +
			" [--apply] <path>",
		ShortHelp: "record living with a contradiction, without resolving it",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: validate, take the lock, execute.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New("defer needs exactly one document path, after the" +
			" flags; try `gnosis defer --finding ID --concept ID --by human:you" +
			" --reason ... c/<id>-<slug>.md`"))
	}
	by, ok := gnosis.ParseActor(c.By)
	if !ok {
		return c.usage(errors.New("--by must be human:<id>; deferring suppresses a" +
			" finding from the review queue, and a machine deciding to live with a" +
			" contradiction is a machine closing the corpus's own findings"))
	}
	concept, err := gnosis.ParseID(c.Concept)
	if err != nil {
		return c.usage(fmt.Errorf(
			"--concept must be the other document's identifier (§5.4 names edges by"+
				" id, never by path): %w", err))
	}

	coordinator := bundle.Coordinator{Dir: c.Bundle, Warn: c.Stderr, Rules: c.Rules}
	outcome, err := coordinator.Execute(ctx, &command.Defer{
		Path:    args[0],
		Concept: concept,
		Finding: strings.TrimSpace(c.Finding),
		By:      by,
		Reason:  c.Reason,
		Eff:     effect(c.Apply),
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(outcome)
}

// effect maps the flag onto the command's gating field.
//
// A function rather than an inline conditional at the one call site, because the mapping
// is the gating decision: `EffectUnset` is rejected by the command, and a caller that
// wrote the zero value here would be refused rather than silently previewing.
func effect(apply bool) command.Effect {
	if apply {
		return command.EffectApply
	}
	return command.EffectPreview
}

// report renders the outcome.
func (c *Config) report(outcome gnosis.Outcome) error {
	if c.JSONL {
		if err := c.EmitOutcome(outcome); err != nil {
			return fmt.Errorf("defer: %w", err)
		}
		if outcome.Status != gnosis.StatusOK {
			return root.ExitError(outcome.Code)
		}
		return nil
	}
	if outcome.Status != gnosis.StatusOK {
		_, _ = fmt.Fprintf(c.Stderr, "%s: %s\n", outcome.Status, outcome.Message)
		return root.ExitError(outcome.Code)
	}
	data, _ := outcome.Data.(map[string]any)
	if deferred, _ := data["deferred"].(bool); !deferred {
		_, _ = fmt.Fprintf(c.Stderr,
			"would defer conflict %v on %v; re-run with --apply to record it\n",
			data["finding"], data["path"])
		return nil
	}
	_, _ = fmt.Fprintf(c.Stdout, "%v\n", data["path"])
	_, _ = fmt.Fprintf(c.Stderr,
		"conflict %v deferred by %v; `gnosis lint` still reports it, because"+
			" reviewing what a team has decided to live with is a different activity"+
			" from reviewing what nobody has looked at\n",
		data["finding"], data["by"])
	return nil
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("defer: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("defer: %w", c.Usage(cause))
}
