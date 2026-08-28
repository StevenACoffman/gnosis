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
	"github.com/StevenACoffman/gnosis/internal/audit"
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

	// Recheck re-fetches the sources tier 0 already records and compares each
	// against the passages this corpus quoted from it (§14.3.2).
	//
	// A flag on `fetch` rather than a `recheck` verb, for the reason DryRun is a
	// field: it is the same operation — read a source, decide, store — differing
	// only in where the list of sources comes from. A separate command would be a
	// second path to tier 0, and §4.6 keeps there being one.
	Recheck bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the fetch command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("fetch").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.DryRun, 'n', "dry-run", "decide dispositions without writing")
	cfg.Flags.BoolVar(&cfg.Recheck, 0, "recheck",
		"re-fetch recorded sources and compare them against the passages quoted from them")
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

--dry-run reports the same dispositions and writes nothing.

--recheck takes the list of sources from tier 0 instead of the command line and
compares each against the passages this corpus quoted from it. Three answers, and
the reason they are three: **drift-benign** means the bytes moved and every
recorded passage is still there, which is a re-archive and nothing more;
**drift-unsupported** means a claim in the corpus has lost its support upstream,
which is a finding on the claim and a person's decision; **drift-unchecked** means
the comparison could not be made and neither of the other two may be asserted.
Reporting the first two alike would put the cheapest maintenance task and the most
serious evidentiary event in one bucket, sized for the cheap one.

A re-check never rewrites or retracts anything. A passage that fails against the
*archived* copy is corruption and a hard failure; this is only ever about the
archive disagreeing with upstream, where the archive is intact.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: load the gates, read each source, decide, store.
//
// **The writer lock is taken here, and it was missing.** `fetch` writes tier 0 and
// rewrites `.gnosis/checked.jsonl` whole — a read-modify-write over the entire
// file — and it did neither under the lock, so two concurrent fetches could lose
// one user's observations outright. Seven functions carried
// `Requires: the writer lock is held` in prose and this caller was the one that did
// not do it; nothing reported that, because a prose precondition has no failure
// mode. It is a `*bundle.Writer` now and the compiler is what asks.
//
// Under --dry-run the lock is still taken. Deciding a disposition reads tier 0, and
// a preview that raced a concurrent fetch would report dispositions against an
// archive that no longer exists — the same argument §9.4 makes for the promote
// gate's preview.
func (c *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 && !c.Recheck {
		return c.usage(errors.New(
			"fetch needs at least one source; try `gnosis fetch ./notes` or a URL"))
	}

	loaded, err := bundle.LoadArchiveStandards(c.Bundle)
	if err != nil {
		return c.fail(root.ReasonStandardsInvalid, err)
	}
	gates := bundle.ArchiveGates(loaded, c.Rules)

	w, err := bundle.AcquireWriter(ctx, c.Bundle)
	if err != nil {
		if bundle.WriterBusy(err) {
			return c.fail(root.ReasonWriterBusy, err)
		}
		return c.fail(root.ReasonNoBundle, err)
	}
	defer w.Release()

	rk, args, err := c.recheckPlan(args)
	if err != nil {
		return err
	}

	result, err := c.fetchAll(ctx, w, args, gates, rk)
	if err != nil {
		return err
	}
	if !c.DryRun {
		if err = w.RecordChecks(time.Now().UTC(), checksFor(result)); err != nil {
			return c.fail(root.ReasonFetchFailed, err)
		}
		if aErr := c.audit(w, result); aErr != nil {
			return aErr
		}
	}
	return c.report(result)
}

// audit records that this fetch happened.
//
// Requires: w still holds the lock; result came from fetchAll.
// Ensures: one row per invocation, or the escalation when the trail lied about
// writing it. Never called under --dry-run, which mutates nothing.
//
// **This was missing, and it was the last mutation §15 could not see.**
// `audit.OpFetch` was declared when the trail was built and had no writer, so the
// one operation that puts durable evidence into the corpus left no record of having
// run. `init` and `index rebuild` were fixed for exactly this reason — a claim that
// holds for some of its subjects is the half-truth §15 is about — and fetch was
// missed because it went through neither the coordinator nor the writer lock.
//
// One row per invocation rather than per source, following `init`. A fetch of three
// sources is one thing somebody did, and `Paths` names what it wrote.
//
// A row is written even when nothing reached tier 0, again following `init`: "we
// re-fetched and everything was already there" is a fact about this machine, and a
// trail holding only the writes would make a repeated fetch look like it never
// happened. `checked.jsonl` records that the sources were *looked at* and cannot say
// that a fetch ran.
//
// The actor is a check rather than a person because the tool caused the write, per
// §5.5's reasoning where `findings.opened_by` names one: a check name is as much an
// answer as an actor is. The trail is per-user anyway, so the file is the person.
func (c *Config) audit(w *bundle.Writer, result *Result) error {
	aErr := w.Audit(&audit.Row{
		At: time.Now().UTC(), Op: audit.OpFetch, Actor: "check:fetch",
		Paths:   result.auditPaths(),
		Outcome: string(root.StatusOK),
		Detail:  result.auditDetail(),
	})
	if aErr == nil {
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr, "warning: the fetch was not audited: %v\n", aErr)
	if bundle.AuditLost(aErr) {
		// The append reported success and the row is not on disk, which no other
		// signal reveals. Best-effort covers a *known* gap; it must not cover a
		// trail that lied about writing (§15).
		return root.ExitError(root.CodeError)
	}
	return nil
}

// fetchAll reads every requested source and stores what each decision produced.
//
// Extracted from exec because the linter reported the complexity and was right:
// exec had come to hold argument validation, standards loading, lock acquisition,
// a nested loop, and a second write. The split is by knowledge rather than by the
// order things happen — exec composes the run, this reads sources.
func (c *Config) fetchAll(
	ctx context.Context, w *bundle.Writer, args []string, gates archive.Gates,
	rk *rechecks,
) (*Result, error) {
	var (
		fetcher bundle.Fetcher
		result  Result
	)
	for _, uri := range args {
		candidates, err := fetcher.Fetch(ctx, uri)
		if err != nil {
			return nil, c.fail(root.ReasonFetchFailed, err)
		}
		for i := range candidates {
			source, err := c.admit(w, &candidates[i], gates)
			if err != nil {
				return nil, c.fail(root.ReasonFetchFailed, err)
			}
			result.add(&source)
			// After admit, not before: admit is what runs the extractor, and the
			// comparison needs the text a quotation was validated against rather
			// than the markup it was taken out of. A nil rk is every ordinary
			// fetch, and does nothing.
			rk.compare(&candidates[i], &result)
		}
	}
	return &result, nil
}

// checksFor is what this run looked at, projected from what it found.
//
// Requires: result came from fetchAll.
// Ensures: one observation per source read, plus one per recorded version a
// --recheck compared, in that order. Pure.
//
// It is derived rather than accumulated alongside, and that is the point: exec
// previously appended to `looked` and to `result` in the same loop, which is two
// mutable accumulators that could disagree about what happened. Every field this
// needs is already on a Source, so one of them was redundant.
//
// Every source that was read was looked at, whatever became of it. §9.2 says a
// no-op re-fetch "advances checked.jsonl", and the observation is just as true for
// one that archived or one that fell through to `referenced`: this user saw these
// bytes at this moment. Recording only the no-ops would leave a freshly fetched
// source reading as never-checked.
func checksFor(result *Result) []bundle.Check {
	out := make([]bundle.Check, 0, len(result.Sources)+len(result.Drift))
	for i := range result.Sources {
		s := &result.Sources[i]
		out = append(out, bundle.Check{
			URI: s.URI, SourceSHA256: s.SourceSHA256, Revision: s.Revision,
		})
	}
	return append(out, comparisons(result)...)
}

// comparisons are the observations a --recheck made about versions it did not fetch.
//
// Requires: result came from fetchAll.
// Ensures: one observation per drift row, keyed on the *recorded* version. Pure, and
// empty for an ordinary fetch.
//
// **These are separate rows because they are about different bytes.** The rows above
// say "I read this source and here is its version". These say "I compared the version
// I already had against what is upstream now, and here is what happened to the
// passages" — a different subject, and the one a claim's archive path resolves to.
//
// Without them the archived version's observation never advances: a re-check that
// found changed bytes recorded a check for the *new* version only, so a claim resting
// on the old archived copy still read as last verified whenever it was first fetched,
// however recently somebody had confirmed its quotations were still upstream.
//
// The verdict travels with the timestamp deliberately, and §14.3.2 is why it is safe:
// freshness answers "when was this checked" and drift answers "does upstream still say
// it". A claim can be freshly checked and have lost its support, and a reader is shown
// both — `show` prints the drift beside the freshness for exactly this pairing.
func comparisons(result *Result) []bundle.Check {
	out := make([]bundle.Check, 0, len(result.Drift))
	for i := range result.Drift {
		d := &result.Drift[i]
		if d.SourceSHA256 == "" {
			continue
		}
		out = append(out, bundle.Check{
			URI: d.URI, SourceSHA256: d.SourceSHA256, Drift: d.State.String(),
		})
	}
	return out
}

// admit decides one candidate and stores what the decision produced.
//
// Extraction is attempted before the decision, not after: `extracted` is only
// available to a candidate that already carries one, and asking the archive to
// call an extractor would put an I/O dependency inside the pure policy.
func (c *Config) admit(
	w *bundle.Writer, cand *archive.Candidate, gates archive.Gates,
) (Source, error) {
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
		Findings:     c.explain(cand, out.Record.RejectReason, gates),
		// Carried from the candidate, not from the record: the record does not
		// have it and must not (§4.3.1).
		Revision: cand.Revision,
	}
	if c.DryRun {
		recordPath, err := out.Record.Path()
		if err != nil {
			return Source{}, fmt.Errorf("fetch: %w", err)
		}
		source.RecordPath = recordPath
		return source, nil
	}

	stored, err := w.StoreEvidence(&out)
	if err != nil {
		return Source{}, fmt.Errorf("fetch: %w", err)
	}
	source.RecordPath = stored.RecordPath
	source.Wrote = stored.Wrote
	return source, nil
}

// explain is what a source the gates refused actually contains, or nothing.
//
// Requires: cand is what was fetched; why is the reason the record carries; gates
// carries the declared caps.
// Ensures: findings only for a refusal a reader can act on, and nothing otherwise.
// Pure apart from reading the ruleset off the config.
//
// The reason on the record is one reason because a disposition is one reason. This is
// the detail that answers what a reader does next, and it is reported for exactly the
// refusals where there *is* a next step:
//
//   - a scan finding says which class or which rule, and where;
//   - a size bound says how many bytes against what cap, which is the difference
//     between truncating an example and arguing the threshold down.
//
// A source refused for its extension gets nothing, and that is right: the extension is
// already the whole finding, and re-scanning would say nothing about it.
//
// A refused *extraction* is explained from the extraction's own text rather than the
// source's, because that is the text the reason is about — the record says so too,
// carrying the extraction's reason rather than the source's for the same reason.
func (c *Config) explain(
	cand *archive.Candidate, why archive.RejectReason, gates archive.Gates,
) []string {
	text := candidateText(cand)

	switch why {
	case archive.ReasonHiddenCharacters, archive.ReasonInjectionPattern,
		archive.ReasonSecret:
		return bundle.ScanFindings(c.Rules, string(text))
	case archive.ReasonOversize, archive.ReasonEmbeddedPayload:
		// Re-measured rather than carried out of Decide, for the same reason the
		// scan findings are re-derived: it keeps `archive.Decide` about dispositions,
		// and the two cannot disagree because both go through one measurement.
		if b := archive.Oversize(
			text, gates.PerFileCap, gates.EmbeddedPayloadCap,
		); b.Exceeded() {
			return []string{b.Detail()}
		}
		return nil
	default:
		return nil
	}
}

// fail and usage adapt root's reporting to this command's name.
func (c *Config) fail(reason string, cause error) error {
	return fmt.Errorf("fetch: %w", c.Fail(reason, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("fetch: %w", c.Usage(cause))
}
