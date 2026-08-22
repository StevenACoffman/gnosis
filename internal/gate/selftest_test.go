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

// TestUnprovenSignalsAreNamed. A self-test that quietly skipped a signal with no
// implementation would report "held" over a battery that never exercised it — the
// same silence VerdictUnchecked exists to break.
func TestUnprovenSignalsAreNamed(t *testing.T) {
	t.Parallel()
	got := gate.SelfTest()
	if len(got.Unproven) != 1 || got.Unproven[0] != gate.SignalConflict {
		t.Errorf("Unproven = %v, want only [conflict]", got.Unproven)
	}
}

// TestEveryImplementedSignalHasAControl derives the expectation from the report
// rather than restating a list: a signal implemented later without a planted
// defect must show up as unproven, and this fails if the two sets ever disagree
// with what the gate actually evaluates.
//
// It asserts the forward direction only, and the reverse used to hold. "Unchecked"
// and "unproven" coincided while every unchecked signal was an unbuilt one, and
// the security signal separated them: it is implemented, it has a planted defect
// it rejects, and it still reports Unchecked for a candidate whose scan did not
// run every §9.3 stage. Unproven is a fact about the *signal*; unchecked is a fact
// about *this candidate*. Asserting the old equivalence would now forbid exactly
// the state §9.3 is in.
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
	}
}

// TestAnUnprovenSignalNeverPasses. The forward assertion above is only worth
// something if an unproven signal cannot quietly approve: a check nobody has shown
// can fail must not be able to authorise a write.
func TestAnUnprovenSignalNeverPasses(t *testing.T) {
	t.Parallel()
	unproven := map[gate.Signal]bool{}
	for _, s := range gate.SelfTest().Unproven {
		unproven[s] = true
	}
	report := evaluate(admissiblePtr())
	for _, res := range report.Results {
		if unproven[res.Signal] && res.Verdict == gate.VerdictPass {
			t.Errorf("%s passed with no planted defect proving it can fail", res.Signal)
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
