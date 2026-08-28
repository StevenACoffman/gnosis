package scan

// The four stages SPEC §9.3 specifies, in the order it lists them.
const (
	StageHidden    = "hidden-characters"
	StageInjection = "injection-patterns"
	StageSecrets   = "secrets"
	StageOversize  = "oversize"
)

// Coverage is which of §9.3's stages a scan actually performed.
//
// It exists because "no findings" and "§9.3 satisfied" are different claims and
// only one of them is available to a caller reading a finding list. A caller that
// cannot tell them apart will eventually assert the second on the strength of the
// first — which is the specific failure a security stage cannot be allowed to have,
// since the content being gated arrived from outside.
type Coverage struct {
	// Ran names the stages performed, in §9.3 order.
	Ran []string `json:"ran"`

	// Missing names the stages specified and not performed, in §9.3 order. Empty
	// only when the scan was complete.
	Missing []string `json:"missing,omitempty"`
}

// Complete reports whether every specified stage ran.
//
// Requires: nothing; the zero Coverage is not complete, which is the safe
// reading of a value nobody populated.
// Ensures: true only when Ran covers §9.3 and Missing is empty.
func (c Coverage) Complete() bool { return len(c.Missing) == 0 && len(c.Ran) > 0 }

// Stages are §9.3's four stage names, in the order that section lists them.
//
// Requires: nothing.
// Ensures: a fresh slice per call, so a caller cannot reorder the canonical list for
// the next one. Pure.
func Stages() []string {
	return []string{StageHidden, StageInjection, StageSecrets, StageOversize}
}

// CoverageOf reports the coverage of a scan that performed exactly these stages.
//
// Requires: ran names stages the caller **actually performed**. That is the one
// thing this function cannot check and the reason it is called from a single place
// (`bundle.scanCandidate`), where the stages and the code performing them sit
// together and a test asserts the pairing.
// Ensures: Ran and Missing partition §9.3's stage list, both in §9.3 order, so two
// scans of one text are comparable. Pure.
//
// A name that is not one of §9.3's stages does not reduce Missing and does not
// appear in Ran. **A typo therefore makes coverage look worse, never better**, which
// is the direction a coverage report has to fail in — and is why this needs no error
// return for a caller that mistypes a constant.
//
// This replaced two functions that each answered part of the question — one on
// Ruleset and one for the no-ruleset case. Composing from what was performed is
// better than either, because the previous pair could only describe the
// configurations they were written for: neither could say that stage 4 ran, which is
// what made §9.3's last gap invisible from inside the type that reported on it.
func CoverageOf(ran ...string) Coverage {
	performed := make(map[string]bool, len(ran))
	for _, stage := range ran {
		performed[stage] = true
	}

	out := Coverage{Ran: make([]string, 0, len(ran))}
	for _, stage := range Stages() {
		if performed[stage] {
			out.Ran = append(out.Ran, stage)
			continue
		}
		out.Missing = append(out.Missing, stage)
	}
	return out
}
