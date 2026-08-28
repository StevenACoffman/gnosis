package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

const archived = "The service retries three times before giving up on the request."

// cites builds a corpus of one claim citing one archived file with the given quotes.
func cites(quotes ...string) *lint.Snapshot {
	return &lint.Snapshot{
		ArchiveText: map[string]string{"evidence/text/aa/x.txt": archived},
		Documents: []lint.Document{{
			Path: "c/a.md", Type: "Reference",
			Claims: []lint.Claim{{
				ID: "c1", Anchor: "Retries are capped.",
				Quotes:       quotes,
				ArchivePaths: []string{"evidence/text/aa/x.txt"},
			}},
		}},
	}
}

// TestAPassageTooShortToCheckIsNotAFinding is the adversarial case, and §9.4 goes to some
// trouble to avoid the accusation it would make.
//
// `quotecheck` separates "searched for and not found" from "never searched for", and only
// the first is a statement about the source. Reporting an unchecked passage would put
// "this archived text does not support your claim" in front of a reader on the strength
// of a run too short to have been looked for at all.
func TestAPassageTooShortToCheckIsNotAFinding(t *testing.T) {
	t.Parallel()
	// Short enough that quotecheck declines to search for it.
	if got := runNamed(t, cites("retries"), "evidence"); len(got) != 0 {
		t.Errorf("an unchecked passage was reported as unsupported:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestAQuoteTheArchiveDoesNotHoldIsReported is the check, and the message carries what
// makes it actionable: the archive is content-addressed, so the frontmatter changed.
func TestAQuoteTheArchiveDoesNotHoldIsReported(t *testing.T) {
	t.Parallel()
	got := runNamed(t, cites("The service retries four times before giving up"), "evidence")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{"c1", "cannot have changed", "after admission"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not carry %q:\n%s", want, got[0])
		}
	}
}

// TestAQuoteTheArchiveHoldsIsSilent keeps the check from firing on the state it asks for.
func TestAQuoteTheArchiveHoldsIsSilent(t *testing.T) {
	t.Parallel()
	if got := runNamed(t, cites(archived), "evidence"); len(got) != 0 {
		t.Errorf("a supported quotation was reported:\n%s", strings.Join(got, "\n"))
	}
}

// TestAnUnreadableArchiveIsNotThisChecksFinding keeps one problem from being reported
// twice. A cited file that is not there is `archive-path`'s finding, and saying the
// source does not support the claim would be describing a file nobody opened.
func TestAnUnreadableArchiveIsNotThisChecksFinding(t *testing.T) {
	t.Parallel()
	snap := cites("The service retries four times before giving up")
	snap.ArchiveText = map[string]string{}
	// With nothing readable there is nothing to re-validate, so it skips rather than
	// reporting the claim.
	if reason := skipReason(t, snap, "evidence"); !strings.Contains(reason, "re-validated") {
		t.Errorf("the skip does not say why: %q", reason)
	}
}
