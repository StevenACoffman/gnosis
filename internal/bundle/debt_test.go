package bundle_test

import (
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// promotedRow builds a successful promotion row carrying the given signals.
func promotedRow(path, actor string, signals ...string) audit.Row {
	return audit.Row{
		At:      time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC),
		Op:      audit.OpPromote,
		Actor:   actor,
		Paths:   []string{path},
		Outcome: string(gnosis.StatusOK),
		Detail:  "promoted from quarantine over unrun signals: checked by hand",
		Signals: signals,
	}
}

// TestOwedCountsPerSignal, because the question a reader has is per signal — "which
// claims were admitted with no conflict check" — and a row listing three signals
// against one path would have to be regrouped by every caller.
func TestOwedCountsPerSignal(t *testing.T) {
	t.Parallel()

	got := bundle.Owed(&bundle.Trail{Rows: []audit.Row{
		promotedRow("c/a.md", "human:priya", "conflict", "security"),
		promotedRow("c/b.md", "human:sam", "conflict"),
	}})

	if got.BySignal["conflict"] != 2 {
		t.Errorf("conflict = %d, want 2", got.BySignal["conflict"])
	}
	if got.BySignal["security"] != 1 {
		t.Errorf("security = %d, want 1", got.BySignal["security"])
	}
	if strings.Join(got.Signals, ",") != "conflict,security" {
		t.Errorf("signals = %v, want sorted", got.Signals)
	}
	if len(got.Carried) != 3 {
		t.Errorf("carried %d entries, want one per (path, signal)", len(got.Carried))
	}
	if got.Rows != 2 {
		t.Errorf("promotions = %d, want 2", got.Rows)
	}
}

// TestOwedCountsTheDenominator. "34 documents were admitted with no conflict check"
// means something different against 40 promotions than against 4000, so a clean
// promotion still counts toward Rows.
func TestOwedCountsTheDenominator(t *testing.T) {
	t.Parallel()

	clean := promotedRow("c/clean.md", "human:priya")
	clean.Signals = nil
	got := bundle.Owed(&bundle.Trail{Rows: []audit.Row{
		clean,
		promotedRow("c/carried.md", "human:priya", "conflict"),
	}})

	if got.Rows != 2 {
		t.Errorf("promotions = %d, want both", got.Rows)
	}
	if len(got.Carried) != 1 {
		t.Errorf("carried %d, want only the one that was", len(got.Carried))
	}
}

// TestARefusedPromotionIsNotDebt. A refusal records its unrun signals too — §9.5
// wants to know that a document may have been blocked by a check nobody built — but
// that document is not in the corpus, and counting it would report debt the corpus
// is not carrying.
func TestARefusedPromotionIsNotDebt(t *testing.T) {
	t.Parallel()

	refused := promotedRow("c/refused.md", "human:priya", "conflict")
	refused.Outcome = string(gnosis.StatusBlocked)

	got := bundle.Owed(&bundle.Trail{Rows: []audit.Row{refused}})
	if len(got.Carried) != 0 {
		t.Errorf("a refused promotion was counted as debt: %+v", got.Carried)
	}
	if got.Rows != 0 {
		t.Errorf("promotions = %d; a refusal is not a promotion", got.Rows)
	}
}

// TestOtherOperationsAreNotDebt. `fetch`, `admit`, `init`, and `rebuild` all write
// rows, and none of them admits a claim to the corpus.
func TestOtherOperationsAreNotDebt(t *testing.T) {
	t.Parallel()

	for _, op := range []audit.Op{audit.OpFetch, audit.OpAdmit, audit.OpInit, audit.OpRebuild} {
		row := promotedRow("c/a.md", "human:priya", "conflict")
		row.Op = op
		if got := bundle.Owed(&bundle.Trail{Rows: []audit.Row{row}}); len(got.Carried) != 0 {
			t.Errorf("%s was counted as debt", op)
		}
	}
}

// TestADamagedTrailMakesTheCountAFloor. This is the one thing the register must not
// get wrong: a total computed from a trail with unreadable lines is smaller than the
// truth, and reporting it as a total is the flattering direction.
func TestADamagedTrailMakesTheCountAFloor(t *testing.T) {
	t.Parallel()

	intact := bundle.Owed(&bundle.Trail{
		Rows: []audit.Row{promotedRow("c/a.md", "human:priya", "conflict")},
	})
	if !intact.Complete() {
		t.Error("an intact trail reported an incomplete count")
	}

	damaged := bundle.Owed(&bundle.Trail{
		Rows:      []audit.Row{promotedRow("c/a.md", "human:priya", "conflict")},
		Malformed: []int{7, 9},
	})
	if damaged.Complete() {
		t.Error("a damaged trail reported a complete count")
	}
	if damaged.Unreadable != 2 {
		t.Errorf("unreadable = %d, want 2", damaged.Unreadable)
	}
	// The rows that did parse are still reported. A partial answer about history is
	// what Trail exists to make usable; refusing to count at all would be worse.
	if len(damaged.Carried) != 1 {
		t.Errorf("a damaged trail dropped the rows it could read: %+v", damaged.Carried)
	}
}

// TestTheZeroDebtIsComplete, because a corpus that has promoted nothing is not a
// corpus whose count is in doubt.
func TestTheZeroDebtIsComplete(t *testing.T) {
	t.Parallel()

	got := bundle.Owed(&bundle.Trail{})
	if !got.Complete() {
		t.Error("an empty trail reported an incomplete count")
	}
	if got.Carried == nil || got.Signals == nil {
		t.Error("empty debt returned nil slices; a caller would have to check")
	}
}

// TestOwedIsDeterministic. A report that reordered itself between runs over one
// trail would be unusable, and the entries come out of a map.
func TestOwedIsDeterministic(t *testing.T) {
	t.Parallel()

	rows := []audit.Row{
		promotedRow("c/c.md", "human:sam", "security", "conflict"),
		promotedRow("c/a.md", "human:priya", "conflict"),
		promotedRow("c/b.md", "human:priya", "security"),
	}
	first := render(bundle.Owed(&bundle.Trail{Rows: rows}))
	for range 8 {
		if got := render(bundle.Owed(&bundle.Trail{Rows: rows})); got != first {
			t.Fatalf("two runs differ:\n%s\n%s", first, got)
		}
	}
	// Sorted by path then signal, which is what makes it comparable at all.
	if !strings.HasPrefix(first, "c/a.md/conflict") {
		t.Errorf("not sorted by path: %s", first)
	}
}

// TestPathsCountsEachDocumentOnce. It is the population a sample draws from, and
// sampling the (path, signal) pairs would draw one document three times and call it
// three observations.
func TestPathsCountsEachDocumentOnce(t *testing.T) {
	t.Parallel()

	got := bundle.Owed(&bundle.Trail{Rows: []audit.Row{
		promotedRow("c/a.md", "human:priya", "conflict", "security"),
		promotedRow("c/b.md", "human:sam", "conflict"),
	}}).Paths()

	if strings.Join(got, ",") != "c/a.md,c/b.md" {
		t.Errorf("paths = %v, want each document once, sorted", got)
	}
}

// TestRestrictedKeepsTheDenominator. A sample that also shrank its own denominator
// would report a rate of one, which is the arithmetic §17 spends a section on.
func TestRestrictedKeepsTheDenominator(t *testing.T) {
	t.Parallel()

	all := bundle.Owed(&bundle.Trail{Rows: []audit.Row{
		promotedRow("c/a.md", "human:priya", "conflict"),
		promotedRow("c/b.md", "human:sam", "conflict"),
		promotedRow("c/c.md", "human:sam", "security"),
	}})
	narrowed := all.Restricted([]string{"c/a.md"})

	if narrowed.Rows != all.Rows {
		t.Errorf("promotions = %d, want the trail's %d", narrowed.Rows, all.Rows)
	}
	if len(narrowed.Carried) != 1 {
		t.Errorf("carried = %+v, want only the selected path", narrowed.Carried)
	}
	if narrowed.BySignal["security"] != 0 {
		t.Error("a signal outside the selection was counted")
	}
	if strings.Join(narrowed.Signals, ",") != "conflict" {
		t.Errorf("signals = %v, want only the selected one", narrowed.Signals)
	}
}

// render flattens a Debt to a comparable string, so a determinism assertion reads
// as one comparison rather than a nested loop.
func render(d *bundle.Debt) string {
	var b strings.Builder
	for i := range d.Carried {
		b.WriteString(d.Carried[i].Path)
		b.WriteString("/")
		b.WriteString(d.Carried[i].Signal)
		b.WriteString(" ")
	}
	b.WriteString("|")
	b.WriteString(strings.Join(d.Signals, ","))
	return b.String()
}
