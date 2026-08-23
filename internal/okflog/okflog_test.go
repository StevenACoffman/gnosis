package okflog_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/okflog"
)

const sample = `# Log

Notes about this corpus.

## 2026-08-20

- Raised per_file_cap to 512 KiB; findings 14 → 11.

## 2026-08-15

- Initial import.
`

func TestParseSplitsDatedSections(t *testing.T) {
	t.Parallel()
	preamble, entries := okflog.Parse(sample)

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Date != "2026-08-20" || entries[1].Date != "2026-08-15" {
		t.Errorf("dates = %q, %q", entries[0].Date, entries[1].Date)
	}
	if !strings.Contains(strings.Join(preamble, "\n"), "Notes about this corpus") {
		t.Error("the preamble was lost")
	}
}

// TestRoundTripIsStable, so appending twice does not accumulate blank lines and a
// diff of two logs shows only what changed.
func TestRoundTripIsStable(t *testing.T) {
	t.Parallel()
	preamble, entries := okflog.Parse(sample)
	once := okflog.Render(preamble, entries)

	p2, e2 := okflog.Parse(once)
	if twice := okflog.Render(p2, e2); twice != once {
		t.Errorf("a second round trip changed the log:\n%q\n%q", once, twice)
	}
}

// TestNewSectionsGoFirst: a log read top-down should begin with what just
// happened.
func TestNewSectionsGoFirst(t *testing.T) {
	t.Parallel()
	got := okflog.Add(sample, "2026-08-21", "- Something new.")

	_, entries := okflog.Parse(got)
	if entries[0].Date != "2026-08-21" {
		t.Errorf("the newest entry is %q, not first", entries[0].Date)
	}
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}
}

// TestTodaysSectionIsReused rather than duplicated, and the note goes last:
// within one day the order events happened in is the order worth keeping.
func TestTodaysSectionIsReused(t *testing.T) {
	t.Parallel()
	got := okflog.Add(sample, "2026-08-20", "- A second thing that day.")

	_, entries := okflog.Parse(got)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 — the day was duplicated", len(entries))
	}
	body := strings.Join(entries[0].Lines, "\n")
	if !strings.Contains(body, "Raised per_file_cap") || !strings.Contains(body, "second thing") {
		t.Errorf("the section lost content:\n%s", body)
	}
	if strings.Index(body, "Raised") > strings.Index(body, "second thing") {
		t.Error("the new note was placed before the existing one")
	}
}

// TestAddIsPure, so a caller previewing an entry sees exactly what will be written.
func TestAddIsPure(t *testing.T) {
	t.Parallel()
	first := okflog.Add(sample, "2026-08-21", "- A note.")
	for range 20 {
		if again := okflog.Add(sample, "2026-08-21", "- A note."); again != first {
			t.Fatalf("two adds differ:\n%q\n%q", first, again)
		}
	}
}

func TestAddToAnEmptyLog(t *testing.T) {
	t.Parallel()
	got := okflog.Add("", "2026-08-21", "- The first note.")
	if !strings.HasPrefix(got, "## 2026-08-21\n") {
		t.Errorf("an empty log did not gain a heading:\n%q", got)
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("want exactly one trailing newline:\n%q", got)
	}
}

// TestANonDateHeadingIsNotAnEntry. The `log-format` check already reports one, and
// a parser that rejected what a linter merely flags would have the two disagreeing
// about the same file.
func TestANonDateHeadingIsNotAnEntry(t *testing.T) {
	t.Parallel()
	src := "## Not a date\n\n- Something.\n\n## 2026-08-20\n\n- Dated.\n"

	_, entries := okflog.Parse(src)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want only the dated one", len(entries))
	}
	if entries[0].Date != "2026-08-20" {
		t.Errorf("date = %q", entries[0].Date)
	}
	// And it survives a round trip rather than being dropped.
	preamble, e := okflog.Parse(src)
	if !strings.Contains(okflog.Render(preamble, e), "Not a date") {
		t.Error("the undated heading was discarded")
	}
}

func TestSinceFiltersByDate(t *testing.T) {
	t.Parallel()
	_, entries := okflog.Parse(sample)

	cases := map[string]int{"": 2, "2026-08-16": 1, "2026-08-20": 1, "2026-09-01": 0}
	for since, want := range cases {
		if got := len(okflog.Since(entries, since)); got != want {
			t.Errorf("since %q = %d entries, want %d", since, got, want)
		}
	}
}
