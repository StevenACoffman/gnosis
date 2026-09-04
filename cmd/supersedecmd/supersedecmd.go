// Package supersedecmd implements the "supersede" CLI command.
package supersedecmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// longHelp is the command's prose, extracted from New because a page of help text
// inside a constructor puts the function over the length limit and none of it is logic.
const longHelp = `Record that one claim replaced another, keeping the one it replaced.

Supersession, never deletion. The losing claim is marked deprecated — OKF's word for
"kept for links and history; no longer current" — and the winning claim records the
edge. What that buys is the property a corpus exists for: it can always answer what we
believed in March and why we changed. A delete destroys that visibly and a rewrite
destroys it silently.

The two claims may live in one document or in two. When they live in two, the winner's
edge is written first: a crash between the writes then leaves an edge pointing at a
claim still marked current, which is visible and fixed by running the command again.
The other order would leave a deprecated claim nothing supersedes, which cannot be told
from a claim somebody abandoned.

It takes no rationale, and that is deliberate. The reasoning belongs to the warrant —
lint's warrant check reports a claim that supersedes another and records no
gnosis_warrant — so asking for a reason here as well would collect it where no reader
looks while the warrant stayed empty. Adjudicate first, then supersede, or the corpus
will tell you which half is missing.

A claim of an episodic type cannot be superseded. Its claims are ineligible for
conflict detection, because two reports of different moments are both true: a corpus
adjudicating "we set it to 3 in March" against "we set it to 5 in June" would be
adjudicating its own history.

Without --apply it reports what would change and writes nothing. Applying takes the
writer lock.`

// Config holds the configuration for the supersede command.
type Config struct {
	*root.Config

	// LoserClaim and WinnerClaim name the claims within their documents.
	LoserClaim  string
	WinnerClaim string

	// By is who recorded it. An agent may: the supersession is bookkeeping over a
	// decision recorded elsewhere, and the warrant is what requires a person.
	By string

	// Apply writes. Preview is the default for the reason §4.6.2 gives.
	Apply bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the supersede command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("supersede").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.LoserClaim, 0, "loser-claim", "",
		"the claim being replaced, within the first document")
	cfg.Flags.StringVar(&cfg.WinnerClaim, 0, "winner-claim", "",
		"the claim replacing it, within the second document")
	cfg.Flags.StringVar(&cfg.By, 0, "by", "", "who recorded it, as <kind>:<id>")
	cfg.Flags.BoolVar(&cfg.Apply, 0, "apply", "write the supersession rather than preview it")
	cfg.Command = &ff.Command{
		Name: "supersede",
		Usage: "gnosis supersede --loser-claim ID --winner-claim ID --by ACTOR" +
			" <LOSER-PATH> <WINNER-PATH> [--apply]",
		ShortHelp: "record that one claim replaced another, keeping the one replaced",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: parse the actor, hand the command over, render.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return c.usage(errors.New("supersede needs two document paths after the " +
			"flags — the losing document then the winning one; they may be the same file"))
	}
	by, ok := gnosis.ParseActor(c.By)
	if !ok {
		return c.usage(errors.New(
			"--by must be <kind>:<id>, kind one of human, agent, check"))
	}

	coordinator := bundle.Coordinator{Dir: c.Bundle, Warn: c.Stderr, Rules: c.Rules}
	outcome, err := coordinator.Execute(ctx, &command.Supersede{
		LoserPath:   args[0],
		LoserClaim:  c.LoserClaim,
		WinnerPath:  args[1],
		WinnerClaim: c.WinnerClaim,
		By:          by,
		Eff:         effect(c.Apply),
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(outcome)
}

// effect maps the flag to the command's field. Preview is the default because §4.6.2
// requires the zero value not to be the one that writes.
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
			return fmt.Errorf("supersede: %w", err)
		}
		return exitFor(outcome)
	}
	if outcome.Message != "" {
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", outcome.Message)
	} else {
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", supersededLine(outcome))
	}
	if outcome.Status == gnosis.StatusOK && !c.Apply {
		_, _ = fmt.Fprintf(c.Stderr,
			"nothing was written; re-run with --apply to record it\n")
	}
	return exitFor(outcome)
}

// supersededLine is the human sentence for a successful supersession or preview.
//
// It says the loser is *kept*, because that is the part a reader is most likely to be
// unsure about and the part the design turns on — somebody who believes the claim was
// deleted will not go looking for it.
func supersededLine(outcome gnosis.Outcome) string {
	data, _ := outcome.Data.(map[string]any)
	loser, _ := data["loser"].(string)
	winner, _ := data["winner"].(string)
	verb := "would record"
	if done, _ := data["superseded"].(bool); done {
		verb = "recorded"
	}
	return verb + ": " + winner + " supersedes " + loser +
		", which is deprecated and kept"
}

// exitFor maps an outcome to this command's exit code.
func exitFor(outcome gnosis.Outcome) error {
	if outcome.Status == gnosis.StatusOK {
		return nil
	}
	return root.ExitError(root.CodeFindings)
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("supersede: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("supersede: %w", c.Usage(cause))
}
