package gnosis_test

import (
	"testing"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

func TestFreshnessOf(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var never time.Time
	yesterday := now.AddDate(0, 0, -1)
	lastWeek := now.AddDate(0, 0, -7)
	nextYear := now.AddDate(1, 0, 0)

	cases := map[string]struct {
		checkedAt   time.Time
		staleAfter  time.Time
		hasUpstream bool
		want        gnosis.Freshness
	}{
		// The case the four-state vocabulary exists for. Never looked is not fine.
		"never checked":                {never, never, true, gnosis.FreshnessUnknown},
		"never checked, future expiry": {never, nextYear, true, gnosis.FreshnessUnknown},

		// No upstream is not staleness: reporting it as stale would suggest an
		// action nobody can take.
		"no upstream":              {never, never, false, gnosis.FreshnessNotApplicable},
		"no upstream, was checked": {yesterday, never, false, gnosis.FreshnessNotApplicable},

		// Checked and not expired.
		"checked, no expiry":    {yesterday, never, true, gnosis.FreshnessFresh},
		"checked, expiry ahead": {yesterday, nextYear, true, gnosis.FreshnessFresh},

		// The document declared its own expiry and it passed. This outranks having
		// been checked: a source can be verified unchanged and still be past the
		// date its author said to revisit it.
		"expired despite a check": {yesterday, lastWeek, true, gnosis.FreshnessStale},
		"expires exactly now":     {yesterday, now, true, gnosis.FreshnessStale},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := gnosis.FreshnessOf(now, tc.checkedAt, tc.staleAfter, tc.hasUpstream)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTheZeroFreshnessIsUnknown is the placement the whole type turns on. A zero
// value of Fresh would collapse "we never looked" into "it is fine" by default, on
// every document in a new corpus.
func TestTheZeroFreshnessIsUnknown(t *testing.T) {
	t.Parallel()
	var f gnosis.Freshness
	if f != gnosis.FreshnessUnknown {
		t.Errorf("the zero Freshness is %q, not unknown", f)
	}
	if f.Trustworthy() {
		t.Error("the zero Freshness is trustworthy")
	}
}

// TestOnlyFreshIsTrustworthy. NotApplicable is not — a source with no upstream is
// not thereby current, it is merely not checkable — and a caller wanting "nothing
// is wrong here" is asking a different question.
func TestOnlyFreshIsTrustworthy(t *testing.T) {
	t.Parallel()
	if !gnosis.FreshnessFresh.Trustworthy() {
		t.Error("fresh is not trustworthy")
	}
	for _, f := range []gnosis.Freshness{
		gnosis.FreshnessUnknown, gnosis.FreshnessStale, gnosis.FreshnessNotApplicable,
	} {
		if f.Trustworthy() {
			t.Errorf("%q is trustworthy", f)
		}
	}
}

// TestTheClockIsAParameter, so two runs at one instant agree and a test can pin
// the answer rather than assert a range.
func TestTheClockIsAParameter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	checked := now.AddDate(0, 0, -1)
	expiry := now.AddDate(0, 0, 1)

	first := gnosis.FreshnessOf(now, checked, expiry, true)
	for range 20 {
		if again := gnosis.FreshnessOf(now, checked, expiry, true); again != first {
			t.Fatalf("two calls differ: %q then %q", first, again)
		}
	}
	// And advancing the clock past the expiry changes the answer, or the
	// parameter is not being used.
	if got := gnosis.FreshnessOf(expiry, checked, expiry, true); got != gnosis.FreshnessStale {
		t.Errorf("at the expiry the answer is %q, want stale", got)
	}
}
