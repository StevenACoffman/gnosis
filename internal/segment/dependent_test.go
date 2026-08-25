package segment_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/segment"
	"github.com/StevenACoffman/gnosis/internal/standards"
)

// reasons is the shipped word list, so these cases test the data as well as the code.
// A word deleted from indicators.toml fails here, which is what makes the file a
// checked artifact rather than a suggestion.
func reasons(t *testing.T) []string {
	t.Helper()
	in, err := standards.LoadIndicators(standards.DefaultIndicators())
	if err != nil {
		t.Fatalf("the shipped indicator list does not load: %v", err)
	}
	got := in.Words(standards.RoleReason)
	if len(got) == 0 {
		t.Fatal("the shipped indicator list carries no reason words")
	}
	return got
}

// texts renders the claims for comparison.
func texts(claims []segment.Claim) []string {
	out := make([]string, 0, len(claims))
	for _, c := range claims {
		out = append(out, c.Text)
	}
	return out
}

// TestACutIsRefusedWhenItWouldStrandAReason is the measured defect this list exists
// to close, and the sentence is the one the probe produced.
//
// "Because the SLA is 400ms." passes every rule segmentation had: it is separated by
// a coordinating join, it is non-empty, it does not open with a pronoun, and
// standsAlone finds a copula with something before it. It is still a fragment whose
// main clause is in its sibling, and the package's stated invariant — every returned
// claim stands alone — did not hold for it.
func TestACutIsRefusedWhenItWouldStrandAReason(t *testing.T) {
	t.Parallel()
	const sentence = "The retry budget is three, and because the SLA is 400ms."

	if got := segment.Claims(sentence, nil); len(got) != 2 {
		t.Fatalf("the fixture no longer reproduces the defect; got %d claims: %v",
			len(got), texts(got))
	}
	got := segment.Claims(sentence, reasons(t))
	if len(got) != 1 {
		t.Fatalf("the cut was made anyway: %v", texts(got))
	}
	if got[0].Text != sentence {
		t.Errorf("the refused sentence was altered: %q", got[0].Text)
	}
}

// TestARefusalCostsCoarsenessAndNeverInvention is the property that makes a false
// positive survivable, and it is the reason this list gates cuts rather than making
// them: whatever the words get wrong, the result is one claim whose evidence must
// cover more of it — never a claim nobody wrote.
func TestARefusalCostsCoarsenessAndNeverInvention(t *testing.T) {
	t.Parallel()
	markers := reasons(t)

	for _, in := range []string{
		"The cache is enabled, but it is not shared across sessions.",
		"The retry budget is three, and because the SLA is 400ms.",
		"Timeouts are enforced; however, retries are not.",
		"The cache is cleared, but since the flag is off nothing happens.",
	} {
		var rebuilt strings.Builder
		for _, c := range segment.Claims(in, markers) {
			rebuilt.WriteString(c.Anchor)
			// Every anchor must appear in the input verbatim. A refusal may make a
			// claim coarser; it must never make one the author did not write.
			if !strings.Contains(in, c.Anchor) {
				t.Errorf("%q produced an anchor absent from it: %q", in, c.Anchor)
			}
		}
		if rebuilt.Len() == 0 {
			t.Errorf("%q produced nothing", in)
		}
	}
}

// TestAMarkerMustOpenTheClauseRatherThanAppearInIt is the boundary that keeps the
// list from stopping segmentation almost entirely: these words make a clause
// dependent only when they introduce it.
func TestAMarkerMustOpenTheClauseRatherThanAppearInIt(t *testing.T) {
	t.Parallel()
	markers := reasons(t)

	// "because" is inside the right clause, not at its head, so the clause is a
	// complete assertion and the cut stands.
	const inside = "The budget is three, and we keep it because the SLA is tight."
	if got := segment.Claims(inside, markers); len(got) != 2 {
		t.Errorf("a clause merely containing a marker was refused: %v", texts(got))
	}
}

// TestAWordBoundaryIsRequired keeps "sincere" from reading as "since".
func TestAWordBoundaryIsRequired(t *testing.T) {
	t.Parallel()
	const sentence = "The report is short, and sincere effort is the reason."
	if got := segment.Claims(sentence, reasons(t)); len(got) != 2 {
		t.Errorf("a word merely starting with a marker was refused: %v", texts(got))
	}
}

// TestNoMarkersBehavesAsBefore is the degradation path: a corpus whose indicator file
// will not load segments exactly as it did before the file existed, which is coarser
// only where the words would have helped.
func TestNoMarkersBehavesAsBefore(t *testing.T) {
	t.Parallel()
	const sentence = "The cache is enabled, but it is not shared across sessions."
	if got := segment.Claims(sentence, nil); len(got) != 2 {
		t.Errorf("an ordinary cut stopped happening with no markers: %v", texts(got))
	}
}
