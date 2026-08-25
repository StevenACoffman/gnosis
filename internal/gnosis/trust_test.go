package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestOKFConformanceTable is SPEC §18.5.1, written before §14.1 is built.
//
// gnosis claims OKF conformance in §5, §11, and §14, and nothing checked that claim
// against the specification. This is the check: OKF §7's three actor forms plus
// `gnosis.Actor`'s two additions, asserting for each what ParseActor does **and**
// what tier the fold yields.
//
// **The two rows where they disagree are the whole test.**
// `process:finance-nightly` and `reference_agent/gemini-2.5-pro` are conformant OKF,
// are rejected by ParseActor, and are machine-confirmed for tier purposes. A table
// without them passes under exactly the merge that breaks conformance — narrowing
// the fold to Actor, which is the natural thing to write, would reject both
// documents outright, and §11 forbids rejecting a concept for the shape of an
// optional family.
//
// It is written now rather than with §14.1 because the divergence already exists in
// a shipped type, it was introduced without touching trust metadata at all, and the
// cost of finding it later is a corpus whose tiers were computed by a parser that
// refused half its inputs.
func TestOKFConformanceTable(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		actor       string
		parses      bool
		contributes gnosis.Tier
	}{
		// OKF §7's three forms.
		"human, the one form both agree on": {
			actor: "human:priya", parses: true, contributes: gnosis.TierHumanReviewed,
		},
		"process, OKF-valid and refused by the mint-side type": {
			actor: "process:finance-nightly", parses: false,
			contributes: gnosis.TierMachineConfirmed,
		},
		"producer/version, OKF-valid and refused by the mint-side type": {
			actor: "reference_agent/gemini-2.5-pro", parses: false,
			contributes: gnosis.TierMachineConfirmed,
		},
		// gnosis's two additions, which are not OKF forms.
		"agent, a gnosis kind": {
			actor: "agent:ingest", parses: true, contributes: gnosis.TierMachineConfirmed,
		},
		"check, a gnosis kind": {
			actor: "check:duplicate", parses: true, contributes: gnosis.TierMachineConfirmed,
		},
		// Unprefixed, which OKF §7 tells producers not to write and which a
		// consumer will nonetheless receive.
		"unprefixed": {
			actor: "priya", parses: false, contributes: gnosis.TierMachineConfirmed,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, ok := gnosis.ParseActor(tc.actor)
			if ok != tc.parses {
				t.Errorf("ParseActor(%q) = %t, want %t", tc.actor, ok, tc.parses)
			}
			if got := gnosis.FoldTrust([]string{tc.actor}); got != tc.contributes {
				t.Errorf("FoldTrust([%q]) = %v, want %v", tc.actor, got, tc.contributes)
			}
		})
	}
}

// TestTheFoldIsMorePermissiveThanTheParser, stated as a property rather than
// row-by-row: every actor the parser accepts the fold also classifies, and the fold
// classifies some the parser refuses. If that ever inverts, one of the two
// populations has been narrowed to the other and §14.1.1 has been broken.
func TestTheFoldIsMorePermissiveThanTheParser(t *testing.T) {
	t.Parallel()

	refused := []string{
		"process:finance-nightly",
		"reference_agent/gemini-2.5-pro",
		"priya",
		"unknown:thing",
	}
	for _, actor := range refused {
		if _, ok := gnosis.ParseActor(actor); ok {
			t.Errorf("ParseActor accepted %q; the mint-side grammar has been widened", actor)
		}
		if got := gnosis.FoldTrust([]string{actor}); got != gnosis.TierMachineConfirmed {
			t.Errorf("FoldTrust([%q]) = %v; an unrecognised actor must still count as "+
				"non-human rather than as nothing", actor, got)
		}
	}
}

// TestNoTrustFrontmatterIsUnverified. OKF §5.3's first rule, and the state of every
// document in a corpus that has not started recording verification — so the wrong
// answer here would misreport the whole corpus on day one.
func TestNoTrustFrontmatterIsUnverified(t *testing.T) {
	t.Parallel()

	for name, list := range map[string][]string{
		"nil":            nil,
		"empty":          {},
		"a blank entry":  {""},
		"blank entries":  {"", "   "},
		"blank and tabs": {"\t"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := gnosis.FoldTrust(list); got != gnosis.TierUnverified {
				t.Errorf("FoldTrust(%q) = %v, want unverified", list, got)
			}
		})
	}
}

// TestOneHumanIsEnough, and order does not matter. OKF §5.3 says `verified` by a
// human actor is human-reviewed; a list mixing machines and a person is not
// downgraded by whichever entry happens to come last.
func TestOneHumanIsEnough(t *testing.T) {
	t.Parallel()

	for name, list := range map[string][]string{
		"human first": {"human:priya", "agent:ingest", "process:nightly"},
		"human last":  {"agent:ingest", "process:nightly", "human:priya"},
		"human alone": {"human:priya"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := gnosis.FoldTrust(list); got != gnosis.TierHumanReviewed {
				t.Errorf("FoldTrust(%v) = %v, want human-reviewed", list, got)
			}
		})
	}
}

// TestTheHumanPrefixIsExact. Accepting `Human:` would be guessing at a producer's
// intent in the direction that raises a tier, which is the only direction this fold
// must never guess in.
func TestTheHumanPrefixIsExact(t *testing.T) {
	t.Parallel()

	for _, actor := range []string{"Human:priya", "HUMAN:priya", "human", "human:", " human:priya"} {
		if gnosis.IsHumanActor(actor) {
			t.Errorf("IsHumanActor(%q) = true; the prefix must be exact", actor)
		}
		if got := gnosis.FoldTrust([]string{actor}); got == gnosis.TierHumanReviewed {
			t.Errorf("FoldTrust([%q]) promoted to human-reviewed", actor)
		}
	}
}

// TestTheZeroTierIsUnverified. The same discipline as every other zero value here:
// a value nobody populated must not claim the strongest thing in the set.
func TestTheZeroTierIsUnverified(t *testing.T) {
	t.Parallel()

	var tier gnosis.Tier
	if tier != gnosis.TierUnverified {
		t.Error("the zero Tier is not unverified")
	}
	if tier.String() != "unverified" {
		t.Errorf("the zero Tier renders as %q", tier.String())
	}
	// An out-of-range tier reports the weakest thing it could be rather than
	// falling through to a name that asserts confirmation.
	if got := gnosis.Tier(99).String(); got != "unverified" {
		t.Errorf("an unrecognised tier renders as %q", got)
	}
}
