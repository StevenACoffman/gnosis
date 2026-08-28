// Package auditcmd implements the "audit" CLI command.
package auditcmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/bundle"
)

// defaultWindowDays is --gained's period when no --since is given.
//
// A default rather than a required flag: the friction belongs on the rate this report
// refuses to compute, not on asking the question at all.
const defaultWindowDays = 30

// longHelp is the command's prose.
//
// A constant rather than an inline literal because the command has three reports now
// and the help that explains them pushed `New` past the length the linter allows —
// which was the right complaint about the wrong thing: `New` registers a command, and
// the paragraphs are not part of registering it.
const longHelp = `Report on the write trail.

--outstanding lists every decision gnosis asked a person for and nobody made: a
promotion that reached needs_human, whose draft is still in quarantine, and which
was never promoted and never discarded.

**Abandonment is not refusal, and that is why this report exists.** A promotion the
gate refused is recomputable — run the gate again and get the same answer. A
promotion somebody declined is a decision, and belongs in the record with its
reason. A promotion somebody was asked about and walked away from is neither: no
decision was made and nothing records that one is owed. Without this report those
drafts are indistinguishable from drafts nobody has looked at yet, which is the
state a corpus accumulates quietly.

A draft nobody has ever run ` + "`gnosis promote`" + ` against is **not** listed. It is
unexamined rather than abandoned, ` + "`gnosis quarantine`" + ` already lists it, and
reporting a fresh corpus as a pile of neglected decisions on its first day would
teach a reader to skip the category.

--unsupported lists what each source was found *not* to support: a claim a reply
asserted, checked against the archived text the prompt was built from, and not
located there.

**A corpus recorded what a source supports and kept no trace of the opposite.** A
refused reply was reported once and forgotten, so the same assertion could be
offered again by the same model with nothing saying it had been tried. This is the
same asymmetry the rejected-alias list already fixed one level down, where keeping
the conclusion and throwing away the reasoning was the thing to avoid.

Only *unsupported* claims appear, never *unchecked* ones. "Sought in the archive and
not there" is a statement about the source; "nobody looked" is not, and recording the
second here would assert that a source contradicts a passage too short to check.

--churn lists the sources tier 0 holds more than one version of, and what each move
cost: bytes that moved and kept the passages this corpus quotes, bytes that moved and
lost them, and versions nobody has compared. FPF's Effort field asks what a claim
costs to keep current and this is the computable half — how often its sources move.

It needed no new field. A source fetched twice has two records, so the record count
per source *is* how many times the bytes changed; nothing had asked the question.

**A count, never a cost.** "This source moved six times" is not an effort estimate,
it is the observation an estimate would rest on. Six moves that all kept their
passages is a source that churns without changing what it says, and §14.3.2 already
says its claims want a shorter stale_after; one move that lost a passage is a
different event and no number of the first adds up to it.

--gained counts what the corpus acquired in a window: documents promoted, replies
admitted, sources archived, and drafts a reader looked at and declined.

**It exists because every other report here counts something wrong.** Hamming's
rating dynamic: "if everyone starts out at 95% there is little a person can do to
raise their rating but much which will lower it; hence the obvious strategy is to play
things safe." A corpus whose only visible signal is problems-found rewards
contributing less and claiming less, and this is the same trail asked the opposite
question.

A declined draft counts as a gain, which is the entry somebody will argue about. A
draft declined is a judgement the corpus now holds and did not before; counting only
additions would make deciding-against invisible, and somebody who read forty drafts
and dropped thirty-nine did more for the corpus than somebody who admitted all forty.

There is no rate and no total-since-the-beginning. A number that only grows says
nothing, and a rate invites a target — which invites the padding this is meant to stop
rewarding. --since bounds it; the default window is thirty days.

The list is a count of things to look at, not a score. A damaged trail makes it a
floor rather than a total, and it says so.

--subjects reports how the vocabulary is occupied: per declared subject key, how many
claims rest on it, across how many documents, and which surface phrases authors
actually wrote for it.

**It is an instrument, not a report of problems, and it exists because the detector it
serves cannot be built yet.** The alias-collision rule fires only when somebody
*declares* a colliding alias. It cannot fire when two groups have been using one word
differently and neither declared it, which is the ordinary way the problem arises —
and detecting that needs a threshold, which nothing can calibrate without a population
to measure. This produces the population.

One column is a signal rather than a number. "no shared source" marks a key whose
claims come from two documents that read nothing in common, which is the shape the
silent-drift failure actually takes: two internally consistent halves and nothing
comparing them. It is not a defect — two teams often read different documentation
about the same thing — and it is shown because whoever is scanning this table is the
person who can tell which case it is.

There is no coverage figure and no score. A population looks like coverage and can be
raised by declaring subjects nobody uses, so this exits ok whatever it finds.

Reads only. It takes no lock and never writes.`

// Config holds the configuration for the audit command.
type Config struct {
	*root.Config

	// Outstanding reports the decisions the tool asked for and nobody made.
	//
	// A flag rather than the command's only behaviour, because §15 names two more
	// reports over the same trail — `--reversed` when §10.6.5's warrants exist —
	// and a bare `gnosis audit` that meant one of them would have to change
	// meaning when the second arrived.
	Outstanding bool

	// Unsupported reports what each source was found not to support.
	Unsupported bool

	// Churn reports how often each source has moved, and how those moves turned out.
	Churn bool

	// Gained reports what the corpus acquired, rather than what is wrong with it.
	Gained bool

	// SubjectPop reports how the vocabulary is occupied.
	//
	// Named for the field rather than the flag because `Subjects` would read as a
	// list of them, and this is a boolean asking for a report.
	SubjectPop bool

	// Unretrieved reports which claims searches were not seen to return (§12.2).
	Unretrieved bool

	// Since bounds --gained and --unretrieved. A count with no period is
	// uninterpretable.
	Since string

	Flags   *ff.FlagSet
	Command *ff.Command
}

// Result is the payload.
type Result struct {
	*bundle.Undecided
}

// New registers the audit command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("audit").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Outstanding, 0, "outstanding",
		"list the decisions this tool asked for and nobody made")
	cfg.Flags.BoolVar(&cfg.Unsupported, 0, "unsupported",
		"list the claims each source was found not to support")
	cfg.Flags.BoolVar(&cfg.Churn, 0, "churn",
		"list the sources whose bytes keep moving, and what the moves cost")
	cfg.Flags.BoolVar(&cfg.Gained, 0, "gained",
		"count what the corpus acquired, rather than what is wrong with it")
	cfg.Flags.BoolVar(&cfg.SubjectPop, 0, "subjects",
		"report how the vocabulary is occupied: claims and sources per subject key")
	cfg.Flags.BoolVar(&cfg.Unretrieved, 0, "unretrieved",
		"list the claims this user's searches were not seen to return")
	cfg.Flags.StringVar(&cfg.Since, 0, "since", "",
		"the window for --gained and --unretrieved, as YYYY-MM-DD;"+
			" defaults to the last 30 days")
	cfg.Command = &ff.Command{
		Name: "audit",
		Usage: "gnosis audit (--outstanding | --unsupported | --churn | --gained" +
			" | --subjects | --unretrieved)",
		ShortHelp: "report on the write trail",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec is the imperative shell: read, compute, render.
//
// Every input is read here and every fold is pure, which is what lets each report's
// definition be tested from literals rather than from a bundle arranged to hold each
// case.
//
// **Two of the five never open the trail**, and that ordering is the point rather
// than an optimisation: a bundle whose trail is damaged can still say which of its
// sources keep moving and how its vocabulary is occupied.
func (c *Config) exec(ctx context.Context, args []string) error {
	if err := c.validate(args); err != nil {
		return err
	}
	switch {
	case c.Unretrieved:
		return c.reportReach(ctx)
	case c.SubjectPop:
		pop, err := bundle.LoadPopulation(c.Bundle)
		if err != nil {
			return c.unreadable(err)
		}
		return c.reportPopulation(pop)
	case c.Churn:
		churn, err := bundle.LoadChurn(c.Bundle)
		if err != nil {
			return c.unreadable(err)
		}
		return c.reportChurn(churn)
	default:
		return c.fromTrail()
	}
}

// validate reports why this invocation names no single report, or nil.
//
// Requires: nothing.
// Ensures: a usage error naming what to fix, never a bare refusal.
func (c *Config) validate(args []string) error {
	if len(args) != 0 {
		return c.usage(errors.New("audit takes no arguments; try `gnosis audit --outstanding`"))
	}
	if c.Since != "" && !c.Gained && !c.Unretrieved {
		return c.usage(errors.New(
			"--since bounds --gained and --unretrieved and applies to no other report"))
	}
	if chosen(
		c.Outstanding, c.Unsupported, c.Churn, c.Gained, c.SubjectPop, c.Unretrieved,
	) != 1 {
		// No default report — the command will grow `--reversed` (§10.6.5), so a bare
		// invocation that silently meant one of them would change meaning when the
		// next arrived. And two at once would interleave two lists on one stream with
		// nothing separating them.
		return c.usage(errors.New("audit needs exactly one report: " +
			"--outstanding, --unsupported, --churn, --gained, --subjects," +
			" or --unretrieved"))
	}
	return nil
}

// fromTrail answers the three reports that read the write trail.
func (c *Config) fromTrail() error {
	trail, err := bundle.AuditTrail(c.Bundle)
	if err != nil {
		return c.unreadable(err)
	}
	if c.Gained {
		since, sErr := c.window()
		if sErr != nil {
			return c.usage(sErr)
		}
		return c.reportGains(bundle.Gained(&trail, since))
	}
	if c.Unsupported {
		return c.reportWithheld(bundle.NotAuthorized(&trail))
	}
	drafts, err := bundle.Quarantined(c.Bundle)
	if err != nil {
		return c.unreadable(err)
	}
	return c.report(&Result{Undecided: bundle.Outstanding(&trail, drafts)})
}

// chosen counts how many reports were asked for.
func chosen(flags ...bool) int {
	n := 0
	for _, on := range flags {
		if on {
			n++
		}
	}
	return n
}

// reportChurn renders the churn register.
//
// **`ok`, not findings, and this is the one report here that is not a finding.** A
// source that moves is doing what sources do; §14.3.2 calls a benign drift "not a
// downgrade of trust" and rendering churn as something to fix would train a reader past
// the row that matters. Withdrawn support is already a finding where it happens — at
// the re-check that found it, and in `--unsupported`.
func (c *Config) reportChurn(churn *bundle.Churn) error {
	if c.JSONL {
		if err := c.EmitOK(churn); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		return nil
	}

	for i := range churn.Sources {
		s := &churn.Sources[i]
		_, _ = fmt.Fprintf(c.Stdout, "%d versions\t%s\t%s\n",
			s.Versions, outcomesOf(s), s.URI)
	}
	if len(churn.Sources) == 0 {
		_, _ = fmt.Fprintf(c.Stderr, "no source has moved, across %s\n",
			countOf(churn.Recorded, "recorded source"))
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr, "%s moved, out of %s\n",
		countOf(len(churn.Sources), "source"),
		countOf(churn.Recorded, "recorded source"))
	return nil
}

// outcomesOf renders what a source's moves cost, omitting the zeroes.
//
// The four are never summed. A benign move and a withdrawn passage are different
// events, and a single number covering both would be the bucket §14.3.2 split. Worst
// first, for the same reason the rows are ordered that way.
func outcomesOf(s *bundle.Churning) string {
	var parts []string
	for _, part := range []struct {
		n    int
		name string
	}{
		{s.Unsupported, "unsupported"},
		{s.Benign, "benign"},
		{s.Unchecked, "unchecked"},
		{s.Current, "current"},
	} {
		if part.n > 0 {
			parts = append(parts, strconv.Itoa(part.n)+" "+part.name)
		}
	}
	return strings.Join(parts, ", ")
}

// window is the start of --gained's period.
//
// Requires: nothing; an empty --since means the default.
// Ensures: a parse failure is a usage error naming the form, because a caller who typed
// a date wrongly needs the form and not a Go parse error.
//
// Thirty days by default, and a default rather than a required flag because the
// friction belongs on the *rate* this report refuses to compute, not on asking the
// question. Read from the clock here, which is why this is the shell.
func (c *Config) window() (time.Time, error) {
	if c.Since == "" {
		return time.Now().UTC().AddDate(0, 0, -defaultWindowDays), nil
	}
	at, err := time.Parse(time.DateOnly, c.Since)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"--since takes a date as YYYY-MM-DD, got %q", c.Since)
	}
	return at, nil
}

// reportGains renders what the corpus acquired.
//
// **`ok`, always, and this is the one report here that could not be anything else.**
// Every other report in this command counts something to act on. Exiting non-zero on
// good news would be the asymmetry this report exists to correct, arriving through the
// exit code.
func (c *Config) reportGains(gains *bundle.Gains) error {
	if c.JSONL {
		if err := c.EmitOK(gains); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		return c.warnIfFloor(gains.Complete(), gains.Unreadable)
	}

	since := gains.Since.Format(time.DateOnly)
	if !gains.Any() {
		// Said rather than printed as four zeroes: "nothing since the 23rd" and "we
		// did not look" must not render alike.
		_, _ = fmt.Fprintf(c.Stderr, "nothing gained since %s\n", since)
		return c.warnIfFloor(gains.Complete(), gains.Unreadable)
	}
	for _, row := range []struct {
		n    int
		what string
	}{
		{gains.Promoted, "documents promoted"},
		{gains.Admitted, "replies admitted"},
		{gains.Archived, "sources archived"},
		{gains.Declined, "drafts declined"},
	} {
		if row.n > 0 {
			_, _ = fmt.Fprintf(c.Stdout, "%d\t%s\n", row.n, row.what)
		}
	}
	_, _ = fmt.Fprintf(c.Stderr, "since %s\n", since)
	return c.warnIfFloor(gains.Complete(), gains.Unreadable)
}

// reportWithheld renders what the sources were found not to support.
//
// **Findings, like `--outstanding`, and for a different reason.** An outstanding
// decision is owed by a person; a withheld claim is something a model asserted and the
// archive refused, which is worth a caller's attention because a *repeated* one means
// the extraction keeps proposing something the source does not say. Either way the
// examination completed and found something, which is §17's `findings`.
func (c *Config) reportWithheld(result *bundle.Unauthorized) error {
	if c.JSONL {
		return c.emitWithheld(result)
	}

	for i := range result.Withheld {
		wh := &result.Withheld[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\n", wh.Source, wh.Submitter, wh.Claim)
	}
	if len(result.Withheld) == 0 {
		_, _ = fmt.Fprintf(c.Stderr, "no claims were withheld\n")
		return c.warnIfFloor(result.Complete(), result.Unreadable)
	}
	_, _ = fmt.Fprintf(c.Stderr, "%s withheld across %s\n",
		countOf(len(result.Withheld), "claim"), countOf(result.Sources, "source"))
	if wErr := c.warnIfFloor(result.Complete(), result.Unreadable); wErr != nil {
		return wErr
	}
	return root.ExitError(root.CodeFindings)
}

// emitWithheld writes the machine envelope for --unsupported.
func (c *Config) emitWithheld(result *bundle.Unauthorized) error {
	if len(result.Withheld) == 0 {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		return c.warnIfFloor(result.Complete(), result.Unreadable)
	}
	message := countOf(len(result.Withheld), "claim") + " withheld"
	if err := c.EmitFindings(root.ReasonNeedsHuman, message, result); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	if wErr := c.warnIfFloor(result.Complete(), result.Unreadable); wErr != nil {
		return wErr
	}
	return root.ExitError(root.CodeFindings)
}

// report renders the list.
//
// **An outstanding decision is a finding.** It is not `ok`: something is owed and a
// person is the only one who can supply it, so a CI job that wants to assert nobody
// left a promotion half-decided has an exit code to branch on. It is not `error`
// either — the examination completed and the tool worked, which is §17's distinction
// and the reason there is a fourth status at all.
//
// Nothing outstanding is `ok` and silent on stdout, so the empty case does not
// print a heading with nothing under it.
func (c *Config) report(result *Result) error {
	if c.JSONL {
		return c.emit(result)
	}

	for i := range result.Abandoned {
		a := &result.Abandoned[i]
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\n",
			a.Path, askedAt(a.Attempts, a.Asked))
	}
	if len(result.Abandoned) == 0 {
		_, _ = fmt.Fprintf(c.Stderr, "no decisions outstanding across %s\n",
			countOf(result.Drafts, "draft"))
		return c.warnIfPartial(result)
	}

	_, _ = fmt.Fprintf(c.Stderr, "%s outstanding across %s\n",
		countOf(len(result.Abandoned), "decision"), countOf(result.Drafts, "draft"))
	// What was needed, once per entry, because the list's whole value is that it
	// says what to do rather than only what is undone.
	for i := range result.Abandoned {
		if a := &result.Abandoned[i]; a.Reason != "" {
			_, _ = fmt.Fprintf(c.Stderr, "  %s: %s\n", a.Path, a.Reason)
		}
	}
	if wErr := c.warnIfPartial(result); wErr != nil {
		return wErr
	}
	return root.ExitError(root.CodeFindings)
}

// emit writes the machine envelope.
func (c *Config) emit(result *Result) error {
	if len(result.Abandoned) == 0 {
		if err := c.EmitOK(result); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		return c.warnIfPartial(result)
	}
	message := countOf(len(result.Abandoned), "decision") + " outstanding"
	if err := c.EmitFindings(root.ReasonNeedsHuman, message, result); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	if wErr := c.warnIfPartial(result); wErr != nil {
		return wErr
	}
	return root.ExitError(root.CodeFindings)
}

// warnIfPartial says so when the count is a floor.
//
// To stderr in both output modes, because the JSON carries `unreadable_lines` and a
// person reading a terminal does not read the JSON. The exit status is unchanged: the
// trail is this machine's cache, and `doctor` is what reports damage to it as a
// finding.
func (c *Config) warnIfPartial(result *Result) error {
	return c.warnIfFloor(result.Complete(), result.Unreadable)
}

// warnIfFloor says so when a count is a floor.
//
// One function for both reports, because both are folds over the same trail and a
// second copy of this sentence is a second chance to word it differently.
func (c *Config) warnIfFloor(complete bool, unreadable int) error {
	if complete {
		return nil
	}
	_, _ = fmt.Fprintf(c.Stderr,
		"warning: %d unreadable line(s) in the write trail, so this list is a "+
			"floor rather than a total; run `gnosis doctor` for the damage\n",
		unreadable)
	return nil
}

// askedAt renders when a decision was last put in front of somebody, and how many
// times.
//
// One phrase rather than two columns, because running the command produced
// "asked 2 times	asked 2026-08-23" — the word twice in one line, which reads as a
// formatting bug. The count is omitted for a single ask, which is the ordinary case
// and where a count is noise.
func askedAt(attempts int, when time.Time) string {
	on := "asked " + when.Format(time.DateOnly)
	if attempts <= 1 {
		return on
	}
	return on + ", " + strconv.Itoa(attempts) + " attempts"
}

// countOf renders a count with its noun pluralised, so a report reads as a sentence
// rather than as a field.
func countOf(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// fail and usage adapt root's reporting to this command's name.
// unreadable is every failure this command can have.
//
// The reason is not a parameter because `audit` reads and never writes: the trail,
// the drafts, tier 0 and the corpus can each fail to load, and there is no other kind
// of failure to distinguish. A parameter every caller passed the same value to said
// there was a choice being made here, and there is not.
func (c *Config) unreadable(cause error) error {
	return fmt.Errorf("audit: %w", c.Fail(root.ReasonNoBundle, cause))
}

func (c *Config) usage(cause error) error {
	return fmt.Errorf("audit: %w", c.Usage(cause))
}

// reportPopulation renders how the vocabulary is occupied.
//
// **`ok` always, never findings.** §17 forbids presenting a count as health, and a
// population is the most tempting such count there is: it looks like coverage and it
// can be raised by declaring subjects nobody uses. Exiting non-zero would turn an
// instrument into a target.
//
// The disjoint-evidence marker is the one row that is a signal rather than a number.
// It is the condition §5.8.2.1 records as the drift detector's trigger — two
// documents about one key, neither reading anything the other read — and it is shown
// because a reader scanning this table is exactly the person who can say whether the
// two halves are about the same thing.
func (c *Config) reportPopulation(pop *bundle.Population) error {
	if c.JSONL {
		if err := c.EmitOK(pop); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		return nil
	}

	if !pop.Any() {
		// Said rather than printed as an empty table: "no claim names a subject yet"
		// and "we did not look" must not render alike.
		_, _ = fmt.Fprintln(c.Stderr, "no claim names a subject yet")
		return nil
	}
	for i := range pop.Subjects {
		s := &pop.Subjects[i]
		line := fmt.Sprintf("%s\t%s\t%s\t%s",
			s.Key, countOf(s.Claims, "claim"), countOf(s.Documents, "document"),
			strings.Join(s.Surfaces, ", "))
		if s.DisjointEvidence {
			line += "\tno shared source"
		}
		_, _ = fmt.Fprintln(c.Stdout, line)
	}
	if pop.Unresolved > 0 {
		// Phrased so the noun carries the count and no verb has to agree with it.
		// The first draft of this line and of two lint messages read "1 claim name"
		// and "1 document declare" — a class of defect no substring assertion sees
		// and running the command shows immediately.
		_, _ = fmt.Fprintf(c.Stderr,
			"subject unresolved in %s; see `gnosis lint`\n",
			countOf(pop.Unresolved, "claim"))
	}
	if pop.Undeclared > 0 {
		_, _ = fmt.Fprintf(c.Stderr, "%s declared and unused\n",
			countOf(pop.Undeclared, "subject key"))
	}
	return nil
}

// reportReach answers §12.2's claim-grain question: which claims has nothing returned?
//
// **The report reads the index and the retrieval log and folds them purely**, which is
// why `bundle.Unreached` takes values rather than a database: every case it has to get
// right — a claim with no lead, a retrieval older than the window, a log that is empty —
// is testable from literals.
//
// **It skips when nothing has ever been searched**, rather than reporting the whole
// corpus. A corpus nobody has searched has not failed to be useful, and naming every
// claim in it on day one is the loudest thing this tool could say about nothing.
func (c *Config) reportReach(ctx context.Context) error {
	since, err := c.window()
	if err != nil {
		return c.usage(err)
	}
	log, err := bundle.LoadRetrievals(c.Bundle)
	if err != nil {
		return c.unreadable(err)
	}

	db, err := bundle.OpenIndexForRead(ctx, c.Bundle)
	if err != nil {
		return c.unreadable(err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.AllClaims(ctx)
	if err != nil {
		return c.unreadable(err)
	}
	claims := make([]bundle.QuietClaim, 0, len(rows))
	for _, r := range rows {
		claims = append(claims, bundle.QuietClaim{ClaimID: r.ID, Path: r.Path, Lead: r.Lead})
	}
	return c.renderReach(bundle.Unreached(claims, log, since), len(log))
}

// renderReach writes the reach report.
//
// **A count over a window and never a fraction** (§12.2, §17). The two numbers are
// printed side by side and no ratio is derived from them, because a
// proportion-of-the-corpus-retrieved figure looks like progress and rises when a claim is
// deleted.
//
// **"not observed returned", never "never retrieved".** Recording is per-user and
// best-effort, so the honest claim is about what this log saw. The wording carries the
// whole guarantee, and a stronger phrasing would be the report asserting reach it did not
// measure.
func (c *Config) renderReach(reach *bundle.Reach, logged int) error {
	if c.JSONL {
		if err := c.EmitOK(reach); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		return nil
	}
	if logged == 0 {
		_, _ = fmt.Fprintln(c.Stderr,
			"no search has been recorded yet, so nothing can be said about reach;"+
				" run `gnosis search --claims <QUERY>` first")
		return nil
	}
	for _, q := range reach.Quiet {
		_, _ = fmt.Fprintf(c.Stdout, "%s\t%s\t%s\n", q.Path, q.ClaimID, q.Lead)
	}
	_, _ = fmt.Fprintf(c.Stderr,
		"%d retrievable claim(s); %d observed returned since %s, %d not observed\n",
		reach.Claims, reach.Observed, reach.Window.Format(time.DateOnly),
		len(reach.Quiet))
	return nil
}
