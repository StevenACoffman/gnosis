package bundle_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// superseded is a two-claim document whose first claim carries a nested block, so the
// entry boundary has to step over it to reach the second.
const superseded = "---\n" +
	"type: Rule\ntitle: Request Timeout\n" +
	"gnosis_claims:\n" +
	"  - id: c1\n" +
	"    anchor: The timeout is 400ms.\n" +
	"    verified:\n" +
	"      - by: human:priya\n        at: 2026-08-01T00:00:00Z\n" +
	"  - id: c2\n" +
	"    anchor: Retries share the budget.\n" +
	"---\n# Request Timeout\n\nThe timeout is 400ms. Retries share the budget.\n"

// TestDeprecateMarksOnlyTheNamedClaim, and the nested `verified` block is why the
// fixture has one: an entry boundary that stopped at the first indented list would put
// the status on the wrong claim.
func TestDeprecateMarksOnlyTheNamedClaim(t *testing.T) {
	t.Parallel()

	out, err := bundle.Deprecate([]byte(superseded), "c2")
	if err != nil {
		t.Fatalf("Deprecate: %v", err)
	}
	text := string(out)
	if strings.Count(text, "status: deprecated") != 1 {
		t.Fatalf("want exactly one deprecation:\n%s", text)
	}
	if strings.Index(text, "status: deprecated") < strings.Index(text, "id: c2") {
		t.Errorf("the deprecation landed on the wrong claim:\n%s", text)
	}
	if !strings.HasSuffix(text, "The timeout is 400ms. Retries share the budget.\n") {
		t.Errorf("the body changed:\n%s", text)
	}
}

// TestSupersedesRecordsTheEdgeOnTheWinner, which is the half `warrant` reads: a claim
// carrying this edge and no gnosis_warrant is a decision nobody wrote down.
func TestSupersedesRecordsTheEdgeOnTheWinner(t *testing.T) {
	t.Parallel()

	out, err := bundle.Supersedes([]byte(superseded), "c1", "c/other.md#c9")
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "gnosis_supersedes:\n      - c/other.md#c9") {
		t.Errorf("the edge did not land:\n%s", text)
	}
	// On c1, which means before c2's entry begins.
	if strings.Index(text, "gnosis_supersedes") > strings.Index(text, "id: c2") {
		t.Errorf("the edge landed on the wrong claim:\n%s", text)
	}
}

// TestSupersedesAppendsToAnExistingList, so a claim that replaced two others records
// both rather than losing the first.
func TestSupersedesAppendsToAnExistingList(t *testing.T) {
	t.Parallel()

	once, err := bundle.Supersedes([]byte(superseded), "c1", "c/a.md#x")
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}
	twice, err := bundle.Supersedes(once, "c1", "c/b.md#y")
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}
	text := string(twice)
	if !strings.Contains(text, "- c/a.md#x") || !strings.Contains(text, "- c/b.md#y") {
		t.Errorf("a second supersession replaced the first:\n%s", text)
	}
	if strings.Count(text, "gnosis_supersedes:") != 1 {
		t.Errorf("the field was written twice:\n%s", text)
	}
}

// TestSupersedeRefusesAnUnknownClaim, at both ends: a supersession recorded against a
// claim that does not exist is worse than a refused one, because nothing later reports
// it.
func TestSupersedeRefusesAnUnknownClaim(t *testing.T) {
	t.Parallel()

	if _, err := bundle.Deprecate([]byte(superseded), "c9"); errs.ErrorCode(err) != errs.ENOTFOUND {
		t.Errorf("deprecating an unknown claim did not report ENOTFOUND: %v", err)
	}
	_, err := bundle.Supersedes([]byte(superseded), "c9", "c/other.md#x")
	if errs.ErrorCode(err) != errs.ENOTFOUND {
		t.Errorf("superseding from an unknown claim did not report ENOTFOUND: %v", err)
	}
}

// TestTheSupersessionEdgeNamesAnIdentifier is §5.4's rule, and the reason it is a test:
// the path form reads better, shipped for a day, and breaks on exactly the retitle
// §5.1.1 makes routine — at which point the edge names a file that does not exist,
// pointing at the one claim nobody is looking at any more.
func TestTheSupersessionEdgeNamesAnIdentifier(t *testing.T) {
	t.Parallel()

	out, err := bundle.Supersedes([]byte(superseded), "c1",
		gnosis.ClaimRef(gnosis.ID("01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"), "c9"))
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "- 01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d#c9") {
		t.Errorf("the edge does not name an identifier:\n%s", text)
	}
	// The slug is what a retitle changes, so its absence is the property.
	if strings.Contains(text, ".md#") {
		t.Errorf("the edge carries a path, which a retitle breaks:\n%s", text)
	}

	// And what was written parses back, which is what `warrant` and `audit
	// --reversed` need in order to follow it.
	for _, line := range strings.Split(text, "\n") {
		ref, found := strings.CutPrefix(strings.TrimSpace(line), "- ")
		if !found || !strings.Contains(ref, "#") {
			continue
		}
		if _, _, ok := gnosis.ParseClaimRef(ref); !ok {
			t.Errorf("the edge %q does not parse back", ref)
		}
	}
}
