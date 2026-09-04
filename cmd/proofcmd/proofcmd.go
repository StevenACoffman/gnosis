// Package proofcmd implements the "proof" CLI command.
package proofcmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/skillet/proof"
)

// longHelp is the command's prose, extracted from New for `misscmd`'s reason: a page of
// help text inside a constructor puts the function over the length limit and none of it
// is logic.
const longHelp = `Bind this corpus's bytes into a proof packet (SPEC §8.5, §16.2).

A packet names every file in the shareable corpus and the digest of its contents, so
that ` + "`adh`" + ` can close an arc that touched the knowledge base under
no-proof-no-close: the arc declares what it produced, and a later verifier can say
whether those exact bytes are still there.

What it covers is the corpus and tier 0 — concepts, archived evidence, the ontology,
the standards files, the log. What it never covers is .gnosis/, which holds the audit
trail, the prompt cache, the miss log and the coverage ledger. Those are per-user and
derived: two colleagues at one commit have different ones and are both right, so a
digest over them would fail for a reader who had done nothing wrong.

The commit is recorded when there is one, and a bundle that is not under version
control is still provable. Provenance says where the bytes came from; the digests say
what they are, and only the second is what a verifier checks.

Reads only. It takes no lock and writes nothing inside the bundle.`

// Config holds the configuration for the proof command.
type Config struct {
	*root.Config

	// Out is where the packet is written. Required: a packet printed to stdout and
	// nowhere else is one nobody can verify against later.
	Out string

	// Arc names what the packet is proof *for*, and is the caller's word rather than
	// gnosis's — the arc lives in the tool closing it.
	Arc string

	// Flags and Command are this command's own. Declaring them is not boilerplate:
	// root.Config has fields of both names, an embedded field is reachable by the same
	// selector, and without these `cfg.Flags = ...` would assign the root's flag set —
	// which is how `synthesize` once made every other command require --model.
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result reports what was bound.
type Result struct {
	Out       string `json:"out"`
	Arc       string `json:"arc"`
	Artifacts int    `json:"artifacts"`

	// GitSHA is empty when the bundle is not a worktree, which is a supported state
	// rather than a degraded one. Not omitempty, so a reader can tell a packet that
	// recorded no provenance from one whose provenance a renderer dropped.
	GitSHA string `json:"git_sha"`
}

// New registers the proof command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("proof").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"path to write the proof packet to (required)")
	cfg.Flags.StringVar(&cfg.Arc, 0, "arc", "",
		"the arc this packet is proof for (required)")
	create := &ff.Command{
		Name:      "create",
		Usage:     "gnosis proof create --arc NAME --out PATH",
		ShortHelp: "bind the corpus and tier 0 into a proof packet",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	cfg.Command = &ff.Command{
		Name:        "proof",
		Usage:       "gnosis proof <SUBCOMMAND>",
		ShortHelp:   "bind corpus and tier-0 digests into a proof packet",
		Subcommands: []*ff.Command{create},
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: list, resolve, hash, write.
func (c *Config) exec(_ context.Context, args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New(
			"proof create takes no positional arguments; --arc and --out carry both values"))
	}
	if c.Arc == "" || c.Out == "" {
		return c.usage(errors.New("proof create requires --arc and --out"))
	}

	paths, err := bundle.PortablePaths(c.Bundle)
	if err != nil {
		return c.fail(err)
	}
	if len(paths) == 0 {
		// Named here rather than left to proof.Create's EINVALID, which says the
		// packet declares no artifacts and not *why* this bundle produced none. An
		// empty corpus is the ordinary reason and the remedy is not a flag.
		return c.fail(errors.New(
			"this bundle holds no shareable file, so there is nothing to prove;" +
				" .gnosis/ is per-user and never covered"))
	}
	sha, err := bundle.HeadSHA(c.Bundle)
	if err != nil {
		return c.fail(err)
	}

	packet, err := proof.Create(c.Bundle, c.Arc, sha, paths)
	if err != nil {
		return c.fail(err)
	}
	if err := proof.Save(c.Out, &packet); err != nil {
		return c.fail(err)
	}
	return c.report(&Result{
		Out: c.Out, Arc: c.Arc, Artifacts: len(packet.Artifacts), GitSHA: sha,
	})
}

// report renders the outcome.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("proof: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(c.Stdout, "%s\n", filepath.ToSlash(result.Out))
	_, _ = fmt.Fprintf(c.Stderr, "%d artifact(s) bound to arc %q\n",
		result.Artifacts, result.Arc)
	if result.GitSHA == "" {
		// Said out loud, because the packet still verifies and a reader who expected
		// provenance would otherwise find its absence only by opening the JSON.
		_, _ = fmt.Fprintf(c.Stderr,
			"no commit recorded: this bundle is not a git worktree, or has no commits."+
				" The packet still verifies on its bytes\n")
	}
	return nil
}

// fail and usage adapt root's reporting to this command's name.
//
// **The reason is fixed rather than passed in**, which `init` does for the same finding:
// every call site was handing over the same token, and a parameter with one argument is a
// parameter a reader has to check.
//
// The limit is worth stating where it is felt. `--out` naming a path gnosis cannot write
// reports `no_bundle` too, which is coarse: the bundle was fine and the output location
// was not. There is no reason token for that, and adding one for a single consumer is
// what `bundle.corrupt` records deciding against — the message names the path either way,
// so the caller is told the truth and only a machine branching on the token is misled.
// The second consumer is what would justify the code.
func (c *Config) fail(cause error) error {
	return fmt.Errorf("proof: %w", c.Fail(root.ReasonNoBundle, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("proof: %w", c.Usage(cause))
}
