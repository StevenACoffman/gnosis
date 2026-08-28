package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// pinned builds one claim carrying a pin, with the text the shell would have normalised.
func pinned(anchor string, value float64, prose string) *lint.Snapshot {
	return &lint.Snapshot{
		Bounds: map[string]*lint.Bound{
			"c1": {
				SubjectKey: "retry.max_attempts", Dimension: "count",
				Op: "<=", Value: value, Raw: "pin",
				Pinned: true, ProseText: prose,
			},
		},
		Documents: []lint.Document{{
			Path:   "c/a.md",
			Claims: []lint.Claim{{ID: "c1", Anchor: anchor}},
		}},
	}
}

// TestAPinThatContradictsItsProseIsReported is §10.2.1's one drift case: the pin outranks
// the prose, so a reader gets five where the text says three.
func TestAPinThatContradictsItsProseIsReported(t *testing.T) {
	t.Parallel()
	snap := pinned("Retries must be no more than three.", 5,
		"Retries must be no more than 3 .")
	got := runNamed(t, snap, "constraint-drift")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	// The claim's own words, so a reader can see the disagreement without opening it.
	if !strings.Contains(got[0], "no more than three") {
		t.Errorf("the finding does not quote the prose:\n%s", got[0])
	}
	// And the limit, because silence here is not proof of agreement.
	if !strings.Contains(got[0], "paraphrase") {
		t.Errorf("the finding does not say what it cannot see:\n%s", got[0])
	}
}

// TestASpelledOutNumberSatisfiesItsPin is §7.3's normalisation doing its job. A pin of 3
// against "no more than three" is the *agreeing* case, and reporting it would fire on
// almost every pin somebody bothered to write.
func TestASpelledOutNumberSatisfiesItsPin(t *testing.T) {
	t.Parallel()
	snap := pinned("Retries must be no more than three.", 3,
		"Retries must be no more than 3 .")
	if got := runNamed(t, snap, "constraint-drift"); len(got) != 0 {
		t.Errorf("a pin matching its spelled-out prose was reported:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestAValueInsideALargerNumberIsNotAMatch is the adversarial case, and it fails toward
// noise rather than silence — which is the safe direction here, but only if it is
// deliberate. A pin of 3 must not be satisfied by "30" or "1.3", because a substring
// match would quietly certify a pin against a number nobody wrote.
func TestAValueInsideALargerNumberIsNotAMatch(t *testing.T) {
	t.Parallel()
	for _, prose := range []string{
		"Retries must be no more than 30 .",
		"Latency must be under 1.3 seconds .",
	} {
		snap := pinned("irrelevant", 3, prose)
		if got := runNamed(t, snap, "constraint-drift"); len(got) != 1 {
			t.Errorf("%q satisfied a pin of 3: %d finding(s)", prose, len(got))
		}
	}
}

// TestAUnitAgainstTheNumberStillMatches keeps this check from disagreeing with the parser
// one package over. "400ms" is how a latency budget is written, and the constraint parser
// was already taught to read it — a drift check that could not see it would report every
// duration pin in a corpus.
func TestAUnitAgainstTheNumberStillMatches(t *testing.T) {
	t.Parallel()
	snap := pinned("The timeout must be under 400ms.", 400,
		"The timeout must be under 400ms.")
	if got := runNamed(t, snap, "constraint-drift"); len(got) != 0 {
		t.Errorf("a unit written against its number defeated the match:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestAnUnpinnedCorpusSkipsRatherThanPasses is §12's applicability rule. A derived reading
// cannot drift from the prose it was read out of, so there is nothing here to check — and
// a check that passed silently would be indistinguishable from one that found nothing.
func TestAnUnpinnedCorpusSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{
		Bounds: map[string]*lint.Bound{
			"c1": {SubjectKey: "retry.max_attempts", Op: "<=", Value: 3, Raw: "3"},
		},
		Documents: []lint.Document{{
			Path:   "c/a.md",
			Claims: []lint.Claim{{ID: "c1", Anchor: "Retries must be no more than three."}},
		}},
	}
	reason := skipReason(t, snap, "constraint-drift")
	if !strings.Contains(reason, "gnosis_constraint") {
		t.Errorf("constraint-drift skipped for %q, which does not name what is absent",
			reason)
	}
}
