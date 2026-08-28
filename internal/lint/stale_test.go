package lint_test

import (
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

func now() time.Time       { return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC) }
func days(n int) time.Time { return now().AddDate(0, 0, n) }

// staleSnap builds a corpus of one document with the given expiry and sources.
func staleSnap(
	after time.Time,
	keys []string,
	checks map[string]time.Time,
	window int,
) *lint.Snapshot {
	return &lint.Snapshot{
		Documents: []lint.Document{{
			Path: "c/a.md", StaleAfter: after, SourceKeys: keys,
		}},
		SourceChecks:  checks,
		StalenessDays: window,
	}
}

// staleOnly runs the stale check alone, so a failure names it rather than
// whatever else the registry reported.
func staleOnly(t *testing.T, snap *lint.Snapshot) lint.Report {
	t.Helper()
	for _, c := range lint.Checks(now()) {
		if c.Name == "stale" {
			return lint.Run(snap, []lint.Check{c})
		}
	}
	t.Fatal("no stale check is registered")
	return lint.Report{}
}

func TestStale(t *testing.T) {
	t.Parallel()
	const key = "evidence/text/aa/a.md"
	cases := map[string]struct {
		snap *lint.Snapshot
		want string // "" for silence, else a fragment of the message
	}{
		"past its declared date": {
			staleSnap(days(-1), nil, nil, 0), "stale_after",
		},
		"expires exactly today": {
			staleSnap(now(), nil, nil, 0), "stale_after",
		},
		"expires tomorrow": {
			staleSnap(days(1), nil, nil, 0), "",
		},
		// The author's date is a statement about the claim and does not become
		// less true because nobody checked the source.
		"expired even though checked today": {
			staleSnap(
				days(-1),
				[]string{key},
				map[string]time.Time{key: now()},
				180,
			),
			"stale_after",
		},
		"checked inside the window": {
			staleSnap(time.Time{}, []string{key}, map[string]time.Time{key: days(-10)}, 180), "",
		},
		"checked past the window": {
			staleSnap(time.Time{}, []string{key}, map[string]time.Time{key: days(-200)}, 180),
			"200 days ago",
		},
		// Never-checked is a state, not a finding: it is true of every document in
		// a corpus that has just started fetching, and a warning true of
		// everything teaches a reader to skip the category.
		"never checked": {
			staleSnap(time.Time{}, []string{key}, nil, 180), "",
		},
		"no window declared": {
			staleSnap(time.Time{}, []string{key}, map[string]time.Time{key: days(-900)}, 0), "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := staleOnly(t, tc.snap)
			if tc.want == "" {
				if len(got.Diagnostics) != 0 {
					t.Fatalf("wanted silence, got %+v", got.Diagnostics)
				}
				return
			}
			if len(got.Diagnostics) != 1 {
				t.Fatalf("got %d diagnostics, want 1: %+v", len(got.Diagnostics), got.Diagnostics)
			}
			if !strings.Contains(got.Diagnostics[0].Message, tc.want) {
				t.Errorf("message %q omits %q", got.Diagnostics[0].Message, tc.want)
			}
		})
	}
}

// TestADocumentIsOnlyAsVerifiedAsItsWeakestSource. Taking the newest check would
// let one re-fetch vouch for three sources nobody has looked at.
func TestADocumentIsOnlyAsVerifiedAsItsWeakestSource(t *testing.T) {
	t.Parallel()
	snap := staleSnap(time.Time{},
		[]string{"evidence/text/aa/a.md", "evidence/text/bb/b.md"},
		map[string]time.Time{
			"evidence/text/aa/a.md": now(),      // checked today
			"evidence/text/bb/b.md": days(-300), // and not for ten months
		}, 180)

	got := staleOnly(t, snap)
	if len(got.Diagnostics) != 1 {
		t.Fatalf("the recent check vouched for the old one: %+v", got.Diagnostics)
	}
	if !strings.Contains(got.Diagnostics[0].Message, "300 days") {
		t.Errorf("message reports the wrong source: %q", got.Diagnostics[0].Message)
	}
}

// TestTheCheckSkipsACorpusWithNothingToJudge. Derived applicability per §12:
// neither half can fire on a corpus that declares no expiry and has verified
// nothing, and saying so differs from reporting nothing found.
func TestTheCheckSkipsACorpusWithNothingToJudge(t *testing.T) {
	t.Parallel()
	got := staleOnly(t, &lint.Snapshot{Documents: []lint.Document{{Path: "c/a.md"}}})

	if len(got.Skipped) != 1 {
		t.Fatalf("the check ran on a corpus with nothing to judge: %+v", got)
	}
	if !strings.Contains(got.Skipped[0].Reason, "no source has been verified") {
		t.Errorf("skip reason = %q", got.Skipped[0].Reason)
	}
}

// TestFreshnessOfMatchesTheCheck. A rendered state and a reported finding come
// from one computation, so they cannot disagree — which they would if `show` said
// fresh while `lint` reported stale.
func TestFreshnessOfMatchesTheCheck(t *testing.T) {
	t.Parallel()
	const key = "evidence/text/aa/a.md"
	cases := map[string]struct {
		snap *lint.Snapshot
		want gnosis.Freshness
	}{
		"no sources":    {staleSnap(time.Time{}, nil, nil, 180), gnosis.FreshnessNotApplicable},
		"never checked": {staleSnap(time.Time{}, []string{key}, nil, 180), gnosis.FreshnessUnknown},
		"checked, live": {
			staleSnap(days(30), []string{key}, map[string]time.Time{key: days(-1)}, 180),
			gnosis.FreshnessFresh,
		},
		"expired": {
			staleSnap(days(-1), []string{key}, map[string]time.Time{key: days(-1)}, 180),
			gnosis.FreshnessStale,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := lint.FreshnessOf(now(), &tc.snap.Documents[0], tc.snap)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAnEpisodicTypeIsExemptFromTheWindowButNotFromItsAuthor is §5.8.3.1's derived
// behaviour, and the split matters more than the exemption.
//
// An episode's evidence is a commit hash, immutable by construction, so "re-run
// `gnosis fetch` on them" is advice nobody can act on and the finding never clears.
// But a declared stale_after is the author's own statement about their claim, and an
// author may legitimately ask for an episode to be revisited — exempting that too
// would silence a person rather than an impossible instruction.
func TestAnEpisodicTypeIsExemptFromTheWindowButNotFromItsAuthor(t *testing.T) {
	t.Parallel()

	vocabulary := lint.Vocabulary{
		Declared: true,
		Types: []lint.VocabType{
			{Key: "Episode", Episodic: true},
			{Key: "Rule"},
		},
	}
	long := now().AddDate(0, 0, -400)

	// The window half: exempt for Episode, reported for Rule.
	windowed := &lint.Snapshot{
		Vocabulary:    vocabulary,
		StalenessDays: 30,
		SourceChecks:  map[string]time.Time{"s1": long},
		Documents: []lint.Document{
			{Path: "c/ep.md", Type: "Episode", SourceKeys: []string{"s1"}},
			{Path: "c/rule.md", Type: "Rule", SourceKeys: []string{"s1"}},
		},
	}
	got := lint.StaleFindings(windowed, now())
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d: %+v", len(got), got)
	}
	if got[0].Path != "c/rule.md" {
		t.Errorf("the exemption applied to the wrong document: %s", got[0].Path)
	}

	// The declared half: reported for Episode too, because a person asked.
	asked := &lint.Snapshot{
		Vocabulary: vocabulary,
		Documents: []lint.Document{
			{Path: "c/ep.md", Type: "Episode", StaleAfter: now().AddDate(0, 0, -1)},
		},
	}
	if got := lint.StaleFindings(asked, now()); len(got) != 1 {
		t.Errorf("an author's own stale_after was silenced on an episodic type: %+v", got)
	}
}
