// Package askcmd implements the "ask" CLI command.
package askcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/relay"
)

// longHelp is the command's prose, extracted from New for `misscmd`'s reason.
const longHelp = `Emit an answer prompt for one question, or refuse (SPEC §8.3, §17.0.1).

This retrieves the claims that bear on a question and writes a prompt built from them
and nothing else. gnosis never calls a model: answer the prompt, then hand the reply to
` + "`gnosis file`" + `, which puts it through the same gate an ingested source passes.

The interesting half is that it can refuse, and a refusal is an ordinary outcome rather
than an error. A read path that cannot say "the corpus does not support an answer to
this" produces the same shape of output for a question it cannot answer as for one it
can, and the caller cannot tell which they got. Three refusals, because they are three
different situations with three different remedies:

  silent       nothing was retrieved. Nothing is broken; nobody has written this down.
  unevidenced  claims were found and none carries evidence. Fetch and ingest a source.
  unresolved   a retrieved claim is under a challenge nobody has adjudicated. Only this
               one is a contradiction waiting to be settled.

A question is not FTS5 syntax. Its words become the query and its punctuation does not,
so "how many retries?" searches for the words in it rather than failing to parse.

Takes the lock, because emitting a prompt writes one.`

// Config holds the configuration for the ask command.
type Config struct {
	*root.Config

	// Model and ModelVersion identify what will answer, and are part of the key
	// (§6.1).
	Model        string
	ModelVersion string

	// Limit is how many claims to retrieve. Zero takes the default.
	Limit int

	// Flags and Command are this command's own — see proofcmd for why declaring them
	// is not boilerplate.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the ask command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("ask").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Model, 0, "model", "",
		"what will answer; part of the cache key")
	cfg.Flags.StringVar(&cfg.ModelVersion, 0, "model-version", "",
		"the model's version; part of the cache key")
	cfg.Flags.IntVar(&cfg.Limit, 0, "limit", 0,
		"how many claims to retrieve; 0 takes the default")
	cfg.Command = &ff.Command{
		Name:      "ask",
		Usage:     "gnosis ask --model M <QUESTION>",
		ShortHelp: "emit an answer prompt for one question, or say why the corpus cannot",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: take the lock, retrieve, emit or refuse.
func (c *Config) exec(ctx context.Context, args []string) error {
	question := strings.Join(args, " ")
	if err := c.validate(question); err != nil {
		return c.usage(err)
	}

	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	asked, err := w.AskPrompt(ctx, &bundle.AskOptions{
		Question: question,
		Model:    relay.Model{Name: c.Model, Version: c.ModelVersion},
		Limit:    c.Limit,
		Warn:     c.Stderr,
	})
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	return c.report(asked)
}

// validate settles the invocation before the lock is taken, so a mistake in it is
// reported as one rather than after a wait behind somebody's ingest.
func (c *Config) validate(question string) error {
	if strings.TrimSpace(question) == "" {
		return errors.New("ask needs a question; try `gnosis ask --model M how many" +
			" times does the service retry`")
	}
	if strings.TrimSpace(c.Model) == "" {
		// Not defaulted, for `critic`'s reason: a default model would put a value
		// nobody chose into every cache key, and the first person to change it would
		// invalidate the whole cache without having decided to.
		return errors.New("--model is required; it is part of the cache key")
	}
	if c.Limit < 0 {
		return fmt.Errorf("--limit must be positive, got %d", c.Limit)
	}
	return nil
}

// report renders the outcome.
//
// **A refusal exits 0**, and that is §17.0.1's requirement rather than a leniency. A
// non-zero exit would make "the corpus does not know" indistinguishable from "the
// command broke", which is the collapse the whole section exists to prevent — and it
// would make a script's first response to a silent corpus be to retry.
func (c *Config) report(asked *bundle.Asked) error {
	if c.JSONL {
		if err := c.EmitOK(asked); err != nil {
			return fmt.Errorf("ask: %w", err)
		}
		return nil
	}
	if asked.Prompt != nil {
		if asked.Prompt.Cached {
			_, _ = fmt.Fprintf(c.Stdout, "%s\tcached\n", asked.Prompt.Key)
		} else {
			_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\n", asked.Prompt.Key, asked.Prompt.Path)
		}
		_, _ = fmt.Fprintf(c.Stderr,
			"%d claim(s) retrieved; answer the prompt and file it with"+
				" `gnosis file --key %s --response FILE`\n",
			asked.Retrieved, asked.Prompt.Key)
	} else {
		// The state and the remedy, because a refusal naming only what happened
		// leaves the reader to work out what to do about it — and the three refusals
		// have three different answers.
		_, _ = fmt.Fprintf(c.Stderr, "%s: %s\n",
			asked.Answerability, gnosis.Remedy(asked.Answerability))
	}
	writeShortfalls(c, asked)
	return nil
}

// writeShortfalls names what this query could not see.
//
// **Unconditional on the wire and conditional in prose**, which is `search --claims`'s
// split: a JSON reader cannot tell an absent field from a zero one, and a person reading
// "0 claims carry no lead" on every query learns to skip the line.
func writeShortfalls(c *Config, asked *bundle.Asked) {
	if asked.Unextracted > 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"%d claim(s) in this index carry no lead and are invisible to any query;"+
				" `gnosis ingest` is what extracts them\n", asked.Unextracted)
	}
	if asked.Unevidenced > 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"%d retrieved claim(s) offer no passage and were left out of the prompt;"+
				" an answer may not rest on a claim with nothing behind it\n",
			asked.Unevidenced)
	}
	if asked.Deferred > 0 {
		_, _ = fmt.Fprintf(c.Stderr,
			"%d retrieved claim(s) sit under a challenge somebody deferred; the answer"+
				" is assembled from them because deferring is a decision to live with"+
				" it, not an unresolved contest\n", asked.Deferred)
	}
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("ask: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("ask: %w", c.Usage(cause))
}
