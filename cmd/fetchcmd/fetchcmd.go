// Package fetchcmd implements the "fetch" CLI command.
package fetchcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/archive"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// Config holds the configuration for the fetch command.
type Config struct {
	*root.Config

	// DryRun decides nothing differently. It is a field on the command rather
	// than a separate `preview` verb because SPEC §4.6.2 makes a write a command
	// value: preview and apply are one command differing in one field, which is
	// what makes §9.4's guarantee — that the writer applies exactly what was
	// previewed — constructible rather than merely intended.
	DryRun bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the fetch command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("fetch").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.DryRun, 'n', "dry-run", "decide dispositions without writing")
	cfg.Command = &ff.Command{
		Name:      "fetch",
		Usage:     "gnosis fetch <URI>...",
		ShortHelp: "archive a source as evidence and record what became of it",
		LongHelp: `Read one or more sources and place them in tier 0, the evidence store.

Four adapters and no more: a local file, a local directory, an http(s) resource,
and a git repository. Every additional protocol is a new failure mode in the path
that produces evidence.

Each source gets exactly one of three dispositions, chosen by the gates in
standards/archive.toml and never by a flag. **archived** keeps the source itself,
so a quotation validates offline forever. **extracted** keeps text recovered from
a source that could not be kept — HTML through the one pinned extractor, whose
name and version are recorded so a later re-extraction is visible rather than
silent. **referenced** keeps only the URI and the hash, and is a supported outcome
rather than a failure: it is reasonable to weakly trust a published standard for a
claim nothing else leans on, and what makes that safe is that the weakness stays
visible per claim.

A re-fetch of unchanged bytes writes nothing. Records are addressed by the hash of
their own content and carry no timestamp, so tier 0 grows when the corpus learns
something and not when somebody checks.

--dry-run reports the same dispositions and writes nothing.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: load the gates, read each source, decide, store.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return c.usage(errors.New(
			"fetch needs at least one source; try `gnosis fetch ./notes` or a URL"))
	}

	loaded, err := bundle.LoadArchiveStandards(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonStandardsInvalid, err)
	}
	gates := bundle.ArchiveGates(loaded)

	var (
		fetcher bundle.Fetcher
		result  Result
		looked  []bundle.Check
	)
	for _, uri := range args {
		candidates, ferr := fetcher.Fetch(ctx, uri)
		if ferr != nil {
			return c.fail(root.ReasonFetchFailed, ferr)
		}
		for i := range candidates {
			source, serr := c.admit(&candidates[i], gates)
			if serr != nil {
				return c.fail(root.ReasonFetchFailed, serr)
			}
			looked = append(looked, bundle.Check{
				URI: source.URI, SourceSHA256: source.SourceSHA256,
			})
			result.add(&source)
		}
	}

	// Every source that was read was looked at, whatever became of it. §9.2 says
	// a no-op re-fetch "advances checked.jsonl", and the observation is just as
	// true for one that archived or one that fell through to `referenced`: this
	// user saw these bytes at this moment. Recording only the no-ops would leave
	// a freshly fetched source reading as never-checked.
	if !c.DryRun {
		if err = bundle.RecordChecks(c.Bundle, time.Now().UTC(), looked); err != nil {
			return c.fail(root.ReasonFetchFailed, err)
		}
	}
	return c.report(&result)
}

// admit decides one candidate and stores what the decision produced.
//
// Extraction is attempted before the decision, not after: `extracted` is only
// available to a candidate that already carries one, and asking the archive to
// call an extractor would put an I/O dependency inside the pure policy.
func (c *Config) admit(cand *archive.Candidate, gates archive.Gates) (Source, error) {
	// An extraction that fails is not a fetch that fails. The source still gets
	// a record — as `referenced`, carrying whatever reason its own gates produced.
	_ = bundle.Extract(cand)

	out := archive.Decide(cand, gates)
	source := Source{
		URI:          out.Record.URI,
		SourceSHA256: out.Record.SourceSHA256,
		Disposition:  out.Record.Disposition,
		ArchivePath:  out.Record.ArchivePath,
		RejectReason: out.Record.RejectReason,
	}
	if c.DryRun {
		recordPath, err := out.Record.Path()
		if err != nil {
			return Source{}, fmt.Errorf("fetch: %w", err)
		}
		source.RecordPath = recordPath
		return source, nil
	}

	stored, err := bundle.StoreEvidence(c.Bundle, &out)
	if err != nil {
		return Source{}, fmt.Errorf("fetch: %w", err)
	}
	source.RecordPath = stored.RecordPath
	source.Wrote = stored.Wrote
	return source, nil
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("fetch: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("fetch: %w", c.Usage(cause))
}
