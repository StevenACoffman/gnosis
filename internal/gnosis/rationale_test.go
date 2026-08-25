package gnosis_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// asked is what the tool puts in front of somebody carrying a promotion — the
// sentences a rationale is most likely to be a copy of, because they are on screen
// when it is typed.
var asked = []string{
	"state why you are promoting a candidate the gate could not fully check",
	"the approver must be a person (human:<id>); this promotion cannot be " +
		"self-granted by an agent",
	"a rationale, as --rationale",
}

// TestTheTemplateItselfIsRefused is the failure §10.6.4 says the bet loses to, and
// it is an observed one: a surveyed system had to warn its own agents in prose that
// they were emitting its template verbatim into the required field.
func TestTheTemplateItselfIsRefused(t *testing.T) {
	t.Parallel()

	for name, rationale := range map[string]string{
		"verbatim": "state why you are promoting a candidate the gate could not " +
			"fully check",
		// The workaround for an equality check, which is why the match is
		// containment. §10.6.4 names it in its own argument for quoting the match.
		"with a word added": "OK: state why you are promoting a candidate the " +
			"gate could not fully check.",
		// Re-wrapped and typographically mangled: the fold is what defeats this,
		// and it is the same fold every other guard in the family uses.
		"re-wrapped and curly": "State why you are promoting a candidate\nthe " +
			"gate could not fully check",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			why := gnosis.UnusableRationale(rationale, asked, nil)
			if why == "" {
				t.Fatalf("accepted the tool's own words: %q", rationale)
			}
			// The match is quoted, because a refusal that does not show what it
			// matched is one somebody works around by adding a word.
			if !strings.Contains(why, "promoting a candidate") {
				t.Errorf("the refusal does not show what matched: %s", why)
			}
		})
	}
}

// TestAShortPhraseCannotCondemnARationale is the false-positive direction, and it is
// the one that would do the most damage: a check that refuses real reasoning teaches
// the author to reword rather than to think, which is the opposite of §10.6.4's
// purpose.
func TestAShortPhraseCannotCondemnARationale(t *testing.T) {
	t.Parallel()

	// "a rationale, as --rationale" is four words, under minTemplateWords, so it is
	// not evidence however plainly it appears.
	rationale := "I am supplying a rationale, as --rationale asks, because the " +
		"vendor's published limit is the better source here."
	if why := gnosis.UnusableRationale(rationale, asked, nil); why != "" {
		t.Errorf("a four-word fragment condemned real reasoning: %s", why)
	}
}

// TestARepeatedRationaleNamesTheEarlierOne is the second refusal, and the naming is
// the point rather than a nicety: two decisions may rest on the same reason, and the
// honest way to say so is a reference to the first, which this makes the easy path.
func TestARepeatedRationaleNamesTheEarlierOne(t *testing.T) {
	t.Parallel()

	prior := []gnosis.PriorRationale{{
		Label: "human:sarah on 2026-08-20",
		Text: "Both sources are credible; chose the vendor's published limit over " +
			"the 2024 post because the post predates the rewrite.",
	}}

	// Folded-equal rather than byte-equal: re-wrapped, with a straightened quote.
	repeat := "Both sources are credible; chose the vendor’s published limit\n" +
		"over the 2024 post because the post predates the rewrite."
	why := gnosis.UnusableRationale(repeat, asked, prior)
	if why == "" {
		t.Fatal("accepted a copy of a rationale already on the record")
	}
	if !strings.Contains(why, "human:sarah on 2026-08-20") {
		t.Errorf("the refusal does not name the earlier decision: %s", why)
	}
}

// TestQuotingAnEarlierRationaleIsAllowed is why the prior check is equality and not
// containment.
//
// A rationale that quotes the earlier reasoning and then says why this case differs
// is the most useful thing an author can write, and a containment check would refuse
// exactly it — punishing the author who did the extra work.
func TestQuotingAnEarlierRationaleIsAllowed(t *testing.T) {
	t.Parallel()

	prior := []gnosis.PriorRationale{{
		Label: "human:sarah on 2026-08-20",
		Text:  "Chose the vendor's published limit over the 2024 post.",
	}}
	extended := "Chose the vendor's published limit over the 2024 post. Unlike " +
		"that case this one is per-connection, so the limit applies twice."

	if why := gnosis.UnusableRationale(extended, asked, prior); why != "" {
		t.Errorf("refused a rationale that built on the earlier one: %s", why)
	}
}

// TestAnEmptyRationaleIsSomebodyElsesRefusal keeps two checks from reporting one
// problem twice.
//
// Presence is already named where the other authorisation requirements are, and a
// second message about the same emptiness would make one mistake read as two.
func TestAnEmptyRationaleIsSomebodyElsesRefusal(t *testing.T) {
	t.Parallel()

	for name, rationale := range map[string]string{
		"empty":      "",
		"whitespace": "   \n\t ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if why := gnosis.UnusableRationale(rationale, asked, nil); why != "" {
				t.Errorf("reported emptiness as reuse: %s", why)
			}
		})
	}
}

// TestRealReasoningPasses is the calibration case, and a check with no passing case
// is a check nobody can trust.
func TestRealReasoningPasses(t *testing.T) {
	t.Parallel()

	prior := []gnosis.PriorRationale{{
		Label: "human:sarah on 2026-08-20",
		Text:  "Chose the vendor's published limit over the 2024 post.",
	}}
	for _, rationale := range []string{
		"The conflict check is not built yet and this claim has one source, so " +
			"there is nothing for it to conflict with.",
		"Needed for the incident writeup today; the missing signal is duplicate " +
			"detection and I have checked by hand.",
	} {
		if why := gnosis.UnusableRationale(rationale, asked, prior); why != "" {
			t.Errorf("refused %q: %s", rationale, why)
		}
	}
}
