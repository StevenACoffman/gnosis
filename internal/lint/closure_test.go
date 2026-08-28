package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
)

// tierZero builds a corpus whose archive and ledger are stated separately, which is
// the whole point: the check exists because they can disagree.
func tierZero(archived, recorded []string) *lint.Snapshot {
	snap := &lint.Snapshot{
		Documents:    []lint.Document{{ID: idA, Path: "c/a.md", Type: "Reference"}},
		ArchivedText: map[string]bool{},
		RecordedText: map[string]bool{},
	}
	for _, p := range archived {
		snap.ArchivedText[p] = true
	}
	for _, p := range recorded {
		snap.RecordedText[p] = true
	}
	return snap
}

// closureFindings are the diagnostics this check produced, by category.
func closureFindings(report lint.Report) map[string][]finding.Diagnostic {
	out := map[string][]finding.Diagnostic{}
	for _, d := range report.Diagnostics {
		if strings.HasPrefix(d.Category, "archive-orphan") ||
			strings.HasPrefix(d.Category, "archive-unrecorded") {
			out[d.Category] = append(out[d.Category], d)
		}
	}
	return out
}

// TestAClosedArchiveIsSilent, or the check reports every file in every corpus and
// nobody reads it twice.
func TestAClosedArchiveIsSilent(t *testing.T) {
	t.Parallel()

	const path = "evidence/text/aa/aaaa.md"
	report := lint.Run(tierZero([]string{path}, []string{path}), lint.Checks(testNow()))
	if got := closureFindings(report); len(got) != 0 {
		t.Errorf("a closed archive produced %v", got)
	}
}

// TestAnOrphanedFileIsAWarning. This is the state both backlog entries described from
// opposite directions — bundle closure, and a crash between the content write and the
// record write. Nothing is lost, so it is untidy rather than wrong.
func TestAnOrphanedFileIsAWarning(t *testing.T) {
	t.Parallel()

	const orphan = "evidence/text/bb/bbbb.md"
	report := lint.Run(tierZero([]string{orphan}, nil), lint.Checks(testNow()))

	got := closureFindings(report)["archive-orphan"]
	if len(got) != 1 {
		t.Fatalf("got %d orphan findings, want 1: %+v", len(got), got)
	}
	if got[0].Path != orphan {
		t.Errorf("path = %q, want the orphan", got[0].Path)
	}
	if got[0].Severity != finding.SeverityWarning {
		t.Errorf("severity = %q, want warning — nothing is lost", got[0].Severity)
	}
	// The cost is stated, because "there is an extra file" is not on its own a
	// reason for anybody to act.
	if !strings.Contains(got[0].Message, "budget") {
		t.Errorf("the finding does not say what an orphan costs: %q", got[0].Message)
	}
}

// TestARecordNamingAnAbsentFileIsAnError, and the severity is the reading: the ledger
// claims evidence tier 0 does not hold, so §9.4's invariant has nothing to check a
// quotation against. A claim resting on it is neither supported nor refuted.
func TestARecordNamingAnAbsentFileIsAnError(t *testing.T) {
	t.Parallel()

	const missing = "evidence/text/cc/cccc.md"
	report := lint.Run(tierZero(nil, []string{missing}), lint.Checks(testNow()))

	got := closureFindings(report)["archive-unrecorded"]
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(got), got)
	}
	if got[0].Severity != finding.SeverityError {
		t.Errorf("severity = %q, want error — the ledger claims what is not there",
			got[0].Severity)
	}
	if got[0].Path != missing {
		t.Errorf("path = %q", got[0].Path)
	}
}

// TestBothDirectionsAtOnce. The two halves are independent, and a corpus can be in
// both states — a prune that removed the wrong file and a crash that left another.
func TestBothDirectionsAtOnce(t *testing.T) {
	t.Parallel()

	report := lint.Run(tierZero(
		[]string{"evidence/text/aa/orphan.md", "evidence/text/bb/shared.md"},
		[]string{"evidence/text/bb/shared.md", "evidence/text/cc/absent.md"},
	), lint.Checks(testNow()))

	got := closureFindings(report)
	if len(got["archive-orphan"]) != 1 {
		t.Errorf("orphans = %+v, want the one unaccounted file", got["archive-orphan"])
	}
	if len(got["archive-unrecorded"]) != 1 {
		t.Errorf("unrecorded = %+v, want the one absent file", got["archive-unrecorded"])
	}
}

// TestOneFindingPerFile rather than one for the set. Each orphan is a separate
// decision about whether to keep or prune, and a single finding naming forty paths is
// one a reader defers rather than acts on.
func TestOneFindingPerFile(t *testing.T) {
	t.Parallel()

	report := lint.Run(tierZero([]string{
		"evidence/text/aa/one.md",
		"evidence/text/bb/two.md",
		"evidence/text/cc/three.md",
	}, nil), lint.Checks(testNow()))

	if got := closureFindings(report)["archive-orphan"]; len(got) != 3 {
		t.Errorf("got %d findings for three orphans: %+v", len(got), got)
	}
}

// TestTheCheckIsDeterministic. Both halves iterate maps, and a report that reordered
// itself between runs over one corpus would be unusable.
func TestTheClosureCheckIsDeterministic(t *testing.T) {
	t.Parallel()

	snap := tierZero(
		[]string{"evidence/text/cc/c.md", "evidence/text/aa/a.md", "evidence/text/bb/b.md"},
		[]string{"evidence/text/zz/z.md", "evidence/text/yy/y.md"},
	)
	first := renderClosure(lint.Run(snap, lint.Checks(testNow())))
	for range 8 {
		if got := renderClosure(lint.Run(snap, lint.Checks(testNow()))); got != first {
			t.Fatalf("two runs differ:\n%s\n%s", first, got)
		}
	}
	// Sorted within each half, which is what makes the comparison meaningful.
	if !strings.Contains(first, "aa/a.md") ||
		strings.Index(first, "aa/a.md") > strings.Index(first, "bb/b.md") {
		t.Errorf("the orphans are not sorted: %s", first)
	}
}

// renderClosure flattens the closure findings for a determinism comparison.
func renderClosure(report lint.Report) string {
	var b strings.Builder
	for _, d := range report.Diagnostics {
		if strings.HasPrefix(d.Category, "archive-") && d.Category != "archive-path" {
			b.WriteString(d.Category)
			b.WriteString(":")
			b.WriteString(d.Path)
			b.WriteString(" ")
		}
	}
	return b.String()
}

// TestTheCheckSkipsACorpusWithNoArchive. Derived applicability, per §12: a corpus
// that has fetched nothing has no closure to check, and "no orphans" and "nothing to
// look at" are different answers.
func TestTheClosureCheckSkipsAnEmptyArchive(t *testing.T) {
	t.Parallel()

	report := lint.Run(tierZero(nil, nil), lint.Checks(testNow()))

	var skipped bool
	for _, s := range report.Skipped {
		if s.Check == "archive-closure" {
			skipped = true
			if s.Reason == "" {
				t.Error("archive-closure skipped with no reason")
			}
		}
	}
	if !skipped {
		t.Errorf("the check ran on a corpus with no archive: %+v", report.Skipped)
	}
}
