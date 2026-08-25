// Package schemacmd implements the "schema" CLI command (SPEC §5.7).
package schemacmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/schema"
	"github.com/StevenACoffman/skillet/errs"
)

// longHelp is the command's prose, kept out of New for the reason auditcmd's is:
// registering a command and explaining it are different jobs.
const longHelp = `Generate and maintain AGENTS.md, the corpus's schema document.

Two regions are generated and no more: the **type vocabulary** this corpus declares,
and the **commands** this binary offers. Everything else in the file is yours.

That restraint is the feature rather than an omission. An ETH Zurich study found
auto-generated context files reduced agent success in five of eight settings, so
gnosis writes the mechanical parts — vocabulary, commands, paths — and never the
prose that tells an agent how to think.

**The marker contract, in four rules.** A generated region is the text between
` + "`<!-- gnosis:begin NAME -->`" + ` and ` + "`<!-- gnosis:end NAME -->`" + `, and
gnosis replaces it. Everything outside every marker is preserved byte for byte — not
re-wrapped, not tidied, because your prose is not this tool's to improve. A file
carrying **no markers is never overwritten**: the generated text is written to
AGENTS.generated.md instead, and you decide what to do with it. And a marker that
opens and never closes is a refusal, because the extent of the region is unknown and
one typo must not hand a whole document to a generator.

A region gnosis generates that your file does not carry is left absent rather than
appended. Where a region belongs in your document is your decision.

--check writes nothing and reports whether the committed file is stale. It is a
finding rather than an error: the examination completed and found something, which is
what a CI job branches on.

link makes AGENTS.md canonical and symlinks each agent's expected filename to it, so
Claude, Gemini, and Qwen read one file and cannot drift. It refuses to replace a
regular file — clobbering a hand-written CLAUDE.md to make a symlink is a data loss no
flag should cause quietly — and repoints an existing symlink, which is what the
command is for.`

// Config holds the configuration for the schema command.
type Config struct {
	*root.Config

	// Check reports drift and writes nothing.
	//
	// A flag rather than a subcommand, for the reason `fetch --dry-run` is one: it is
	// the same operation differing in whether the final write happens, so the thing
	// checked and the thing written cannot come apart.
	Check bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload.
type Result struct {
	// Docs is one entry per marked root document, in a fixed order so two runs
	// report alike.
	Docs []DocResult `json:"docs"`

	// Linked are the agent filenames now pointing at the canonical file.
	Linked []string `json:"linked,omitempty"`
}

// plannedDoc pairs a plan with the committed file it is about.
//
// The pairing is needed because MarkedDoc.Path is the *destination*, which is the
// sibling in the unmarked case — so it cannot answer "which file should the reader
// open", which is what every message here has to say.
type plannedDoc struct {
	source string
	doc    bundle.MarkedDoc
}

// DocResult is what happened to one marked root document.
type DocResult struct {
	// Source is the committed file this is about — the thing a reader edits.
	Source string `json:"source"`

	// Path is the file that was written, or would be. It is the sibling path when
	// the committed file carries no markers, so a caller can tell which happened
	// without re-deriving the rule.
	Path string `json:"path"`

	// Outcome is what the merge did, or why it refused.
	Outcome schema.Outcome `json:"outcome"`

	// Wrote reports whether anything reached the disk. False under --check always.
	Wrote bool `json:"wrote"`

	// Stale reports whether the committed file differs from what gnosis would write.
	Stale bool `json:"stale"`
}

// needsHuman reports whether this document's state is something a caller acts on.
//
// The two cases the envelope and the human form share: a stale file under --check is
// work to do, and a run that wrote the sibling did not do what was asked.
func (d *DocResult) needsHuman(check bool) bool {
	return d.Outcome == schema.OutcomeUnmarked || (check && d.Stale)
}

// agentFiles are the per-agent filenames `link` points at the canonical one.
//
// A list rather than a scan, because the set is "what these tools look for" and no
// directory listing can discover it. It is short, and it changes when a tool changes
// its own convention — a fact about the world rather than about this corpus.
//
// A function rather than a package-level slice, following `bundle.isReserved`: a
// mutable package-level collection is shared state, and the linter is right that a
// caller could append to it.
func agentFiles() []string { return []string{"CLAUDE.md", "GEMINI.md", "QWEN.md"} }

// New registers the schema command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("schema").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Check, 0, "check",
		"report whether the committed file is stale, and write nothing")
	cfg.Command = &ff.Command{
		Name:      "schema",
		Usage:     "gnosis schema [--check] | gnosis schema link",
		ShortHelp: "maintain AGENTS.md and the per-agent symlinks",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: read the vocabulary and the command tree, merge, write.
func (c *Config) exec(ctx context.Context, args []string) error {
	switch {
	case len(args) == 1 && args[0] == "link":
		if c.Check {
			return c.usage(errors.New("--check applies to the generated file, not to link"))
		}
		return c.link()
	case len(args) != 0:
		return c.usage(fmt.Errorf(
			"schema takes no arguments or the word `link`, got %q", args[0]))
	}
	return c.write(ctx)
}

// write merges the generated regions into the committed file.
//
// **The lock is taken here and the first version of this command did not take it.**
// `AGENTS.md` is committed and at the bundle root, so two processes writing it would
// interleave — §4.6's rule that the writer owns the bundle, which `log` already
// follows for the one other committed root file a command writes.
//
// Under `--check` no lock is taken, following `standards check`: it reads, and a
// reader that blocked on a writer would make the CI-facing form of every command
// contend with the ingest it is checking.
func (c *Config) write(ctx context.Context) error {
	plans, err := c.plan()
	if err != nil {
		return c.fail(reasonFor(err), err)
	}
	for i := range plans {
		if plans[i].doc.Outcome == schema.OutcomeMalformed {
			return c.refuseMalformed(plans[i].source)
		}
	}

	result := Result{Docs: make([]DocResult, 0, len(plans))}
	for i := range plans {
		result.Docs = append(result.Docs, DocResult{
			Source:  plans[i].source,
			Path:    plans[i].doc.Path,
			Outcome: plans[i].doc.Outcome,
			Stale:   plans[i].doc.Stale,
		})
	}
	if c.Check {
		return c.report(&result)
	}

	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	for i := range plans {
		if wErr := w.WriteMarkedDoc(&plans[i].doc); wErr != nil {
			return c.fail(root.ReasonNoBundle, wErr)
		}
		result.Docs[i].Wrote = plans[i].doc.Stale
	}
	return c.report(&result)
}

// plan works out what both marked root documents should contain.
//
// Requires: nothing.
// Ensures: writes nothing; one entry per document, in a fixed order.
//
// **Both are planned before either is written**, so a malformed marker in one refuses
// the whole run. Writing the good one first would leave a corpus half-updated by a
// command that then reported a failure, and a reader could not tell which half.
func (c *Config) plan() ([]plannedDoc, error) {
	agents, err := bundle.PlanSchemaDoc(c.Bundle, c.commands())
	if err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	index, err := bundle.PlanIndexDoc(c.Bundle)
	if err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return []plannedDoc{
		{source: bundle.SchemaFile, doc: agents},
		{source: bundle.IndexFile, doc: index},
	}, nil
}

// refuseMalformed reports a marker that opens and never closes, naming it.
//
// Nothing is written, for either document: the region's extent is unknown, and a
// generator that guessed would hand a whole file to itself on one typo.
func (c *Config) refuseMalformed(source string) error {
	name, _ := schema.Unclosed(readOrEmpty(filepath.Join(c.Bundle, source)))
	return c.fail(root.ReasonStandardsInvalid, fmt.Errorf(
		"%s has a `%s` marker that opens and never closes; nothing was written, "+
			"because the region's extent is unknown", source, name))
}

// reasonFor classifies a planning failure: an unloadable vocabulary is a different
// problem from an unreadable bundle, and a caller acts differently on each.
func reasonFor(err error) string {
	if errs.ErrorCode(err) == errs.EINVALID {
		return root.ReasonVocabularyInvalid
	}
	return root.ReasonNoBundle
}

// readOrEmpty reads a file for a diagnostic, treating any failure as empty. It runs
// only on the path that has already decided to refuse, so a second error here would
// replace a precise message with a vaguer one.
func readOrEmpty(full string) string {
	raw, err := os.ReadFile(full)
	if err != nil {
		return ""
	}
	return string(raw)
}

// commands is this binary's command list, read from the registered tree.
//
// `c.Config.Command`, not `c.Command`: this type has its own `Command` field for the
// subcommand it registers, and that shadows the embedded root's. Written the short way
// it compiled, ran, and produced an empty list — "No commands were reported" — because
// the schema command has no subcommands of its own. The same shadowing bit `admitcmd`'s
// `--stdin` flag, which is why that one is named `FromStdin`.
func (c *Config) commands() []schema.CommandEntry {
	subs := c.Config.Command.Subcommands
	out := make([]schema.CommandEntry, 0, len(subs))
	for _, sub := range subs {
		out = append(out, schema.CommandEntry{Name: sub.Name, Help: sub.ShortHelp})
	}
	return out
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("schema: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("schema: %w", c.Usage(cause))
}
