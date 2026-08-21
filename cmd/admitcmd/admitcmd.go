// Package admitcmd implements the "admit" CLI command.
package admitcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// Config holds the configuration for the admit command.
type Config struct {
	*root.Config

	// Key is the cache key of the prompt this reply answers.
	Key string

	// Submitter is who supplied the reply.
	Submitter string

	// DryRun maps onto command.EffectPreview. The flag is named for what a person
	// types and the field it sets is the one whose zero value fails closed
	// (SPEC §4.6.2) — the CLI's convention and the command's discipline are
	// different concerns and this is where they meet.
	DryRun bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the admit command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("admit").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Key, 'k', "key", "", "the prompt key this reply answers")
	cfg.Flags.StringVar(&cfg.Submitter, 's', "submitter", "",
		"who supplied the reply, as <kind>:<id>")
	cfg.Flags.BoolVar(&cfg.DryRun, 'n', "dry-run", "check the reply without writing")
	cfg.Command = &ff.Command{
		Name:      "admit",
		Usage:     "gnosis admit --key <key> --submitter <actor> <reply-file>",
		ShortHelp: "check an agent's reply and write it to quarantine",
		LongHelp: `Consume a reply to an extraction prompt.

The reply is cached under its key whatever happens next. The model call is
already spent, and discarding an answer because it turned out to be unusable
would make you pay again to learn the same thing.

Then, in this order: the reply is parsed strictly, its claims are **segmented**,
and only then are quotations checked. That ordering is not incidental. "The cache
is enabled by default, but it is not shared across sessions" is one sentence and
two assertions, and a verifier that attaches one verdict to it reports the whole
sentence supported when a quotation validates only the first half — a silent false
pass in the check the corpus most depends on.

Three outcomes, and the third is the one worth knowing about. A claim whose
quotations are found in the archive passes. A claim whose quotations are looked
for and not found is **unsupported**, and blocks. A claim whose quotations could
not be checked at all — every passage too short to be evidence, or no archived
text to check against — is **unchecked**, and also blocks, separately reported.
"Nobody looked" is not the claim "this is fine", and reporting the two alike would
accuse an agent of fabricating a quotation that may well be accurate.

On pass the document goes to quarantine, not to the corpus. Getting it the rest of
the way is ` + "`gnosis promote`" + `, behind the promote gate.

A reply is rejected whole or accepted whole. A partially applied reply would put
content into quarantine that neither the agent nor a reader believes they
approved, and quarantine is one gate away from the corpus.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: read the reply, build the command, execute.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New("admit needs exactly one reply file"))
	}
	if strings.TrimSpace(c.Key) == "" {
		return c.usage(errors.New(
			"--key is required; `gnosis ingest` prints the key for each prompt"))
	}
	submitter, ok := gnosis.ParseActor(c.Submitter)
	if !ok {
		return c.usage(errors.New(
			"--submitter must be <kind>:<id>, kind one of human, agent, check"))
	}

	reply, err := os.ReadFile(args[0])
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}

	coordinator := bundle.Coordinator{Dir: c.Bundle}
	outcome, err := coordinator.Execute(ctx, &command.Admit{
		Key:       c.Key,
		Reply:     string(reply),
		Eff:       effect(c.DryRun),
		Submitter: submitter,
	})
	if err != nil {
		return c.fail(root.ReasonNeedsHuman, err)
	}
	return c.report(outcome)
}

// effect maps the CLI's flag onto the command's field.
//
// The mapping is explicit rather than a cast because the two encode opposite
// defaults on purpose: a flag's zero value is "not set", and an Effect's zero
// value is rejected. Writing it out is what keeps a future `DryRun` default change
// from silently becoming a live write.
func effect(dryRun bool) command.Effect {
	if dryRun {
		return command.EffectPreview
	}
	return command.EffectApply
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("admit: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("admit: %w", c.Usage(cause))
}
