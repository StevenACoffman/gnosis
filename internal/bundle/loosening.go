package bundle

import (
	"strconv"

	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/standards"
	"github.com/StevenACoffman/skillet/errs"
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
// Ensures: every threshold that moved in the loosening direction, from both
// standards files, with a finding delta where one is computable. A file absent at
// the revision is treated as the seed, because that is what the loader does today
// — a bundle that had no promote.toml was running the seed's values, and comparing
// against nothing would report every value as newly introduced.
func Loosenings(bundleDir, ref string) ([]Loosened, error) {
	const op = "bundle.Loosenings"

	out := make([]Loosened, 0)

	oldArchive, err := archiveAtRef(op, bundleDir, ref)
	if err != nil {
		return nil, err
	}
	curArchive, err := LoadArchiveStandards(bundleDir)
	if err != nil {
		return nil, err
	}
	for _, l := range standards.CompareArchive(oldArchive, curArchive) {
		out = append(out, describeLoosening(bundleDir, standards.ArchiveFileName, l,
			oldArchive, curArchive))
	}

	oldPromote, err := promoteAtRef(op, bundleDir, ref)
	if err != nil {
		return nil, err
	}
	curPromote, err := LoadPromoteStandards(bundleDir)
	if err != nil {
		return nil, err
	}
	for _, l := range standards.ComparePromote(oldPromote, curPromote) {
		out = append(out, Loosened{
			Loosening: l, File: standards.PromoteFileName,
			Why: "this threshold governs the promote gate and the rebuild floor, " +
				"neither of which produces a lint finding",
		})
	}
	return out, nil
}

// describeLoosening attaches a finding delta when the threshold produces findings.
//
// **Only the budget thresholds do.** `corpus_budget` and `corpus_warn_fraction`
// feed `doctor`'s archive-budget diagnostic, so raising either can silence a real
// finding and the delta is exact. The allowlist and the caps govern *admission* —
// they change which sources archive, not what any check reports — and
// `staleness_days` and `in_degree_cut` are read by nothing at all, which is its
// own problem and is in TODO.
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
	case "staleness_days", "in_degree_cut":
		out.Why = "nothing reads this threshold yet, so moving it changes no finding"
	default:
		out.Why = "this threshold governs admission to tier 0, not any check, " +
			"so moving it changes no finding count"
	}
	return out
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
