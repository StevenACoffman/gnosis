package gnosis_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestFoldAuthority walks §10.6.1's three tiers and the boundaries between them,
// because the boundaries are where a corpus discovers what it now requires.
func TestFoldAuthority(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		adjudicators int
		want         gnosis.Authority
		coSigner     bool
		override     bool
	}{
		// A corpus nobody has adjudicated in is a supported configuration, not a
		// degenerate one: at sole there is no second signer to require.
		"nobody": {0, gnosis.AuthoritySole, false, true},
		"one":    {1, gnosis.AuthoritySole, false, true},
		"two":    {2, gnosis.AuthorityPaired, true, true},
		"three":  {3, gnosis.AuthorityPaired, true, true},
		// Quorum is the ceiling, and the one authority whose requirement cannot be
		// waived: with four adjudicators an unavailable one is not a reason the
		// corpus cannot proceed.
		"four":       {4, gnosis.AuthorityQuorum, true, false},
		"forty":      {40, gnosis.AuthorityQuorum, true, false},
		"a negative": {-1, gnosis.AuthoritySole, false, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := gnosis.FoldAuthority(tc.adjudicators)
			if got != tc.want {
				t.Errorf("FoldAuthority(%d) = %v, want %v",
					tc.adjudicators, got, tc.want)
			}
			if got.RequiresCoSigner() != tc.coSigner {
				t.Errorf("%v requires a co-signer = %v, want %v",
					got, got.RequiresCoSigner(), tc.coSigner)
			}
			if got.OverridePermitted() != tc.override {
				t.Errorf("%v permits an override = %v, want %v",
					got, got.OverridePermitted(), tc.override)
			}
		})
	}
}

// TestTheZeroAuthorityRequiresNothing. Every other zero value here is chosen so an
// unpopulated value cannot flatter the corpus; this one is chosen so an unpopulated
// value cannot **block**, because §10.6.3 says escalation must never deadlock.
func TestTheZeroAuthorityRequiresNothing(t *testing.T) {
	t.Parallel()

	var a gnosis.Authority
	if a != gnosis.AuthoritySole || a.RequiresCoSigner() {
		t.Error("the zero Authority requires a co-signer, so an unpopulated snapshot" +
			" would stop a corpus adjudicating anything")
	}
	if got := gnosis.Authority(99).String(); got != "sole" {
		t.Errorf("an unrecognised authority renders as %q", got)
	}
}

// TestAuthorityOfDistinguishesSilenceFromSole is the comma-ok case, and it matters
// because §10.6.3 makes the authority in force at the time of the decision the one
// that governs it: a warrant that records none leaves a reader unable to reconstruct
// what was required, and that is a different fact from a warrant that records `sole`.
func TestAuthorityOfDistinguishesSilenceFromSole(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		declared string
		want     gnosis.Authority
		found    bool
	}{
		"sole":           {"sole", gnosis.AuthoritySole, true},
		"paired":         {"paired", gnosis.AuthorityPaired, true},
		"quorum":         {"quorum", gnosis.AuthorityQuorum, true},
		"padded":         {"  Quorum  ", gnosis.AuthorityQuorum, true},
		"absent":         {"", gnosis.AuthoritySole, false},
		"something else": {"committee", gnosis.AuthoritySole, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, found := gnosis.AuthorityOf(tc.declared)
			if got != tc.want || found != tc.found {
				t.Errorf("AuthorityOf(%q) = (%v, %v), want (%v, %v)",
					tc.declared, got, found, tc.want, tc.found)
			}
		})
	}
}

// TestAdjudicatedIsDerivedFromAnyField. §10.4's provenance class is derived rather
// than declared, so any recorded part of a decision marks the claim as adjudicated —
// including a warrant carrying only a rationale, which is the state `warrant` reports.
func TestAdjudicatedIsDerivedFromAnyField(t *testing.T) {
	t.Parallel()

	var bare gnosis.Warrant
	if bare.Adjudicated() {
		t.Error("the zero Warrant reports itself as a decision")
	}
	for name, w := range map[string]gnosis.Warrant{
		"by only":        {By: "human:priya"},
		"rationale only": {Rationale: "the vendor's published limit is newer"},
		"authority only": {Authority: "paired"},
		"override only":  {OverrideReason: "marcus on leave"},
		"reverses only":  {Reverses: "01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d"},
	} {
		if !w.Adjudicated() {
			t.Errorf("a warrant with %s does not report as a decision", name)
		}
	}
}

// TestAnAuthorityMoveIsTheChangeAndNotTheCount is the distinction that keeps `log.md`
// readable: a corpus going from two adjudicators to three stays at `paired`, and
// announcing it would file an entry saying nothing changed.
func TestAnAuthorityMoveIsTheChangeAndNotTheCount(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		move  gnosis.AuthorityMove
		moved bool
	}{
		"sole to paired": {
			gnosis.AuthorityMove{
				From: gnosis.AuthoritySole, To: gnosis.AuthorityPaired, Adjudicators: 2,
			}, true,
		},
		"paired to quorum": {
			gnosis.AuthorityMove{
				From: gnosis.AuthorityPaired, To: gnosis.AuthorityQuorum, Adjudicators: 4,
			}, true,
		},
		// The authority scales down too, and §10.6.3 requires that to be announced as
		// well: a corpus that stopped requiring a co-signer has loosened.
		"quorum back to paired": {
			gnosis.AuthorityMove{
				From: gnosis.AuthorityQuorum, To: gnosis.AuthorityPaired, Adjudicators: 3,
			}, true,
		},
		"a third adjudicator inside paired": {
			gnosis.AuthorityMove{
				From: gnosis.AuthorityPaired, To: gnosis.AuthorityPaired, Adjudicators: 3,
			}, false,
		},
		"the zero move": {gnosis.AuthorityMove{}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tc.move.Moved(); got != tc.moved {
				t.Errorf("Moved() = %v, want %v", got, tc.moved)
			}
			if tc.moved {
				assertNamesBothEnds(t, tc.move)
			}
		})
	}
}

// assertNamesBothEnds checks the announcement's sentence.
//
// §10.6.3 asks the announcement to say *why*, and the population is the why: a reader
// told the corpus moved to `paired` still cannot tell whether one person arrived or
// three, and only one of those is one departure from moving back.
func assertNamesBothEnds(t *testing.T, move gnosis.AuthorityMove) {
	t.Helper()

	sentence := move.String()
	for _, want := range []string{
		move.From.String(), move.To.String(), "adjudicator",
	} {
		if !strings.Contains(sentence, want) {
			t.Errorf("%q omits %q", sentence, want)
		}
	}
}
