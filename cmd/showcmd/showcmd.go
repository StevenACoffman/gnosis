// Package showcmd implements the "show" CLI command.
package showcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/skillet/errs"
)

// Config holds the configuration for the show command.
type Config struct {
	*root.Config
	Body    bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload: the document, its links, and optionally its text.
type Result struct {
	*index.Detail
	Body string `json:"body,omitempty"`
}

// New registers the show command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("show").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Body, 'b', "body", "include the document text")
	cfg.Command = &ff.Command{
		Name:      "show",
		Usage:     "gnosis show <PATH|ID>",
		ShortHelp: "render a document with its links resolved",
		LongHelp: `Render one document: its title, its outbound links with each target
resolved, and the documents linking back to it.

A reference may be a bare identifier, a current path, or a **stale** path whose
slug no longer matches the title. All three resolve, because the identifier is
parsed out of the filename and matched on rather than the path being looked up.
That is what lets a link written before a retitle keep working with no mapping
table and no redirect.

Links are rendered inline, including the ones that lead nowhere. An unresolved
link is not an error — it is knowledge somebody intended and nobody has written
yet — and hiding it would remove the only record that the author meant to point
somewhere.

The text is read from the file rather than the index, because the file is the
source of truth and the index is a cache of it.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: resolve the reference, gather, render.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return c.usage(errors.New(
			"show needs exactly one path or identifier; try `gnosis show c/<id>-<slug>.md`"))
	}

	db, err := bundle.OpenIndexForRead(ctx, c.Bundle)
	if err != nil {
		return c.fail(root.ReasonNoBundle, err)
	}
	defer func() { _ = db.Close() }()

	detail, err := db.Find(ctx, args[0])
	if err != nil {
		switch errs.ErrorCode(err) {
		case errs.EINVALID:
			return c.usage(err)
		case errs.ENOTFOUND:
			// Not a usage error and not a tool failure: the reference was
			// well-formed and the corpus does not hold it. A caller that asked
			// for something absent needs a distinct code from one that asked
			// wrongly.
			return c.fail(root.ReasonNeedsHuman, err)
		}
		return c.fail(root.ReasonIndexDrift, err)
	}

	result := Result{Detail: detail}
	if c.Body {
		if result.Body, err = c.readBody(detail.Path); err != nil {
			return c.fail(root.ReasonNoBundle, err)
		}
	}
	return c.report(&result)
}

// readBody reads the document from disk. The index holds a searchable copy; this
// reads the original, so what is shown is what is committed.
func (c *Config) readBody(rel string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(c.Bundle, filepath.FromSlash(rel)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf(
				"%s is in the index but not on disk; run `gnosis index rebuild`", rel)
		}
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	return string(raw), nil
}

// report renders the outcome.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("show: %w", err)
		}
		return nil
	}

	_, _ = fmt.Fprintf(c.Stdout, "%s\n%s\n%s\n",
		result.Title, result.Path, result.ID)
	writeLinks(c.Stdout, "links to", result.Outbound)
	writeLinks(c.Stdout, "linked from", result.Inbound)
	if result.Body != "" {
		_, _ = fmt.Fprintf(c.Stdout, "\n%s", result.Body)
	}
	return nil
}

// writeLinks renders one direction of the link graph, or says it is empty.
// Silence would be ambiguous between "no links" and "not looked up".
func writeLinks(w io.Writer, label string, links []index.Resolved) {
	if len(links) == 0 {
		_, _ = fmt.Fprintf(w, "\n%s: none\n", label)
		return
	}
	_, _ = fmt.Fprintf(w, "\n%s:\n", label)
	for _, l := range links {
		_, _ = fmt.Fprintf(w, "  %s\n", l)
	}
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("show: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("show: %w", c.Usage(cause))
}
