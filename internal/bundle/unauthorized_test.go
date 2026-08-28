package bundle_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/audit"
	"github.com/StevenACoffman/gnosis/internal/bundle"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// refused is an admit row for a reply the archive did not support.
func refused(source string, claims ...string) audit.Row {
	return audit.Row{
		At: noon, Op: audit.OpAdmit, Actor: "agent:test",
		Paths: []string{source}, Outcome: string(gnosis.StatusBlocked),
		Unsupported: claims,
	}
}

// TestWhatASourceDoesNotSupportIsKept is the asymmetry the entry names: a corpus
// recorded what a source supports and kept no trace of the opposite, so a refused
// assertion could be offered again with nothing saying it had been tried.
func TestWhatASourceDoesNotSupportIsKept(t *testing.T) {
	t.Parallel()

	const source = "evidence/text/ab/abc.md"
	got := bundle.NotAuthorized(&bundle.Trail{Rows: []audit.Row{
		refused(source, "the cache is shared across sessions"),
	}})

	if len(got.Withheld) != 1 {
		t.Fatalf("want one withheld claim, got %+v", got.Withheld)
	}
	wh := got.Withheld[0]
	switch {
	case wh.Source != source:
		t.Errorf("source = %q, want %q", wh.Source, source)
	case wh.Claim == "":
		t.Error("the entry does not say what was asserted")
	case wh.Submitter == "":
		t.Error("the entry does not say who asserted it")
	case wh.At.IsZero():
		t.Error("the entry does not say when")
	}
	if got.Sources != 1 {
		t.Errorf("sources = %d, want 1", got.Sources)
	}
}

// TestOnlyUnsupportedClaimsAreWithheld is the distinction this must not collapse, and
// it is the same one `quotecheck` draws.
//
// "Sought in the archive and not there" is a statement about the source. "Nobody
// looked" — every passage too short to be evidence — is not, and recording it here
// would put "this source does not support X" in the trail on the strength of a
// quotation that was never checked. §9.4 goes to some trouble not to make that
// accusation, and a report is not the place to make it instead.
func TestOnlyUnsupportedClaimsAreWithheld(t *testing.T) {
	t.Parallel()

	// An admit that was blocked and recorded no unsupported claims: the `unchecked`
	// case, which reaches the trail with the same status and nothing in this field.
	blocked := audit.Row{
		At: noon, Op: audit.OpAdmit, Actor: "agent:test",
		Paths:   []string{"evidence/text/ab/abc.md"},
		Outcome: string(gnosis.StatusBlocked),
		Detail:  "1 claims could not be checked at all",
	}
	got := bundle.NotAuthorized(&bundle.Trail{Rows: []audit.Row{blocked}})
	if len(got.Withheld) != 0 {
		t.Errorf("an unchecked claim was reported as unsupported: %+v", got.Withheld)
	}
}

// TestASuccessfulAdmitWithholdsNothing keeps the report from listing what did hold. A
// row for a reply that landed carries no unsupported claims, and a report that
// scraped its detail line would find the ordinary case.
func TestASuccessfulAdmitWithholdsNothing(t *testing.T) {
	t.Parallel()

	got := bundle.NotAuthorized(&bundle.Trail{Rows: []audit.Row{{
		At: noon, Op: audit.OpAdmit, Actor: "agent:test",
		Paths: []string{"c/01932b7c-cache.md"}, Outcome: string(gnosis.StatusOK),
		Detail: "quarantined from reply abc",
	}}})
	if len(got.Withheld) != 0 {
		t.Errorf("a successful admit contributed %+v", got.Withheld)
	}
}

// TestTheSameClaimTwiceIsTwoEvents is why the entry carries a submitter and a time.
//
// One model asserting something twice is a model that keeps proposing what the source
// does not say — which is the signal worth having — and collapsing the two would hide
// exactly that. This is the opposite choice from `Outstanding`, where three attempts
// at one decision are one outstanding decision, and the difference is that there a
// person owes an answer and here a pattern is being reported.
func TestTheSameClaimTwiceIsTwoEvents(t *testing.T) {
	t.Parallel()

	const (
		source = "evidence/text/ab/abc.md"
		claim  = "the cache is shared across sessions"
	)
	got := bundle.NotAuthorized(&bundle.Trail{Rows: []audit.Row{
		refused(source, claim),
		refused(source, claim),
	}})
	if len(got.Withheld) != 2 {
		t.Errorf("want two events, got %+v", got.Withheld)
	}
	if got.Sources != 1 {
		t.Errorf("sources = %d, want 1", got.Sources)
	}
}

// TestWithheldClaimsAreSortedAndTheFloorIsCarried keeps the report diffable and
// honest: a map has no order, and a count from a damaged trail is a floor.
func TestWithheldClaimsAreSortedAndTheFloorIsCarried(t *testing.T) {
	t.Parallel()

	got := bundle.NotAuthorized(&bundle.Trail{
		Rows: []audit.Row{
			refused("evidence/text/cd/cde.md", "queue reorders work"),
			refused("evidence/text/ab/abc.md", "the cache is shared", "and never cleared"),
		},
		Malformed: []int{7},
	})
	if len(got.Withheld) != 3 {
		t.Fatalf("want three withheld claims, got %+v", got.Withheld)
	}
	if got.Withheld[0].Source > got.Withheld[2].Source {
		t.Errorf("out of order by source: %+v", got.Withheld)
	}
	if got.Withheld[0].Claim > got.Withheld[1].Claim {
		t.Errorf("out of order by claim within a source: %+v", got.Withheld)
	}
	if got.Sources != 2 {
		t.Errorf("sources = %d, want 2", got.Sources)
	}
	if got.Complete() {
		t.Error("a damaged trail reported a total rather than a floor")
	}
}

// TestNothingWithheldIsEmptyNotNil keeps the empty envelope unambiguous: `[]` says the
// report ran and found nothing, `null` says nothing at all.
func TestNothingWithheldIsEmptyNotNil(t *testing.T) {
	t.Parallel()

	if got := bundle.NotAuthorized(&bundle.Trail{}); got.Withheld == nil {
		t.Error("an empty report is nil rather than empty")
	}
}
