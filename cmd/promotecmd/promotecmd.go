// Package promotecmd implements the "promote" CLI command.
package promotecmd

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

// Config holds the configuration for the promote command.
type Config struct {
	*root.Config

	// Apply opts into the write. Preview is the default and that direction is
	// deliberate: a command that writes unless told not to is the shape §9.4
	// argues against, and the cost of the two defaults is asymmetric — a
	// surprising preview wastes a second, a surprising write enters the corpus.
	Apply bool

	// Approver is who authorises this, as <kind>:<id>.
	Approver string

	// Rationale is why. Required when the gate could not check everything.
	Rationale string

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the promote command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("promote").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Apply, 0, "apply", "write the document; preview otherwise")
	cfg.Flags.StringVar(&cfg.Approver, 'a', "approver", "",
		"who authorises this, as <kind>:<id>")
	cfg.Flags.StringVar(&cfg.Rationale, 'r', "rationale", "",
		"why, when the gate could not check everything")
	cfg.Command = &ff.Command{
		Name:      "promote",
		Usage:     "gnosis promote [--apply --approver <actor>] <path>",
		ShortHelp: "move a quarantined document into the corpus, behind the gate",
		LongHelp: `Run the promote gate over a quarantined document and, on approval, write it.

Preview is the default. ` + "`--apply`" + ` opts into the write.

The gate approves a **diff**, not a document: the bytes it judges are the bytes
written, read once, with no re-read in between. Preview and apply are the same
code path down to the final branch, so what you saw is what lands.

Three outcomes.

**approved** — every signal passed. Nothing further is needed.

**refused** — a signal failed. There is no way through this. No flag, no actor,
and no confirmation phrase promotes a document whose quotation is not in the
archive, because none of those makes it true. Fix the document and re-admit it.

**needs_human** — every signal that could run passed, and some could not. §10's
conflict adjudication is unbuilt, and §9.3's scan runs one of its four stages, so
this is the ordinary outcome today rather than an unusual one. A person may carry
it, and carrying it requires three things: an approver who is a ` + "`human:`" + `,
a rationale, and typing the document's path when prompted. Not "yes" — a
confirmation you can supply from muscle memory confirms nothing.

The signals a promotion was carried over are written into the audit trail. That
is what makes this a debt rather than a bypass: when conflict adjudication lands,
every document admitted without it can be found and re-examined.

With ` + "`--jsonl`" + ` there is no prompt, because a machine cannot type a
phrase and a prompt on a pipe hangs. A candidate needing one is reported blocked,
with the requirement in the envelope.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: preview, confirm if required, then apply.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New("promote needs exactly one quarantined path"))
	}
	path := args[0]

	approver, ok := gnosis.ParseActor(c.Approver)
	if !ok && (c.Apply || strings.TrimSpace(c.Approver) != "") {
		return c.usage(errors.New(
			"--approver must be <kind>:<id>, kind one of human, agent, check"))
	}

	coordinator := bundle.Coordinator{Dir: c.Bundle, Warn: c.Stderr}
	cmd := &command.Promote{
		Path:      path,
		Eff:       effect(c.Apply),
		Approver:  approver,
		Rationale: c.Rationale,
	}
	if !c.Apply {
		// A preview needs an approver on the command because Validate rejects an
		// unset one, and inventing a person to preview with would be worse than
		// naming the check that is doing it.
		cmd.Approver = previewActor(approver)
	}

	outcome, err := coordinator.Execute(ctx, cmd)
	if err != nil {
		return c.fail(root.ReasonNeedsHuman, err)
	}
	if c.Apply && needsConfirmation(outcome) {
		outcome, err = c.confirmAndRetry(ctx, &coordinator, cmd, outcome)
		if err != nil {
			return err
		}
	}
	return c.report(outcome)
}

// effect maps the CLI's flag onto the command's field.
//
// Written out rather than cast because the two encode opposite defaults on
// purpose: the flag defaults to not-writing, and an Effect's zero value is
// rejected outright.
func effect(apply bool) command.Effect {
	if apply {
		return command.EffectApply
	}
	return command.EffectPreview
}

// previewActor names who is asking, when a preview did not say.
//
// A preview writes nothing, so the approver on it authorises nothing — but the
// command requires one, and the honest filler is the tool itself rather than a
// borrowed human name. `check:` is the kind for exactly this, and it cannot
// satisfy the human path, so a preview can never be mistaken for an authorisation.
func previewActor(supplied gnosis.Actor) gnosis.Actor {
	if supplied != gnosis.ActorUnset {
		return supplied
	}
	return gnosis.Actor("check:promote-preview")
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("promote: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("promote: %w", c.Usage(cause))
}
