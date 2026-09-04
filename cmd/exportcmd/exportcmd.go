// Package exportcmd implements the "export" CLI command.
package exportcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// The two formats, named once. A caller that misspells one is told both.
const (
	formatOKF   = "okf"
	formatJSONL = "jsonl"
)

// longHelp is the command's prose, extracted from New for `misscmd`'s reason.
const longHelp = `Export the corpus for somebody outside this bundle (SPEC §8.5, §16.3).

Two formats, because two readers want different things.

  --format okf    the portable bundle: every shareable file, copied into --out with
                  its layout intact. A receiver runs gnosis against it directly.
  --format jsonl  one JSON object per concept on stdout: identity, claims, and the
                  archive paths backing them, for a tool that does not want to parse
                  OKF markdown.

Neither carries .gnosis/. That directory holds the audit trail, the prompt cache, the
miss log and the coverage ledger — what one person asked a model and when. It is
per-user and derived, so exporting it would publish a colleague's session history to
whoever the export was for, and it would be different for every colleague at one commit.

The okf form copies the working tree rather than a commit, which is the difference from
` + "`git archive`" + `: a document written and not yet committed is still part of the
corpus somebody is asking for, and a bundle need not be a git repository at all.

An unreadable document is exported carrying the reason it could not be read, never
dropped. A receiver handed a clean-looking export has no way to learn that something is
missing from it.

Reads only. It takes no lock and never writes inside the bundle.`

// Config holds the configuration for the export command.
type Config struct {
	*root.Config

	// Format selects okf or jsonl. Required: there is no default that would be right
	// for both readers, and guessing from --out's presence would make the output shape
	// depend on a flag that is about a location.
	Format string

	// Out is the destination directory for --format okf. Ignored by jsonl, which
	// writes to stdout so it can be piped.
	Out string

	// Flags and Command are this command's own — see proofcmd for why declaring them
	// is not boilerplate.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result reports what left the bundle.
type Result struct {
	Format string `json:"format"`

	// Documents is how many concepts were exported, and Files how many files the okf
	// form copied. Both, because they answer different questions: a receiver checking
	// they got the whole corpus counts documents, and one checking the evidence came
	// too counts files.
	Documents int    `json:"documents"`
	Files     int    `json:"files,omitempty"`
	Out       string `json:"out,omitempty"`
}

// New registers the export command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("export").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Format, 0, "format", "",
		"okf (a portable bundle under --out) or jsonl (one concept per line on stdout)")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"destination directory, required by --format okf")
	cfg.Command = &ff.Command{
		Name:      "export",
		Usage:     "gnosis export --format okf|jsonl [--out DIR]",
		ShortHelp: "export the corpus as a portable bundle or a JSONL stream",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: validate, load, write.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New(
			"export takes no positional arguments; --format and --out carry both values"))
	}
	switch c.Format {
	case formatOKF:
		return c.exportTree()
	case formatJSONL:
		return c.exportRows()
	case "":
		return c.usage(errors.New(
			"export requires --format " + formatOKF + " or --format " + formatJSONL))
	default:
		return c.usage(fmt.Errorf("unknown --format %q; the two are %s and %s",
			c.Format, formatOKF, formatJSONL))
	}
}

// exportTree copies the shareable half of the bundle into --out.
func (c *Config) exportTree() error {
	if c.Out == "" {
		return c.usage(errors.New(
			"--format " + formatOKF + " writes a directory and needs --out"))
	}
	paths, err := bundle.PortablePaths(c.Bundle)
	if err != nil {
		return c.fail(err)
	}
	if len(paths) == 0 {
		return c.fail(errors.New(
			"this bundle holds no shareable file, so an export would be an empty directory"))
	}
	for _, rel := range paths {
		if cErr := copyInto(c.Bundle, c.Out, rel); cErr != nil {
			return c.fail(cErr)
		}
	}
	docs, err := c.load()
	if err != nil {
		return err
	}
	return c.report(&Result{
		Format: formatOKF, Documents: len(docs), Files: len(paths), Out: c.Out,
	})
}

// exportRows writes one JSON object per concept to stdout.
func (c *Config) exportRows() error {
	if c.Out != "" {
		// Refused rather than ignored. A caller who passed --out expects a file, and
		// silently writing to stdout instead would look like the export vanished.
		return c.usage(errors.New("--format " + formatJSONL +
			" writes to stdout; redirect it rather than passing --out"))
	}
	docs, err := c.load()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(c.Stdout)
	for i := range docs {
		if eErr := enc.Encode(bundle.ExportRow(&docs[i])); eErr != nil {
			return c.fail(eErr)
		}
	}
	return c.report(&Result{Format: formatJSONL, Documents: len(docs)})
}

// load reads the corpus.
func (c *Config) load() ([]bundle.Document, error) {
	docs, err := bundle.Load(os.DirFS(c.Bundle))
	if err != nil {
		return nil, c.fail(err)
	}
	return docs, nil
}

// report renders the outcome.
//
// **The jsonl form's own count goes to stderr**, because stdout is the export: a
// summary line mixed into it would be a row the receiver's decoder rejects.
func (c *Config) report(result *Result) error {
	if c.JSONL && result.Format != formatJSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("export: %w", err)
		}
		return nil
	}
	if result.Out != "" {
		// The destination on stdout and the count on stderr, which is `proof create`'s
		// split and the one a caller can use: `gnosis export ... | xargs tar` wants the
		// path and nothing else on the pipe.
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", filepath.ToSlash(result.Out))
		_, _ = fmt.Fprintf(c.Stderr, "%d file(s) covering %d concept(s) written\n",
			result.Files, result.Documents)
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr, "%d concept(s) exported\n", result.Documents)
	return nil
}

// copyInto copies one bundle-relative file to the same place under dst.
//
// Requires: rel came from PortablePaths, so it is bundle-relative and names a file.
// Ensures: the destination's parent directories exist and the bytes are the source's.
//
// The mode is not carried across. A corpus is markdown and JSON that a receiver reads;
// reproducing an executable bit from a bundle somebody else assembled would export a
// permission decision along with the text, which is not what was asked for.
func copyInto(srcDir, dstDir, rel string) error {
	const op = "export.copyInto"

	src, err := os.Open(filepath.Join(srcDir, filepath.FromSlash(rel)))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer func() { _ = src.Close() }()

	full := filepath.Join(dstDir, filepath.FromSlash(rel))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return fmt.Errorf("%s: %w", op, mkErr)
	}
	dst, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if _, cErr := io.Copy(dst, src); cErr != nil {
		_ = dst.Close()
		return fmt.Errorf("%s: %w", op, cErr)
	}
	// Closed here rather than deferred: a deferred close on a file being written
	// discards the error, and a short write reported as success is an export missing
	// the end of a document.
	if cErr := dst.Close(); cErr != nil {
		return fmt.Errorf("%s: %w", op, cErr)
	}
	return nil
}

// fail and usage adapt root's reporting to this command's name. The reason is fixed for
// `proofcmd`'s reason: every way this command fails is the bundle being unusable.
func (c *Config) fail(cause error) error {
	return fmt.Errorf("export: %w", c.Fail(root.ReasonNoBundle, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("export: %w", c.Usage(cause))
}
