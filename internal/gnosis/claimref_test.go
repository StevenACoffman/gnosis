package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// refID is a well-formed document identifier, since a reference's left half must be one.
const refID = gnosis.ID("01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d")

// TestClaimRefRoundTrips is the property all three consumers rely on: a supersession
// edge written today is read back by a report tomorrow, and the critic's ledger matches
// a claim across runs.
func TestClaimRefRoundTrips(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a short claim id":       "c1",
		"a claim id with dashes": "claim-reviewed",
		// The parser cuts at the last separator, so an identifier containing one
		// still resolves on its left. Nothing writes such a claim id today.
		"a claim id carrying the separator": "claim#two",
	}
	for name, claimID := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ref := gnosis.ClaimRef(refID, claimID)
			id, claim, ok := gnosis.ParseClaimRef(ref)
			if !ok || id != refID || claim != claimID {
				t.Errorf("ParseClaimRef(%q) = (%q, %q, %v), want (%q, %q, true)",
					ref, id, claim, ok, refID, claimID)
			}
		})
	}
}

// TestAClaimRefNamesAnIdentifierAndNeverAPath is §5.4's rule, and the reason it is a
// test rather than a comment: the path form reads better, shipped for one day, and
// breaks on exactly the retitle §5.1.1 makes routine.
func TestAClaimRefNamesAnIdentifierAndNeverAPath(t *testing.T) {
	t.Parallel()

	// The form written before this corrected itself. It has an identifier inside it,
	// and extracting one would be guessing that the substring means what it looks
	// like — so it is refused rather than repaired.
	pathForm := "c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-retry-budget.md#claim-reviewed"
	if _, _, ok := gnosis.ParseClaimRef(pathForm); ok {
		t.Error("a path-form reference parsed; an edge silently reinterpreted is worse" +
			" than one reported as unreadable")
	}
	// And what is written carries no slug, so a retitle cannot break it.
	if got := gnosis.ClaimRef(refID, "c1"); got != refID.String()+"#c1" {
		t.Errorf("ClaimRef = %q, want the identifier and the claim", got)
	}
}

// TestParseClaimRefRefusesWhatItCannotRead. A bare identifier is the form written before
// ClaimRef existed, and reading it as a document would send somebody to a concept that
// does not exist — a worse answer than saying the reference cannot be read.
func TestParseClaimRefRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for _, ref := range []string{
		"", "claim-reviewed", "#c1", refID.String() + "#", "#",
		"not-a-uuid#c1", "01932b7c#c1",
	} {
		if _, _, ok := gnosis.ParseClaimRef(ref); ok {
			t.Errorf("ParseClaimRef(%q) reported a reference it cannot resolve", ref)
		}
	}
}
