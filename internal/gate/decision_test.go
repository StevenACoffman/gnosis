package gate_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gate"
)

// report builds a Report with a held control and the given verdicts.
func report(verdicts ...gate.Verdict) gate.Report {
	r := gate.Report{Path: "c/a.md", Control: gate.ControlReport{Held: true}}
	for i, v := range verdicts {
		r.Results = append(r.Results, gate.Result{
			Signal: gate.Signal("s" + string(rune('a'+i))), Verdict: v,
		})
	}
	return r
}

func TestDecide(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		report gate.Report
		want   gate.Decision
	}{
		"every signal passed": {
			report(gate.VerdictPass, gate.VerdictPass), gate.DecisionApproved,
		},
		"one signal could not run": {
			report(gate.VerdictPass, gate.VerdictUnchecked), gate.DecisionNeedsHuman,
		},
		"one signal failed": {
			report(gate.VerdictPass, gate.VerdictFail), gate.DecisionRefused,
		},
		// The precedence that matters. Offering somebody a signature over a
		// document with a known defect is worse than refusing both.
		"failed and unchecked together": {
			report(gate.VerdictFail, gate.VerdictUnchecked), gate.DecisionRefused,
		},
		"the control did not hold": {
			gate.Report{
				Control: gate.ControlReport{Held: false},
				Results: []gate.Result{{Verdict: gate.VerdictPass}},
			},
			gate.DecisionUnavailable,
		},
		"no signals ran at all": {
			gate.Report{Control: gate.ControlReport{Held: true}}, gate.DecisionUnavailable,
		},
		"the zero report": {gate.Report{}, gate.DecisionUnavailable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.report.Decide(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNoDecisionButNeedsHumanIsPromotableByAPerson is the test that keeps this
// design from being a `--force` in different clothing. §15 forbids a bypass; the
// human path is only defensible if it opens for what could not be checked and
// stays shut for what was checked and failed.
func TestNoDecisionButNeedsHumanIsPromotableByAPerson(t *testing.T) {
	t.Parallel()
	all := []gate.Decision{
		gate.DecisionUnavailable, gate.DecisionApproved,
		gate.DecisionNeedsHuman, gate.DecisionRefused,
	}
	for _, d := range all {
		want := d == gate.DecisionNeedsHuman
		if got := d.Promotable(); got != want {
			t.Errorf("%q.Promotable() = %v, want %v", d, got, want)
		}
	}
}

// TestApprovedMeansNoHumanIsNeeded. Approved is the narrower claim and the two
// must not drift: a candidate a person has to sign for is not one that passed.
func TestApprovedMeansNoHumanIsNeeded(t *testing.T) {
	t.Parallel()
	escalated := report(gate.VerdictPass, gate.VerdictUnchecked)
	if escalated.Approved() {
		t.Error("a candidate with an unchecked signal reported as approved")
	}
	clean := report(gate.VerdictPass)
	if !clean.Approved() {
		t.Error("a candidate whose every signal passed was not approved")
	}
}

// TestTheZeroDecisionAuthorisesNothing. The discipline every enum here follows,
// asserted rather than assumed: a Decision nobody populated must not let anything
// through.
func TestTheZeroDecisionAuthorisesNothing(t *testing.T) {
	t.Parallel()
	var d gate.Decision
	if d != gate.DecisionUnavailable {
		t.Fatalf("the zero decision is %q", d)
	}
	if d.Promotable() {
		t.Error("the zero decision is promotable")
	}
}

// TestADecisionMarshalsAsAWord. An agent branching on the integer 0 would be
// branching on declaration order.
func TestADecisionMarshalsAsAWord(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(gate.DecisionNeedsHuman)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(b); got != `"needs_human"` {
		t.Errorf("got %s, want \"needs_human\"", got)
	}
}

// TestAnOutOfRangeDecisionIsNotPromotable guards the direction a mistake should
// fail in.
func TestAnOutOfRangeDecisionIsNotPromotable(t *testing.T) {
	t.Parallel()
	d := gate.Decision(99)
	if d.Promotable() {
		t.Error("an out-of-range decision is promotable")
	}
	if !strings.Contains(d.String(), "invalid") {
		t.Errorf("an out-of-range decision renders as %q", d)
	}
}
