package bundle_test

import (
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// noon is a fixed instant the cases offset from, so no test depends on the clock.
var noon = time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

// asked is a promote row the tool wrote because it needed a person.
func asked(path string, at time.Time) audit.Row {
	return audit.Row{
		At: at, Op: audit.OpPromote, Actor: "human:priya",
		Paths: []string{path}, Outcome: string(gnosis.StatusBlocked),
		Detail: "a person must carry the signals that could not run",
	}
}

// answered is a promote row for a promotion that landed.
func answered(path string, at time.Time) audit.Row {
	return audit.Row{
		At: at, Op: audit.OpPromote, Actor: "human:priya",
		Paths: []string{path}, Outcome: string(gnosis.StatusOK),
	}
}

// dropped is a discard row: the other way of answering.
func dropped(path string, at time.Time) audit.Row {
	return audit.Row{
		At: at, Op: audit.OpDiscard, Actor: "human:priya",
		Paths: []string{path}, Outcome: string(gnosis.StatusOK),
	}
}

// TestOutstandingIsASubtraction walks each term of the definition, because each one
// removes a different way of reporting a decision that was already taken — and a
// report of settled decisions is one nobody reads twice.
func TestOutstandingIsASubtraction(t *testing.T) {
	t.Parallel()

	const path = "c/01932b7c-cache.md"
	for name, tc := range map[string]struct {
		rows   []audit.Row
		drafts []string
		want   bool
	}{
		"asked and never answered": {
			rows: []audit.Row{asked(path, noon)}, drafts: []string{path}, want: true,
		},
		// Both of these keep the draft present on purpose. Written with drafts nil
		// they passed for the wrong reason — the absent draft alone would have
		// cleared them — so the successful promotion and the discard were not being
		// tested at all. A case that passes without exercising its own subject is
		// the fixture mistake this file exists to avoid.
		"asked, then promoted": {
			rows:   []audit.Row{asked(path, noon), answered(path, noon.Add(time.Hour))},
			drafts: []string{path}, want: false,
		},
		"asked, then discarded": {
			rows:   []audit.Row{asked(path, noon), dropped(path, noon.Add(time.Hour))},
			drafts: []string{path}, want: false,
		},
		// The trail says a decision is outstanding and the draft is gone — deleted
		// by hand, which leaves no row. Citing a file that is not there would send
		// the reader to look for it.
		"asked, and the draft vanished": {
			rows: []audit.Row{asked(path, noon)}, drafts: nil, want: false,
		},
		// Promoted, then quarantined again and asked again: outstanding, because the
		// second ask is a second decision and the first answer does not cover it.
		"answered, then asked again": {
			rows: []audit.Row{
				asked(path, noon),
				answered(path, noon.Add(time.Hour)),
				asked(path, noon.Add(2*time.Hour)),
			},
			drafts: []string{path}, want: true,
		},
		// A draft nobody ever ran promote against. Unexamined, not abandoned:
		// `quarantine` lists it, and folding the two together would report a fresh
		// corpus as a pile of neglected decisions on its first day.
		"never asked at all": {
			rows: nil, drafts: []string{path}, want: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := bundle.Outstanding(&bundle.Trail{Rows: tc.rows}, tc.drafts)
			outstanding := len(got.Abandoned) > 0
			if outstanding != tc.want {
				t.Fatalf("outstanding = %v (%+v), want %v",
					outstanding, got.Abandoned, tc.want)
			}
		})
	}
}

// TestOneDecisionHoweverManyAttempts keeps the list measuring uncertainty rather
// than effort.
//
// Somebody who tried three times and put it down three times has one outstanding
// decision. Reporting three would make the list longer where the corpus got no more
// uncertain — and the attempt count is still there, because "three people have
// already declined to sign this" is the part worth knowing.
func TestOneDecisionHoweverManyAttempts(t *testing.T) {
	t.Parallel()

	const path = "c/01932b7c-cache.md"
	got := bundle.Outstanding(&bundle.Trail{Rows: []audit.Row{
		asked(path, noon),
		asked(path, noon.Add(time.Hour)),
		asked(path, noon.Add(2*time.Hour)),
	}}, []string{path})

	if len(got.Abandoned) != 1 {
		t.Fatalf("want one abandoned decision, got %+v", got.Abandoned)
	}
	a := got.Abandoned[0]
	if a.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", a.Attempts)
	}
	// The *last* ask, because that is when the clock a reader cares about starts.
	// The first would make an abandoned document look older every time somebody
	// picked it up again.
	if !a.Asked.Equal(noon.Add(2 * time.Hour)) {
		t.Errorf("asked = %v, want the most recent ask", a.Asked)
	}
	if a.Reason == "" {
		t.Error("the entry does not say what was needed")
	}
}

// TestTheDenominatorAndTheFloorAreCarried is §17's rule applied to this report: a
// count presented as health is forbidden, and a count from a damaged trail is a
// floor rather than a total.
func TestTheDenominatorAndTheFloorAreCarried(t *testing.T) {
	t.Parallel()

	const path = "c/01932b7c-cache.md"
	drafts := []string{path, "c/01932b7d-queue.md", "c/01932b7e-index.md"}

	whole := bundle.Outstanding(&bundle.Trail{Rows: []audit.Row{asked(path, noon)}}, drafts)
	if whole.Drafts != len(drafts) {
		t.Errorf("drafts = %d, want %d", whole.Drafts, len(drafts))
	}
	if !whole.Complete() {
		t.Error("an intact trail reported an incomplete count")
	}

	damaged := bundle.Outstanding(&bundle.Trail{
		Rows:      []audit.Row{asked(path, noon)},
		Malformed: []int{4},
	}, drafts)
	if damaged.Complete() {
		t.Error("a damaged trail reported a total rather than a floor")
	}
}

// TestNoDecisionsOutstandingIsEmptyNotNil keeps the empty case from being ambiguous
// in the envelope: `[]` says the report ran and found nothing, `null` says nothing at
// all.
func TestNoDecisionsOutstandingIsEmptyNotNil(t *testing.T) {
	t.Parallel()

	got := bundle.Outstanding(&bundle.Trail{}, nil)
	if got.Abandoned == nil {
		t.Error("an empty report is nil rather than empty")
	}
}

// TestEveryAbandonedDecisionIsListedInOrder is the case the fixtures were missing,
// and the linter is what pointed at it: every helper here had only ever been handed
// one path, so nothing established that a second abandoned decision appears at all.
//
// The order is asserted because a map has none. A report that reordered itself
// between runs could not be diffed, which is the property every other sorted output
// in this package exists for.
func TestEveryAbandonedDecisionIsListedInOrder(t *testing.T) {
	t.Parallel()

	const (
		cache = "c/01932b7c-cache.md"
		queue = "c/01932b7d-queue.md"
	)
	got := bundle.Outstanding(&bundle.Trail{Rows: []audit.Row{
		// Written newest-first, so the sort is doing the work rather than the input.
		asked(queue, noon.Add(time.Hour)),
		asked(cache, noon),
		// A third path that was answered, to keep the two above from passing
		// merely because everything present is reported.
		answered("c/01932b7e-index.md", noon),
	}}, []string{cache, queue, "c/01932b7e-index.md"})

	if len(got.Abandoned) != 2 {
		t.Fatalf("want two abandoned decisions, got %+v", got.Abandoned)
	}
	if got.Abandoned[0].Path != cache || got.Abandoned[1].Path != queue {
		t.Errorf("out of order: %s then %s",
			got.Abandoned[0].Path, got.Abandoned[1].Path)
	}
}
