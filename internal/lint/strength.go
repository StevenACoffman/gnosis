package lint

import (
	"strings"

	"github.com/StevenACoffman/skillet/textnorm"
)

// Indicators is the closed lexical class of §9.4.1, as the checks compare against it.
//
// A value the shell gathers, like Vocabulary and Strengths. The two roles have different
// readers and are carried separately for that reason: the reason words gate segmentation
// before a claim is ever written, and the conclusion words are read by §17.4's lead check
// after one is.
type Indicators struct {
	// Reason are the words that introduce a reason, longest first.
	Reason []string

	// Conclusion are the words that introduce a conclusion, longest first. They had no
	// reader between shipping and §17.4's check, which is recorded in the file itself.
	Conclusion []string
}

// Strengths is the claim-strength markers a check compares against (§17.3.1).
//
// A value the shell gathers, like Vocabulary, so this package keeps its single
// internal import and a check can be handed exactly the two lists it needs.
type Strengths struct {
	// Universal are the markers that assert without exception, longest first.
	Universal []string

	// Hedged are the markers that assert with one, longest first.
	//
	// Carried so the check can tell a claim that *hedged* from one that said nothing
	// either way. Those are different states and only the second is silent: a claim
	// with one quotation and the word "typically" has already done what the finding
	// would ask of it.
	Hedged []string
}

// Registers is the causal-register class of §17.3.1.1, as the check compares against it.
//
// A value the shell gathers, like Strengths, so this package keeps its single internal
// import. The two roles are carried separately because the check needs to tell three
// states apart and not two: a statement that asserted intervention, one that asserted
// association, and one that said nothing about causation at all. Only the third is
// silent, and collapsing it into either of the others would make the check assert from
// absence.
type Registers struct {
	// Intervention are the markers asserting that something makes something else
	// happen, longest first.
	Intervention []string

	// Association are the markers asserting that two things move together, longest
	// first — what observational evidence can actually support.
	Association []string
}

// Declared reports whether any markers were loaded.
//
// Separate from len() on either list so an absent or unreadable standards file skips
// the check with a reason rather than passing every claim — the absence of the ruler
// reported as a clean measurement.
func (s *Strengths) Declared() bool { return len(s.Universal) > 0 || len(s.Hedged) > 0 }

// Asserts reports the strongest universal marker in text, if any.
//
// Requires: markers are lower-cased.
// Ensures: the longest match, so "must not" is found before "must". Matching is on word
// boundaries under fold, so a reflowed claim still matches and "allow" does not match
// "all". Pure.
func (s *Strengths) Asserts(text string) (string, bool) {
	return firstMarker(text, s.Universal)
}

// Hedges reports whether text carries a hedge.
func (s *Strengths) Hedges(text string) bool {
	_, ok := firstMarker(text, s.Hedged)
	return ok
}

// firstMarker finds the first of markers present in text as a whole word.
//
// Markers arrive longest-first, so the first hit is the longest — which is why
// "must not" is not reported as "must" with the negation dropped.
func firstMarker(text string, markers []string) (string, bool) {
	folded := " " + strings.ToLower(textnorm.Fold(text)) + " "
	for _, m := range markers {
		if strings.Contains(folded, " "+m+" ") ||
			strings.Contains(folded, " "+m+",") ||
			strings.Contains(folded, " "+m+".") {
			return m, true
		}
	}
	return "", false
}

// Declared reports whether any registers were loaded.
//
// Separate from len() on either list, for the reason Strengths.Declared gives: an absent
// standards file must skip the check with a reason rather than pass every claim, which
// would report the absence of the ruler as a clean measurement.
func (r *Registers) Declared() bool {
	return len(r.Intervention) > 0 || len(r.Association) > 0
}

// Rung reports which rung text sits on, and whether it says anything at all.
//
// Requires: markers are lower-cased.
// Ensures: intervention wins a tie, and the reason is the asymmetry the whole check is
// about. "Restarting the pod is associated with fewer leaks and causes the counter to
// reset" asserts intervention somewhere in it, and a reading that returned association
// would let one hedged clause launder a causal claim. Pure.
func (r *Registers) Rung(text string) (rung, marker string, stated bool) {
	if m, ok := firstMarker(text, r.Intervention); ok {
		return "intervention", m, true
	}
	if m, ok := firstMarker(text, r.Association); ok {
		return "association", m, true
	}
	return "", "", false
}
