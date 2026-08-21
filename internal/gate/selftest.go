package gate

import "sort"

// ControlReport is the outcome of the planted-defect self-test.
//
// Held is false whenever any implemented signal failed to reject its planted
// defect or failed to accept its control. Evaluate refuses to gate in that state:
// a verdict from a check that cannot be shown to discriminate is not evidence of
// anything, and reporting one beside the control failure would invite acting on
// it.
type ControlReport struct {
	Held bool `json:"held"`

	// Broken names the signals whose control did not hold, sorted.
	Broken []Signal `json:"broken,omitempty"`

	// Unproven names the signals with no planted defect, sorted. These are
	// exactly the signals with no implementation, and they are reported rather
	// than omitted for the same reason VerdictUnchecked exists: a self-test that
	// silently skipped them would report "held" over a battery that never
	// exercised two of seven checks.
	Unproven []Signal `json:"unproven,omitempty"`
}

// SelfTest runs the planted-defect battery.
//
// Requires: nothing.
// Ensures: pure, allocation-only, no I/O — every fixture is a literal here, which
// is what makes running it on **every** invocation affordable. A gate proven at
// build time is not proven in the binary somebody is actually running, and the
// difference matters precisely when a build is misconfigured or a dependency has
// been swapped underneath it.
//
// Each implemented signal supplies two fixtures: a defect it MUST reject and a
// control it MUST accept. Requiring both is the point — a signal hard-wired to
// fail would catch every defect and is not a check, and one hard-wired to pass
// would accept every control and is not either.
func SelfTest() ControlReport {
	report := ControlReport{Held: true}

	battery := controls()
	for i := range battery {
		c := &battery[i]
		defectVerdict := c.run(&c.defect)
		controlVerdict := c.run(&c.control)
		if defectVerdict != VerdictFail || controlVerdict != VerdictPass {
			report.Held = false
			report.Broken = append(report.Broken, c.signal)
		}
	}
	report.Unproven = unprovenSignals()
	sortSignals(report.Broken)
	return report
}

// unprovenSignals is every signal with no entry in the battery.
//
// Derived by difference rather than listed, so a signal implemented later without
// a control is reported as unproven instead of quietly counting as proven.
func unprovenSignals() []Signal {
	battery := controls()
	proven := make(map[Signal]bool, len(battery))
	for i := range battery {
		proven[battery[i].signal] = true
	}
	var out []Signal
	for _, s := range []Signal{
		SignalEvidence, SignalProvenance, SignalConformance,
		SignalDuplication, SignalHedging, SignalConflict, SignalSecurity,
	} {
		if !proven[s] {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
