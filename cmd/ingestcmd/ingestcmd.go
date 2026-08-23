// Package ingestcmd implements the "ingest" CLI command.
package ingestcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/relay"
)

// Config holds the configuration for the ingest command.
type Config struct {
	*root.Config

	// Model and ModelVersion identify what will answer, and are part of the cache
	// key (SPEC §6.1). They are flags rather than configuration because a key
	// component that could change between two lookups is not a key component.
	Model        string
	ModelVersion string

	// CacheOnly refuses to emit any prompt whose reply is not already cached and
	// exits non-zero listing what is missing. CI uses it (§6.1).
	CacheOnly bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the ingest command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("ingest").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Model, 'm', "model", "", "the model that will answer")
	cfg.Flags.StringVar(&cfg.ModelVersion, 0, "model-version", "", "its version")
	cfg.Flags.BoolVar(&cfg.CacheOnly, 0, "cache-only",
		"emit nothing; fail listing prompts with no cached reply")
	cfg.Command = &ff.Command{
		Name:      "ingest",
		Usage:     "gnosis ingest <URI>...",
		ShortHelp: "emit extraction prompts for archived sources",
		LongHelp: `Emit one extraction prompt per source and stop.

gnosis never calls a model. ingest writes prompts under .gnosis/prompts/ and
suspends; an agent, a person, or a script answers them; ` + "`gnosis admit`" + ` consumes
the replies. That is the whole relay, and it is two commands rather than one
because the tool has no network path to a model and should not acquire one.

Prompts are built from the **archived text**, never from the live source. A
prompt built from a page nobody kept would produce quotations nobody can verify,
so every claim it yielded would be unverifiable by construction.

Each prompt is keyed on the source's content hash, the prompt's own hash, and the
model and version that will answer. A prompt whose reply is already cached is not
written again: re-emitting an answered question invites answering it twice, which
is the cost the cache exists to avoid. A run in which every prompt is cached makes
no model calls at all and reproduces byte-identically.

The model is part of the key on purpose. A reply is a claim about what a
particular model said about a particular text, so serving one model's answer to
another model's question is not a cache hit — it is a substitution nobody was told
about. When the model changes, the corpus re-asks.

--cache-only emits nothing and exits non-zero listing the prompts with no cached
reply, so a CI job can assert that a run needs no new model calls.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: take the lock, render prompts, report.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return c.usage(errors.New(
			"ingest needs at least one source URI; fetch it first with `gnosis fetch`"))
	}
	if strings.TrimSpace(c.Model) == "" {
		// Not defaulted. A default model would put a value nobody chose into every
		// cache key, and the first person to change it would silently invalidate
		// the whole cache without having decided to.
		return c.usage(errors.New("--model is required; it is part of the cache key"))
	}

	lock, err := bundle.AcquireWriterLock(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer lock.Release()

	pending, err := bundle.PromptsFor(c.Bundle, &bundle.PromptOptions{
		Model:     relay.Model{Name: c.Model, Version: c.ModelVersion},
		URIs:      args,
		CacheOnly: c.CacheOnly,
	})
	if err != nil {
		return c.fail(root.ReasonFetchFailed, err)
	}
	return c.report(pending)
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("ingest: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("ingest: %w", c.Usage(cause))
}
