package bundle

import (
	"os"
	"strconv"
	"time"

	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/standards"
	"github.com/StevenACoffman/skillet/errs"
)

// The reasons a loosening has no finding count, stated once each.
//
// They were three literals at three branches of the switch below, and a fourth
// category arrived as a fourth sentence somebody wrote from memory. Naming them
// makes an addition visible and keeps the wording one thing (**rules.md §13**).
const (
	// whyAdmission: the threshold decides what enters tier 0. Moving it changes
	// which sources archive, and no check counts that.
	whyAdmission = "this threshold governs admission to tier 0, not any check, " +
		"so moving it changes no finding count"

	// whyGate: the threshold reaches a gate verdict but no lint finding. That is a
	// third category and it did not exist until §9.3 stage 4 applied the archive's
	// caps to a candidate document — before that these were admission-only, and
	// saying so is now half true in a way a reader would act on.
	whyGate = "this threshold changes no lint finding and does change a promote " +
		"gate verdict: §9.3 stage 4 applies it to a candidate document, so a " +
		"loosening admits candidates the gate previously refused"

	// **There is no `whyUnread` any more.** It said "nothing reads this threshold
	// yet, so moving it changes no finding" and had exactly one user,
	// `in_degree_cut`, which gained the `durability` check as its reader on
	// 2026-09-02. A constant kept for a case that no longer exists is a claim that
	// some threshold is unread, and none is — so it was deleted rather than left
	// waiting, which is what the linter asked for and the right answer anyway. It
	// comes back with the first value declared without a reader, and
	// `standards.Unread` is what will name that value.
)

// Loosened is one threshold that moved in the finding-reducing direction, and
// what that cost in findings if the cost is knowable.
type Loosened struct {
	standards.Loosening

	// File is which standards file it moved in.
	File string `json:"file"`

	// FindingsBefore and FindingsAfter are the count this corpus produced under
	// each value, and Countable says whether they mean anything.
	//
	// §6.2 requires a loosening to be recorded "alongside the finding count before
	// and after", and for most thresholds that number **does not exist**: they
	// govern admission or the promote gate rather than any check, so moving one
	// changes no count at all. Reporting a zero delta for those would be worse
	// than reporting none — it reads as "this loosening cost nothing", when what
	// happened is that nobody measured what it cost.
	FindingsBefore int  `json:"findings_before,omitempty"`
	FindingsAfter  int  `json:"findings_after,omitempty"`
	Countable      bool `json:"countable"`

	// Why explains an uncountable delta, so a reader is told rather than left to
	// infer it from two absent numbers.
	Why string `json:"why,omitempty"`
}

// Loosenings compares this bundle's standards against a git revision.
//
// Requires: bundleDir is inside a git worktree; ref names a revision.
// Ensures: every threshold that moved in the loosening direction, from every standards
// file, with a finding delta where one is computable. A file absent at the revision is
// treated as the seed, because that is what the loader does today — a bundle that had no
// promote.toml was running the seed's values, and comparing against nothing would report
// every value as newly introduced.
//
// **One function per file, and the fourth one is what forced it.** This was a single
// body repeating read-old, read-current, compare, describe; the fourth repetition put it
// past the complexity limit, which was the right moment to notice that each file has its
// own loosening story — a countable delta for one, a gate verdict for another, a
// question nobody counts for a third. Decomposed by that knowledge rather than by the
// order the reads happen in.
func Loosenings(bundleDir, ref string) ([]Loosened, error) {
	const op = "bundle.Loosenings"

	out := make([]Loosened, 0)
	for _, of := range []func(op, bundleDir, ref string) ([]Loosened, error){
		archiveLoosenings, promoteLoosenings, sampleLoosenings, challengeLoosenings,
	} {
		found, err := of(op, bundleDir, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

// archiveLoosenings reports the tier-0 gates that moved, with a finding delta where the
// threshold produces one.
func archiveLoosenings(op, bundleDir, ref string) ([]Loosened, error) {
	old, err := archiveAtRef(op, bundleDir, ref)
	if err != nil {
		return nil, err
	}
	cur, err := LoadArchiveStandards(bundleDir)
	if err != nil {
		return nil, err
	}
	var out []Loosened
	for _, l := range standards.CompareArchive(old, cur) {
		out = append(out, describeLoosening(
			bundleDir, standards.ArchiveFileName, l, old, cur))
	}
	return out, nil
}

// promoteLoosenings reports the gate thresholds that moved.
//
// None of them produces a lint finding, so none carries a delta — stated once here
// rather than recomputed per value, because it is a fact about the file.
func promoteLoosenings(op, bundleDir, ref string) ([]Loosened, error) {
	old, err := promoteAtRef(op, bundleDir, ref)
	if err != nil {
		return nil, err
	}
	cur, err := LoadPromoteStandards(bundleDir)
	if err != nil {
		return nil, err
	}
	var out []Loosened
	for _, l := range standards.ComparePromote(old, cur) {
		out = append(out, Loosened{
			Loosening: l, File: standards.PromoteFileName,
			Why: "this threshold governs the promote gate and the rebuild floor, " +
				"neither of which produces a lint finding",
		})
	}
	return out, nil
}

// sampleLoosenings reports a shrunken critic draw.
//
// **No delta, and the reason is not that nobody built one.** A critic verdict is a
// model's answer, not a check's finding, so the count before and after would mean
// running the critic twice over one corpus — two model calls to report that a smaller
// sample asks fewer questions, which is what the value already says.
func sampleLoosenings(op, bundleDir, ref string) ([]Loosened, error) {
	old, err := sampleAtRef(op, bundleDir, ref)
	if err != nil {
		return nil, err
	}
	cur, err := LoadSampleStandards(bundleDir)
	if err != nil {
		return nil, err
	}
	var out []Loosened
	for _, l := range standards.CompareSample(old, cur) {
		out = append(out, Loosened{
			Loosening: l, File: standards.SampleFileName,
			Why: "a smaller default draw asks the critic fewer questions; a critic " +
				"verdict is a model's answer rather than a check's finding, so the " +
				"delta would cost two model runs to report what the value says",
		})
	}
	return out, nil
}

// challengeLoosenings reports a widened unanswered window, with its exact delta.
func challengeLoosenings(op, bundleDir, ref string) ([]Loosened, error) {
	old, err := challengeAtRef(op, bundleDir, ref)
	if err != nil {
		return nil, err
	}
	cur, err := LoadChallengeStandards(bundleDir)
	if err != nil {
		return nil, err
	}
	var out []Loosened
	for _, l := range standards.CompareChallenge(old, cur) {
		out = append(out, describeChallengeLoosening(bundleDir, l, old, cur))
	}
	return out, nil
}

// describeLoosening attaches a finding delta when the threshold produces findings,
// and says why when it does not.
//
// Requires: l came from standards.CompareArchive; old and cur are the loaded
// standards either side of the change.
// Ensures: either an exact before/after count or a stated reason there is none.
// Never a zero delta standing in for an unmeasured one — §6.2's argument is that a
// zero reads as *this cost nothing*, when what happened is that nobody measured it.
//
// # The categories, and one that was wrong
//
// `corpus_budget` and `corpus_warn_fraction` feed the archive-budget diagnostic, so
// raising either can silence a real finding and the delta is exact.
//
// `staleness_days` **also** produces a count now, and this function said it did not.
// It read "nothing reads this threshold yet", which was true when it was written and
// stopped being true when the `stale` check gained its window: widening the window
// silences `stale` findings, and `standards check --log` was recording that it cost
// nothing. That is the precise reassurance §6.2 exists to withhold, produced by the
// tool §6.2 asked for — and it is why the test below cross-checks every case here
// against `standards.Unread`, which is the one function that already knows.
//
// `per_file_cap` and `embedded_payload_cap` are the third category: no lint finding,
// and a promote-gate verdict, since §9.3 stage 4 applies them to a candidate.
//
// `in_degree_cut` produces a count as of 2026-09-02, and this comment said it was
// "genuinely unread" until then. Raising the cut moves documents out of
// `load-bearing-weak` and into the peripheral count, which silences findings — the same
// shape `staleness_days` had, and the same way this function was wrong about it. Both
// were true when written; the test below is what stops the third one being true for a
// week.
//
// `allowlist` is admission only.
func describeLoosening(
	bundleDir, file string, l standards.Loosening, old, cur *standards.Archive,
) Loosened {
	out := Loosened{Loosening: l, File: file}

	switch l.Key {
	case "corpus_budget", "corpus_warn_fraction":
		before, after, err := budgetFindingDelta(bundleDir, old, cur)
		if err != nil {
			out.Why = "the archive could not be measured: " + err.Error()
			return out
		}
		out.FindingsBefore, out.FindingsAfter, out.Countable = before, after, true
	case "staleness_days":
		before, after, err := staleFindingDelta(bundleDir, old, cur)
		if err != nil {
			out.Why = "the corpus could not be read: " + err.Error()
			return out
		}
		out.FindingsBefore, out.FindingsAfter, out.Countable = before, after, true
	case "in_degree_cut":
		before, after, err := durabilityFindingDelta(bundleDir, old, cur)
		if err != nil {
			out.Why = "the corpus could not be read: " + err.Error()
			return out
		}
		out.FindingsBefore, out.FindingsAfter, out.Countable = before, after, true
	case "per_file_cap", "embedded_payload_cap":
		out.Why = whyGate
	default:
		out.Why = whyAdmission
	}
	return out
}

// staleFindingDelta counts the stale findings under each window.
//
// Requires: bundleDir is a bundle root; old and cur differ in StalenessDays.
// Ensures: the two counts, or an error when the corpus cannot be read.
//
// The corpus is read once and the check run twice, which is what makes the delta
// exact rather than an estimate: both counts describe the same documents and the same
// check records, and differ only in the window applied to them. The same shape
// budgetFindingDelta uses, for the same reason.
//
// The clock is `time.Now` and that is a real dependency rather than a seam this
// forgot: the count is "how many sources are stale *today*", and a fixed clock would
// answer a question nobody asked. It makes the number a measurement rather than a
// property, which is what a log entry recording a threshold change wants.
func staleFindingDelta(bundleDir string, old, cur *standards.Archive) (int, int, error) {
	const op = "bundle.staleFindingDelta"

	fresh, err := LoadFreshness(bundleDir)
	if err != nil {
		return 0, 0, err
	}
	snap, err := Snapshot(os.DirFS(bundleDir), IndexState{}, fresh)
	if err != nil {
		return 0, 0, &errs.Error{Op: op, Err: err}
	}

	now := time.Now().UTC()
	snap.StalenessDays = old.StalenessDays.Value
	before := len(lint.StaleFindings(snap, now))

	snap.StalenessDays = cur.StalenessDays.Value
	after := len(lint.StaleFindings(snap, now))

	return before, after, nil
}

// durabilityFindingDelta counts the durability findings under each in-degree cut.
//
// Requires: bundleDir is a bundle root; old and cur differ in InDegreeCut.
// Ensures: the two counts, or an error when the corpus cannot be read.
//
// The corpus is read once and the check run twice, the shape both sibling deltas use:
// both counts describe the same documents and the same tier-0 records, and differ only
// in the cut applied to them.
//
// The suppressed-peripheral diagnostic is counted with the rest, deliberately. Raising
// the cut turns a page's worth of load-bearing findings into one suppression line, and
// counting only the reported class would show that as a reduction of exactly the size
// somebody hoped for — while counting the line keeps the arithmetic honest about what
// the change did.
func durabilityFindingDelta(bundleDir string, old, cur *standards.Archive) (int, int, error) {
	const op = "bundle.durabilityFindingDelta"

	fresh, err := LoadFreshness(bundleDir)
	if err != nil {
		return 0, 0, err
	}
	snap, err := Snapshot(os.DirFS(bundleDir), IndexState{}, fresh)
	if err != nil {
		return 0, 0, &errs.Error{Op: op, Err: err}
	}

	snap.InDegreeCut = old.InDegreeCut.Value
	before := len(lint.DurabilityFindings(snap))

	snap.InDegreeCut = cur.InDegreeCut.Value
	after := len(lint.DurabilityFindings(snap))

	return before, after, nil
}

// budgetFindingDelta counts the budget diagnostics under each value.
//
// The archive is measured once and diagnosed twice, which is what makes the delta
// exact rather than an estimate: both counts describe the same bytes on disk and
// differ only in the threshold applied to them.
func budgetFindingDelta(bundleDir string, old, cur *standards.Archive) (int, int, error) {
	size, err := MeasureArchive(bundleDir, old.CorpusBudget.Value, old.CorpusWarnFraction.Value)
	if err != nil {
		return 0, 0, err
	}
	before := len(lint.DiagnoseBudget(&size))

	size.Budget = cur.CorpusBudget.Value
	size.WarnFraction = cur.CorpusWarnFraction.Value
	after := len(lint.DiagnoseBudget(&size))

	return before, after, nil
}

// archiveAtRef loads the archive standards as they stood at a revision, falling
// back to the seed when the file was not there.
func archiveAtRef(op, bundleDir, ref string) (*standards.Archive, error) {
	src, err := FileAtRef(bundleDir, ref, standards.ArchiveFileName)
	if err != nil {
		if errs.ErrorCode(err) != errs.ENOTFOUND {
			return nil, err
		}
		src = standards.DefaultArchive()
	}
	a, err := standards.LoadArchive(src)
	if err != nil {
		return nil, &errs.Error{Op: op, Message: op + ": at " + ref, Err: err}
	}
	return a, nil
}

// promoteAtRef is archiveAtRef for the gate thresholds.
func promoteAtRef(op, bundleDir, ref string) (*standards.Promote, error) {
	src, err := FileAtRef(bundleDir, ref, standards.PromoteFileName)
	if err != nil {
		if errs.ErrorCode(err) != errs.ENOTFOUND {
			return nil, err
		}
		src = standards.DefaultPromote()
	}
	p, err := standards.LoadPromote(src)
	if err != nil {
		return nil, &errs.Error{Op: op, Message: op + ": at " + ref, Err: err}
	}
	return p, nil
}

// LogEntry renders a loosening as the line §6.2 wants in log.md.
//
// Requires: l describes one threshold.
// Ensures: one line, naming the file, the key, both values, and the finding delta
// when there is one. Where there is not, it says why rather than omitting the
// clause — an entry that quietly drops the count reads as though nobody thought
// about it, and §6.2's whole argument is that the count is what distinguishes a
// threshold that was wrong from one that was inconvenient.
func (l *Loosened) LogEntry() string {
	line := "- Loosened `" + l.Key + "` in `" + l.File + "`: " +
		l.From + " to " + l.To + ". "
	if l.Countable {
		line += "Findings " + strconv.Itoa(l.FindingsBefore) + " to " +
			strconv.Itoa(l.FindingsAfter) + "."
	} else {
		line += "No finding count: " + l.Why + "."
	}
	return line
}

// describeChallengeLoosening attaches the finding delta for a widened window.
//
// Requires: l came from standards.CompareChallenge.
// Ensures: an exact before/after count, or a stated reason there is none.
//
// It is a function rather than a case in describeLoosening's switch because that one
// takes a `*standards.Archive` pair: the two files are different types, and widening the
// switch to take `any` would trade a compile-time guarantee for a runtime one to save a
// function.
func describeChallengeLoosening(
	bundleDir string, l standards.Loosening, old, cur *standards.Challenge,
) Loosened {
	out := Loosened{Loosening: l, File: standards.ChallengeFileName}
	before, after, err := challengeFindingDelta(bundleDir, old, cur)
	if err != nil {
		out.Why = "the corpus could not be read: " + err.Error()
		return out
	}
	out.FindingsBefore, out.FindingsAfter, out.Countable = before, after, true
	return out
}

// challengeFindingDelta counts the unanswered-challenge findings under each window.
//
// Requires: bundleDir is a bundle root; old and cur differ in UnansweredDays.
// Ensures: the two counts, or an error when the corpus cannot be read.
//
// The corpus is read once and the check run twice, the shape every sibling delta uses.
// The clock is `time.Now` for the reason staleFindingDelta's is: the count is "how many
// challenges are overdue *today*", and a fixed clock would answer a question nobody
// asked.
func challengeFindingDelta(
	bundleDir string, old, cur *standards.Challenge,
) (int, int, error) {
	const op = "bundle.challengeFindingDelta"

	fresh, err := LoadFreshness(bundleDir)
	if err != nil {
		return 0, 0, err
	}
	snap, err := Snapshot(os.DirFS(bundleDir), IndexState{}, fresh)
	if err != nil {
		return 0, 0, &errs.Error{Op: op, Err: err}
	}

	now := time.Now().UTC()
	snap.ChallengeDays = old.UnansweredDays.Value
	before := len(lint.AgedChallenges(snap, now))

	snap.ChallengeDays = cur.UnansweredDays.Value
	after := len(lint.AgedChallenges(snap, now))

	return before, after, nil
}

// challengeAtRef is archiveAtRef for the challenge window.
func challengeAtRef(op, bundleDir, ref string) (*standards.Challenge, error) {
	src, err := FileAtRef(bundleDir, ref, standards.ChallengeFileName)
	if err != nil {
		if errs.ErrorCode(err) != errs.ENOTFOUND {
			return nil, err
		}
		src = standards.DefaultChallenge()
	}
	c, err := standards.LoadChallenge(src)
	if err != nil {
		return nil, &errs.Error{Op: op, Message: op + ": at " + ref, Err: err}
	}
	return c, nil
}

// sampleAtRef is archiveAtRef for the draw configuration.
func sampleAtRef(op, bundleDir, ref string) (*standards.Sample, error) {
	src, err := FileAtRef(bundleDir, ref, standards.SampleFileName)
	if err != nil {
		if errs.ErrorCode(err) != errs.ENOTFOUND {
			return nil, err
		}
		src = standards.DefaultSample()
	}
	s, err := standards.LoadSample(src)
	if err != nil {
		return nil, &errs.Error{Op: op, Message: op + ": at " + ref, Err: err}
	}
	return s, nil
}
