package lint

import (
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// ArchiveSize is what the evidence store currently costs, gathered by the caller.
//
// Zero for a corpus that has fetched nothing, which is the ordinary state of a
// fresh bundle and not a condition to report.
type ArchiveSize struct {
	// Bytes is the total size of everything under evidence/.
	Bytes int64 `json:"bytes"`

	// Budget and WarnFraction come from standards/ (§4.3). A zero Budget means
	// no budget was declared, and nothing is reported.
	Budget       int64   `json:"budget"`
	WarnFraction float64 `json:"warn_fraction"`

	// Largest names the biggest files, biggest first, so a warning is actionable
	// rather than only alarming. §9.2 requires this: a caller told the archive is
	// large and not told what is large in it has to go and look.
	Largest []ArchiveFile `json:"largest,omitempty"`
}

// ArchiveFile is one archived file and its size.
type ArchiveFile struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// DiagnoseBudget reports an archive approaching or past its declared budget.
//
// Requires: size was gathered from one bundle at one moment.
// Ensures: nothing when no budget is declared or the archive is under the warning
// threshold; one warning between the threshold and the budget; one error past it.
// Sorted by the caller, as every other diagnostic is.
//
// **It reports and never refuses**, which §9.2 states as a rule and which is worth
// keeping the reason for: "growth that nobody was told about is the failure mode".
// The budget is not a limit on what a corpus may hold — a team that genuinely needs
// 300 MB of evidence should have it — it is a promise that nobody arrives at 300 MB
// without having been told on the way. A gate here would make the tool refuse
// evidence somebody deliberately gathered, which is the opposite of the point.
//
// Past the budget it becomes an error rather than a warning, and that is a
// severity change rather than a behaviour change: still nothing refuses. The
// escalation exists because a warning that has been true for six months has
// stopped being read.
func DiagnoseBudget(size *ArchiveSize) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	if size.Budget <= 0 || size.WarnFraction <= 0 {
		return out
	}

	threshold := int64(float64(size.Budget) * size.WarnFraction)
	switch {
	case size.Bytes > size.Budget:
		out = append(out, budgetFinding(size, finding.SeverityError,
			"the evidence archive is over its declared budget"))
	case size.Bytes >= threshold:
		out = append(out, budgetFinding(size, finding.SeverityWarning,
			"the evidence archive is approaching its declared budget"))
	}
	return out
}

// budgetFinding renders the diagnostic, naming both numbers and the largest files.
func budgetFinding(
	size *ArchiveSize,
	severity finding.Severity,
	headline string,
) finding.Diagnostic {
	message := headline + ": " + humanBytes(size.Bytes) + " of " +
		humanBytes(size.Budget)
	if len(size.Largest) > 0 {
		message += "; largest: " + namedFiles(size.Largest)
	}
	return finding.Diagnostic{
		Severity: severity,
		Category: "archive-budget",
		Path:     "evidence/",
		Message:  message,
		Action:   finding.ActionGuided,
	}
}

// namedFiles renders the largest files, bounded so a message stays readable.
func namedFiles(files []ArchiveFile) string {
	const shown = 5

	if len(files) > shown {
		files = files[:shown]
	}
	parts := make([]string, 0, len(files))
	for _, f := range files {
		parts = append(parts, f.Path+" ("+humanBytes(f.Bytes)+")")
	}
	return strings.Join(parts, ", ")
}

// humanBytes renders a size the way a person reads one.
//
// Binary units, because the budget is written in them: 256 KiB and 256 MiB are the
// declared values, and reporting 268.4 MB against a 256 MiB budget would make a
// reader do arithmetic to find out whether they are over.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for size := n / unit; size >= unit; size /= unit {
		div *= unit
		exp++
	}
	whole := float64(n) / float64(div)
	return strconv.FormatFloat(whole, 'f', 1, 64) + " " + []string{"KiB", "MiB", "GiB", "TiB"}[exp]
}

// SortArchiveFiles orders files biggest first, so a caller can take the head.
//
// Ties break on path, so two runs over one archive produce the same list and a
// diff of two reports means something.
func SortArchiveFiles(files []ArchiveFile) {
	sort.Slice(files, func(i, j int) bool {
		if files[i].Bytes != files[j].Bytes {
			return files[i].Bytes > files[j].Bytes
		}
		return files[i].Path < files[j].Path
	})
}
