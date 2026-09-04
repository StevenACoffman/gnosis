package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestOnlyOneMissReasonIsActionable is the distinction the whole log turns on.
//
// §6.4's payoff is that "a reason that recurs is a deterministic check waiting to be
// written". That is true of one of these reasons and false of the other: extraction has
// no deterministic alternative and never will, so its rows recur forever and name no
// work. A report that could not tell them apart would be dominated by the line nobody
// can act on.
func TestOnlyOneMissReasonIsActionable(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		reason     gnosis.MissReason
		actionable bool
	}{
		"a path exists and declined": {gnosis.MissNoPredicate, true},
		"no path exists":             {gnosis.MissNoPath, false},
		"unset":                      {gnosis.MissReasonUnset, false},
		"a reason a later gnosis wrote": {
			gnosis.MissReason("something_else"), false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tc.reason.Actionable(); got != tc.actionable {
				t.Errorf("%v.Actionable() = %v, want %v", tc.reason, got, tc.actionable)
			}
		})
	}
}

// TestParseMissReasonRefusesWhatItDoesNotKnow. A row written by a later gnosis must be
// reported as unrecognised rather than folded into one of these two: the count is the
// output, and a miscounted row is a wrong answer where a missing one is only a gap.
func TestParseMissReasonRefusesWhatItDoesNotKnow(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "  ", "no_deterministic_selector", "unset"} {
		if got, ok := gnosis.ParseMissReason(raw); ok {
			t.Errorf("ParseMissReason(%q) = (%v, true), want refused", raw, got)
		}
	}
	for _, raw := range []string{"no_deterministic_path", "  NO_DETERMINISTIC_PATH  "} {
		if got, ok := gnosis.ParseMissReason(raw); !ok || got != gnosis.MissNoPath {
			t.Errorf("ParseMissReason(%q) = (%v, %v)", raw, got, ok)
		}
	}
}

// TestTheZeroMissReasonNamesNothing, so a row nobody characterised cannot swell whichever
// group it defaulted into.
func TestTheZeroMissReasonNamesNothing(t *testing.T) {
	t.Parallel()

	var r gnosis.MissReason
	if r != gnosis.MissReasonUnset || r.String() != "unset" || r.Actionable() {
		t.Errorf("the zero MissReason is %q, actionable=%v", r.String(), r.Actionable())
	}
	text, err := gnosis.MissNoPredicate.MarshalText()
	if err != nil || string(text) != "no_deterministic_predicate" {
		t.Errorf("MarshalText = %q, %v", text, err)
	}
}
