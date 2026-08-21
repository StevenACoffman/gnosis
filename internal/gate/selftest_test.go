package gate_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gate"
)

// TestControlHolds is the claim §9.5 makes: every implemented signal can be shown
// to reject a planted defect and accept a control. A gate nobody has proven can
// fail is not a gate.
func TestControlHolds(t *testing.T) {
	t.Parallel()
	got := gate.SelfTest()
	if !got.Held {
		t.Fatalf("the control failed for %v", got.Broken)
	}
	if len(got.Broken) != 0 {
		t.Errorf("Held is true but Broken is %v", got.Broken)
	}
}

// TestUnprovenSignalsAreNamed. A self-test that quietly skipped the two signals
// with no implementation would report "held" over a battery that never exercised
// two of seven checks — the same silence VerdictUnchecked exists to break.
func TestUnprovenSignalsAreNamed(t *testing.T) {
	t.Parallel()
	got := gate.SelfTest()
	if len(got.Unproven) != 2 {
		t.Fatalf("Unproven = %v, want the two unbuilt signals", got.Unproven)
	}
	if got.Unproven[0] != gate.SignalConflict || got.Unproven[1] != gate.SignalSecurity {
		t.Errorf("Unproven = %v, want [conflict security]", got.Unproven)
	}
}

// TestEveryImplementedSignalHasAControl derives the expectation from the report
// rather than restating a list: a signal implemented later without a planted
// defect must show up as unproven, and this fails if the two sets ever disagree
// with what the gate actually evaluates.
func TestEveryImplementedSignalHasAControl(t *testing.T) {
	t.Parallel()
	control := gate.SelfTest()
	unproven := map[gate.Signal]bool{}
	for _, s := range control.Unproven {
		unproven[s] = true
	}

	report := evaluate(admissiblePtr())
	for _, res := range report.Results {
		// A signal that produced a real verdict must have been proven able to
		// produce the other one.
		if res.Verdict != gate.VerdictUnchecked && unproven[res.Signal] {
			t.Errorf("%s returned %v but has no planted defect", res.Signal, res.Verdict)
		}
		if res.Verdict == gate.VerdictUnchecked && !unproven[res.Signal] {
			t.Errorf("%s is unchecked but claims to have a control", res.Signal)
		}
	}
}

// TestSelfTestIsPure, so a report is reproducible and the control cannot pass on
// one invocation and fail on the next for reasons nobody can see.
func TestSelfTestIsPure(t *testing.T) {
	t.Parallel()
	first := gate.SelfTest()
	for range 20 {
		again := gate.SelfTest()
		if again.Held != first.Held || len(again.Broken) != len(first.Broken) ||
			len(again.Unproven) != len(first.Unproven) {
			t.Fatalf("two self-tests differ:\n%+v\n%+v", first, again)
		}
	}
}
