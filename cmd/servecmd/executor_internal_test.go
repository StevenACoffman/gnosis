package servecmd

// White-box, and the filename is what the linter sanctions for it. What is under test is
// the transport's own correction to a message — a function with no exported surface,
// because the outcome it corrects is the coordinator's and the correction is this
// package's private business. Reaching it through the HTTP surface would test the
// coordinator and the server to assert one sentence.

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestABlockedDecisionSaysWhatThisSurfaceCanDo is the defect a hand run found and the
// tests had not: the refusal was correct and its instruction was impossible.
//
// §4.6.2.1 keeps the escalated path terminal-only, so the coordinator asks the caller to
// "confirm by typing the document's path exactly" — right at a terminal, unfollowable
// here. A reviewer told to type something they cannot type concludes the server is
// broken rather than that the decision belongs elsewhere.
func TestABlockedDecisionSaysWhatThisSurfaceCanDo(t *testing.T) {
	t.Parallel()

	got := withTransportRemedy(gnosis.Blocked(gnosis.ReasonNeedsHuman,
		"confirm by typing the document's path exactly: c/a.md",
		map[string]any{"decision": "needs_human"}))

	if !strings.Contains(got.Message, "gnosis promote") {
		t.Errorf("the refusal does not name where the decision can be finished: %q",
			got.Message)
	}
	// The verdict is untouched. §8.0: "one verdict, two renderings — never two
	// verdicts", and only `message` is for a person.
	if got.Status != gnosis.StatusBlocked || got.Code != gnosis.CodeBlocked ||
		got.Reason != gnosis.ReasonNeedsHuman {
		t.Errorf("the verdict changed with the rendering: %+v", got)
	}
	if !strings.Contains(got.Message, "confirm by typing") {
		t.Error("the coordinator's own message was replaced rather than extended")
	}
}

// TestAnOutcomeThatIsNotBlockedIsUntouched. The remedy is for one case, and a sentence
// appended to a successful promotion would be a server explaining a problem nobody has.
func TestAnOutcomeThatIsNotBlockedIsUntouched(t *testing.T) {
	t.Parallel()

	for name, outcome := range map[string]gnosis.Outcome{
		"ok":       gnosis.OK(map[string]any{"path": "c/a.md"}),
		"findings": gnosis.Findings(gnosis.ReasonUnparsable, "a document is unreadable", nil),
		"blocked for another reason": gnosis.Blocked(
			gnosis.ReasonWriterBusy, "somebody else is writing", nil),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := withTransportRemedy(outcome); got.Message != outcome.Message {
				t.Errorf("message = %q, want it unchanged", got.Message)
			}
		})
	}
}
