// Package initcmd implements the "init" CLI command.
package initcmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/ontology"
)

// indexDoc is the OKF §8 entry point. It carries no frontmatter because it is a
// reserved navigational file rather than a concept (OKF §3.1).
const indexDoc = `# Index

The entry point to this knowledge base. It is written for a reader — or an agent
— arriving with a question and no idea where to look.

Keep it a map, not a mirror. A generated list of every document is available from
` + "`gnosis search`" + ` and ` + "`gnosis graph`" + `; what belongs here is the
handful of paths through the corpus that a newcomer actually needs.
`

// logDoc seeds OKF §9's update log with no entries. Entry headings must be
// "## YYYY-MM-DD", so seeding one would mean stamping today's date into a file
// nothing has happened to yet.
const logDoc = `# Update Log

What changed in this knowledge base, and why. Each entry is a date heading in
the OKF §9 form:

    ## 2026-01-31
    * what changed, and the reasoning that is not obvious from the diff
`

// gitignore keeps tier 3 out of git. Everything under .gnosis/ is derived from
// the bundle and the evidence archive, so committing it would put a cache under
// review and invite a merge conflict in a file nobody edits (SPEC §4.5).
const gitignore = `# Derived, regenerable state. Rebuild with: gnosis index rebuild
.gnosis/
`

// Config holds the configuration for the init command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result reports what the scaffold did. Existing paths are named rather than
// counted: running init on a bundle that already has one is the ordinary case,
// and a caller needs to know nothing was overwritten.
type Result struct {
	Bundle   string   `json:"bundle"`
	Created  []string `json:"created"`
	Existing []string `json:"existing,omitempty"`
}

// file is one scaffolded path and its contents.
type file struct {
	path    string
	content []byte
}

// New registers the init command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("init").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "init",
		Usage:     "gnosis init [--bundle DIR]",
		ShortHelp: "scaffold a knowledge base",
		LongHelp: `Scaffold an OKF bundle with a seed vocabulary and an empty index.

Nothing is ever overwritten. A path that already exists is reported as existing
and left alone, so running init against a live corpus is safe and says so — this
is the command most likely to be run by mistake in the wrong directory, and the
one where guessing would cost the most.

The seed vocabulary declares five types and no subjects. Types are needed
immediately because OKF requires one on every document; a subject is a vocabulary
negotiation, and there is nothing to negotiate until the corpus holds claims that
disagree.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: make directories, write absent files, open the
// index so the schema exists.
//
// The writer lock is taken first and covers the whole run, including the markdown
// scaffold. SPEC §4.6 states the requirement in full for exactly this reason: the
// writer owns the bundle rather than the database, so serialising only the
// index-open would coordinate the cache and leave two concurrent `init` runs
// racing over ontology.toml.
func (c *Config) exec(ctx context.Context, _ []string) error {
	result := Result{Bundle: c.Bundle, Created: []string{}, Existing: []string{}}

	// The bundle root has to exist before a lock can be placed inside it, and
	// `init` is the one command whose job is to create it.
	if err := os.MkdirAll(c.Bundle, 0o750); err != nil {
		return c.fail(err)
	}
	lock, err := bundle.AcquireWriterLock(ctx, c.Bundle)
	if err != nil {
		return c.fail(err)
	}
	defer lock.Release()

	for _, dir := range []string{".", "c"} {
		if err := os.MkdirAll(filepath.Join(c.Bundle, dir), 0o750); err != nil {
			return c.fail(err)
		}
	}

	for _, f := range scaffold() {
		created, err := writeIfAbsent(filepath.Join(c.Bundle, f.path), f.content)
		if err != nil {
			return c.fail(err)
		}
		if created {
			result.Created = append(result.Created, f.path)
		} else {
			result.Existing = append(result.Existing, f.path)
		}
	}

	// Opening the index creates .gnosis/ and applies the schema, so a freshly
	// initialised bundle is immediately usable rather than usable after the
	// first command that happens to need a database.
	db, err := bundle.OpenIndex(ctx, c.Bundle)
	if err != nil {
		return c.fail(err)
	}
	if err := db.Close(); err != nil {
		return c.fail(err)
	}

	sort.Strings(result.Created)
	sort.Strings(result.Existing)
	return c.report(result)
}

// report renders the outcome. Creating nothing is not a failure: it means the
// bundle was already initialised.
func (c *Config) report(result Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("init: %w", err)
		}
		return nil
	}
	for _, p := range result.Created {
		_, _ = fmt.Fprintf(c.Stdout, "created %s\n", p)
	}
	for _, p := range result.Existing {
		_, _ = fmt.Fprintf(c.Stderr, "kept existing %s\n", p)
	}
	_, _ = fmt.Fprintf(c.Stderr, "initialised %s: %d created, %d already present\n",
		result.Bundle, len(result.Created), len(result.Existing))
	return nil
}

// fail adapts root's reporting to this command's name. Every way init can fail
// is the same way — the bundle location is not usable — so the reason is fixed
// rather than passed in at each call site.
func (c *Config) fail(cause error) error {
	return fmt.Errorf("init: %w", c.Fail(root.ReasonNoBundle, cause))
}

// scaffold is the file set a new bundle starts with, in a fixed order so two
// runs report identically.
func scaffold() []file {
	return []file{
		{path: ontology.FileName, content: ontology.Starter()},
		{path: "index.md", content: []byte(indexDoc)},
		{path: "log.md", content: []byte(logDoc)},
		{path: ".gitignore", content: []byte(gitignore)},
	}
}

// writeIfAbsent writes content only when path does not exist.
//
// Requires: path's parent directory exists.
// Ensures: reports false and changes nothing when the file is already there.
// O_EXCL rather than a stat-then-write: the check and the write are one
// operation, so a concurrent init cannot land between them.
func writeIfAbsent(path string, content []byte) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := f.Write(content); err != nil {
		return false, errors.Join(fmt.Errorf("write %s: %w", path, err), f.Close())
	}
	if err := f.Close(); err != nil {
		return false, fmt.Errorf("close %s: %w", path, err)
	}
	return true, nil
}
