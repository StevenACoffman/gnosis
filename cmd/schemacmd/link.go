package schemacmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/schema"
)

// link makes AGENTS.md canonical and points each agent's filename at it.
//
// Requires: nothing; an absent AGENTS.md is reported rather than created, because
// linking to a file that does not exist would leave every agent reading a broken link.
// Ensures: one symlink per agent filename, and a refusal that names the file when one
// of them is a regular file somebody wrote.
//
// # Why a regular file is a refusal and a symlink is not
//
// §5.7 borrows this from `mdm rules link`: one canonical file, symlinked, so several
// agents "read one file and cannot drift". Repointing an existing symlink is what the
// command is *for* — a symlink is a pointer and pointing it somewhere else loses
// nothing.
//
// A regular file is different in kind. Somebody wrote it, and replacing it with a
// symlink deletes what they wrote — a data loss no flag should be able to cause
// quietly, and the same argument as the marker contract's fail-closed rule one level
// up: a file predating this command was not written under its contract.
func (c *Config) link() error {
	canonical := filepath.Join(c.Bundle, bundle.SchemaFile)
	if _, err := os.Stat(canonical); err != nil {
		return c.fail(root.ReasonNoBundle, fmt.Errorf(
			"no %s to link to; run `gnosis schema` first", bundle.SchemaFile))
	}

	var linked []string
	for _, name := range agentFiles() {
		full := filepath.Join(c.Bundle, name)
		switch replaceable, err := linkable(full); {
		case err != nil:
			return c.fail(root.ReasonNoBundle, err)
		case !replaceable:
			return c.fail(root.ReasonNeedsHuman, fmt.Errorf(
				"%s is a regular file somebody wrote; move it aside and re-run, "+
					"because replacing it with a symlink would delete it", name))
		}
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return c.fail(root.ReasonNoBundle, err)
		}
		// A relative target, so the bundle stays movable: an absolute one would break
		// the moment somebody cloned the corpus to another path.
		if err := os.Symlink(bundle.SchemaFile, full); err != nil {
			return c.fail(root.ReasonNoBundle, err)
		}
		linked = append(linked, name)
	}
	return c.report(&Result{Linked: linked})
}

// linkable reports whether the path may be replaced with a symlink.
//
// Requires: nothing; an absent path is replaceable.
// Ensures: true for an absent path and for an existing symlink, false for anything
// else. `Lstat` rather than `Stat`, because `Stat` follows the link and would report an
// existing symlink as whatever it points at — including, once the command has run
// once, as a regular file. That would make the second run refuse to do what the first
// one did.
func linkable(full string) (bool, error) {
	info, err := os.Lstat(full)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("inspect %s: %w", full, err)
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}

// report renders the outcome.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		return c.emitEnvelope(result)
	}

	for _, name := range result.Linked {
		_, _ = fmt.Fprintf(c.Stdout, "%s -> %s\n", name, bundle.SchemaFile)
	}
	if len(result.Linked) > 0 {
		return nil
	}

	acted := false
	for i := range result.Docs {
		if c.describe(&result.Docs[i]) {
			acted = true
		}
	}
	if acted {
		return root.ExitError(root.CodeFindings)
	}
	return nil
}

// describe writes one document's line and reports whether it needs a person.
//
// **The unmarked case names both files and says what to do**, because it is the common
// path rather than the edge: `init` has seeded `index.md` with curated prose since the
// beginning, so every bundle that exists has a hand-written one, and a reader told only
// "not touched" would think the command had failed.
func (c *Config) describe(doc *DocResult) bool {
	switch {
	case doc.Outcome == schema.OutcomeUnmarked:
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", doc.Path)
		_, _ = fmt.Fprintf(c.Stderr,
			"%s carries no gnosis markers, so it was not touched. The generated "+
				"regions are in %s; paste the ones you want and re-run.\n",
			doc.Source, doc.Path)
		return true
	case c.Check && doc.Stale:
		_, _ = fmt.Fprintf(c.Stderr,
			"%s is stale; run `gnosis schema` to update the generated regions\n",
			doc.Source)
		return true
	case c.Check:
		_, _ = fmt.Fprintf(c.Stdout, "%s is up to date\n", doc.Source)
	case doc.Wrote:
		_, _ = fmt.Fprintf(c.Stdout, "%s\n", doc.Path)
	default:
		_, _ = fmt.Fprintf(c.Stdout, "%s was already up to date\n", doc.Source)
	}
	return false
}

// emitEnvelope writes the machine envelope.
//
// The two findings cases are the same two the human path reports, for the same reasons:
// a stale file under --check is something a caller acts on, and a run that wrote the
// sibling file did not do what was asked.
func (c *Config) emitEnvelope(result *Result) error {
	var owed []string
	for i := range result.Docs {
		doc := &result.Docs[i]
		if !doc.needsHuman(c.Check) {
			continue
		}
		if doc.Outcome == schema.OutcomeUnmarked {
			owed = append(owed, doc.Source+" carries no gnosis markers; wrote "+doc.Path)
			continue
		}
		owed = append(owed, doc.Source+" is stale")
	}
	if len(owed) == 0 {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
		return nil
	}
	if err := c.EmitFindings(root.ReasonNeedsHuman, strings.Join(owed, "; "), result); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return root.ExitError(root.CodeFindings)
}
