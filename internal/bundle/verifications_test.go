package bundle_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

const verifiedID = gnosis.ID("0192a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b")

// TestADocumentLevelVerificationIsNotInherited is the adversarial case, and it is the
// decision this table's grain turned on.
//
// OKF §5.2 puts `verified` at document level and §5.5 keys the table by claim. Expanding
// one to the other would assert that somebody verified each claim when they verified a
// page — §5.5.1 refused exactly that inheritance for `subject`, and §5.5's own reason for
// making this a table is that a human sign-off and an automated pass must stay
// distinguishable. A page-level expansion destroys that distinction wholesale, and it
// would raise every claim's trust tier for free.
func TestADocumentLevelVerificationIsNotInherited(t *testing.T) {
	t.Parallel()

	// The document declares `verified`; the claim does not. Nothing may be written.
	docs := []bundle.Document{{
		ID: verifiedID, Path: "c/a.md", Type: "Rule", Body: "A sentence.",
		Claims: []bundle.DocClaim{{ID: "c1", Anchor: "A sentence."}},
	}}
	if rows := bundle.VerificationRows(docs); len(rows) != 0 {
		t.Errorf("a claim inherited %d verification(s) it did not declare", len(rows))
	}
}

// TestAnActorWithNoTimestampIsKept is OKF §11's tolerance where it costs something to
// get wrong: the actor is the half §14.1's trust fold reads, so dropping the event would
// lower a concept's tier because somebody omitted a date.
func TestAnActorWithNoTimestampIsKept(t *testing.T) {
	t.Parallel()
	docs := []bundle.Document{{
		ID: verifiedID, Path: "c/a.md", Type: "Rule", Body: "A sentence.",
		Claims: []bundle.DocClaim{{
			ID: "c1", Anchor: "A sentence.",
			Verified: []bundle.Verification{{By: "human:priya"}},
		}},
	}}
	rows := bundle.VerificationRows(docs)
	if len(rows) != 1 {
		t.Fatalf("want one row, got %d", len(rows))
	}
	if rows[0].By != "human:priya" || rows[0].At != "" {
		t.Errorf("row = %+v, want the actor kept and the time empty", rows[0])
	}
}

// TestEventsOnAnUnaddressableClaimAreSkipped is the foreign key stated as a test: a
// verification points at a claim row, and a claim with no anchor has none.
func TestEventsOnAnUnaddressableClaimAreSkipped(t *testing.T) {
	t.Parallel()
	docs := []bundle.Document{{
		ID: verifiedID, Path: "c/a.md", Type: "Rule", Body: "A sentence.",
		Claims: []bundle.DocClaim{{
			ID: "c1", Verified: []bundle.Verification{{By: "human:priya", At: "2026-08-27"}},
		}},
	}}
	if rows := bundle.VerificationRows(docs); len(rows) != 0 {
		t.Errorf("an unaddressable claim produced %d verification row(s)", len(rows))
	}
}

// TestTwoEventsWithOneActorAreBothKept is why the table has no key. OKF §5.2 makes these
// events, and two events are two facts even when they look alike — collapsing them would
// lose that a claim was re-verified.
func TestTwoEventsWithOneActorAreBothKept(t *testing.T) {
	t.Parallel()
	docs := []bundle.Document{{
		ID: verifiedID, Path: "c/a.md", Type: "Rule", Body: "A sentence.",
		Claims: []bundle.DocClaim{{
			ID: "c1", Anchor: "A sentence.",
			Verified: []bundle.Verification{
				{By: "human:priya", At: "2026-03-01"},
				{By: "human:priya", At: "2026-08-27"},
			},
		}},
	}}
	if rows := bundle.VerificationRows(docs); len(rows) != 2 {
		t.Errorf("re-verification collapsed to %d row(s)", len(rows))
	}
}
