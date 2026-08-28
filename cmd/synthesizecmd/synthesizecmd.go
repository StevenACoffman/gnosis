// Package synthesizecmd implements the "synthesize" CLI command.
package synthesizecmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/relay"
)

// longHelp is the command's prose.
const longHelp = `Emit a rewrite prompt for one concept.

Synthesis is the gated half of accretion (SPEC §6.3). Appending evidence to a
concept needs no model and cannot lose anything: the quotation validates or it
does not. Rewriting a body is replacement, and it is where a model silently drops
what it did not think important.

So the reply is gated on evidence rather than on prose. A rewrite may reorganise,
reword, merge or split the claims it likes; what it may not do is arrive without a
quotation the document already held. That check is a set comparison and not a
count, because a rewrite that dropped one passage and added another would balance.

gnosis never calls a model. This writes a prompt under .gnosis/prompts/ and prints
the key; answer it and hand the reply to ` + "`gnosis admit`" + `, which applies the
gate and reports the diff before writing anything.

The document is hashed when the prompt is emitted. If it changes before the reply
arrives, the reply is refused: an answer computed against bytes that are gone is
the window §9.4 closes for promotion, one level up.

Takes the lock, because emitting a prompt writes one.`

// Config holds the configuration for the synthesize command.
type Config struct {
	*root.Config

	// Model and ModelVersion identify what will answer, and are part of the cache
	// key (§6.1).
	Model        string
	ModelVersion string

	// Flags and Command are this command's own, and declaring them is not
	// boilerplate.
	//
	// **`root.Config` has fields of both names, and an embedded field is reachable
	// by the same selector.** Without these, `cfg.Flags = ...` assigns the *root's*
	// flag set and `cfg.Command = ...` replaces the root command — so registering
	// this command silently made every other one require `--model`. The same
	// shadowing produced `admitcmd`'s `FromStdin` and the schema command's empty
	// command list; this is its third appearance, and the first where it broke a
	// binary rather than one message.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload.
type Result struct {
	// Path is the concept the prompt asks about.
	Path string `json:"path"`

	// Key is what the reply must be admitted under.
	Key string `json:"key"`

	// Prompt is where the question was written, bundle-relative.
	Prompt string `json:"prompt,omitempty"`

	// Cached reports that this question was already answered, so no prompt was
	// written — §6.1's promise that a second run over unchanged inputs costs
	// nothing.
	Cached bool `json:"cached"`
}

// New registers the synthesize command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("synthesize").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Model, 'm', "model", "",
		"the model that will answer; part of the cache key")
	cfg.Flags.StringVar(&cfg.ModelVersion, 0, "model-version", "",
		"the model's version; part of the cache key")
	cfg.Command = &ff.Command{
		Name:      "synthesize",
		Usage:     "gnosis synthesize <path>",
		ShortHelp: "emit a gated rewrite prompt for one concept",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: take the lock, emit, report.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New(
			"synthesize takes exactly one concept path; try `gnosis synthesize c/retry.md`"))
	}
	if c.Model == "" {
		return c.usage(errors.New("--model is required; it is part of the cache key"))
	}

	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	pending, err := w.RewritePrompt(&bundle.RewriteOptions{
		Model: relay.Model{Name: c.Model, Version: c.ModelVersion},
		Path:  args[0],
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(&Result{
		Path: args[0], Key: pending.Key, Prompt: pending.Prompt, Cached: pending.Cached,
	})
}

// report renders the outcome.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("synthesize: %w", err)
		}
		return nil
	}
	if result.Cached {
		_, _ = fmt.Fprintf(c.Stdout,
			"%s: already answered under key %s\n", result.Path, result.Key)
		return nil
	}
	_, _ = fmt.Fprintf(c.Stdout, "%s\n", result.Prompt)
	_, _ = fmt.Fprintf(c.Stderr,
		"\nAnswer it, then: gnosis admit --key %s <reply-file>\n"+
			"The rewrite is refused if it drops a quotation %s already holds.\n",
		result.Key, result.Path)
	return nil
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("synthesize: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("synthesize: %w", c.Usage(cause))
}
