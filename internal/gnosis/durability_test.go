package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestFoldDurability walks §14.4's table, and the two rows worth the test are the
// mixed one and the empty one: a page that is half archived is not provable, and a
// page citing nothing is not unprovable.
func TestFoldDurability(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		support []gnosis.Support
		want    gnosis.Durability
	}{
		"nothing cited": {
			support: nil, want: gnosis.DurabilityNotApplicable,
		},
		"every source archived": {
			support: []gnosis.Support{gnosis.SupportDurable, gnosis.SupportDurable},
			want:    gnosis.DurabilityProvable,
		},
		"every source referenced": {
			support: []gnosis.Support{gnosis.SupportWeak, gnosis.SupportWeak},
			want:    gnosis.DurabilityUnprovable,
		},
		"one archived beside one referenced": {
			support: []gnosis.Support{gnosis.SupportDurable, gnosis.SupportWeak},
			want:    gnosis.DurabilityPartlyProvable,
		},
		"order does not matter": {
			support: []gnosis.Support{gnosis.SupportWeak, gnosis.SupportDurable},
			want:    gnosis.DurabilityPartlyProvable,
		},
		// An address resolving to no record is archive-closure's finding, not this
		// one. Counting it as weak here would report one defect under two names.
		"an address that resolves to no record": {
			support: []gnosis.Support{gnosis.SupportNone},
			want:    gnosis.DurabilityNotApplicable,
		},
		"an unresolvable address beside an archived one": {
			support: []gnosis.Support{gnosis.SupportNone, gnosis.SupportDurable},
			want:    gnosis.DurabilityProvable,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := gnosis.FoldDurability(tc.support); got != tc.want {
				t.Errorf("FoldDurability(%v) = %v, want %v", tc.support, got, tc.want)
			}
		})
	}
}

// TestTheZeroDurabilityAssertsNothing. The failure direction this type cannot afford
// is an unpopulated value claiming a quotation can be checked offline.
func TestTheZeroDurabilityAssertsNothing(t *testing.T) {
	t.Parallel()

	var d gnosis.Durability
	if d != gnosis.DurabilityNotApplicable {
		t.Error("the zero Durability is not not-applicable")
	}
	if d == gnosis.DurabilityProvable {
		t.Error("the zero Durability claims a quotation can be checked offline")
	}
	if got := gnosis.Durability(99).String(); got != "not-applicable" {
		t.Errorf("an unrecognised durability renders as %q", got)
	}
	text, err := d.MarshalText()
	if err != nil || string(text) != "not-applicable" {
		t.Errorf("MarshalText = %q, %v; the envelope must carry the word", text, err)
	}
}

// TestClassifyWeakness pins §14.4.1, including the case where both conditions hold.
func TestClassifyWeakness(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		durability      gnosis.Durability
		inDegree, cut   int
		citedByProvable bool
		want            gnosis.Weakness
	}{
		"a provable document is never weak": {
			durability: gnosis.DurabilityProvable, inDegree: 40, cut: 5,
			want: gnosis.WeaknessNotWeak,
		},
		"a partly provable document is never weak": {
			durability: gnosis.DurabilityPartlyProvable, inDegree: 40, cut: 5,
			want: gnosis.WeaknessNotWeak,
		},
		"a document nothing cites is peripheral": {
			durability: gnosis.DurabilityUnprovable, inDegree: 0, cut: 5,
			want: gnosis.WeaknessPeripheral,
		},
		"one link below the cut is still peripheral": {
			durability: gnosis.DurabilityUnprovable, inDegree: 4, cut: 5,
			want: gnosis.WeaknessPeripheral,
		},
		"at the cut is load-bearing, because the cut is inclusive": {
			durability: gnosis.DurabilityUnprovable, inDegree: 5, cut: 5,
			want: gnosis.WeaknessLoadBearing,
		},
		"provable work resting on it is reported even off-centre": {
			durability: gnosis.DurabilityUnprovable, inDegree: 1, cut: 5,
			citedByProvable: true, want: gnosis.WeaknessCitedByProvable,
		},
		"both conditions: centrality is the stronger reason": {
			durability: gnosis.DurabilityUnprovable, inDegree: 9, cut: 5,
			citedByProvable: true, want: gnosis.WeaknessLoadBearing,
		},
		// A cut of zero would make in-degree zero load-bearing, which is every
		// document in a corpus with no links. The caller declines to run instead,
		// and this pins that the classifier does not manufacture the class.
		"no declared cut": {
			durability: gnosis.DurabilityUnprovable, inDegree: 0, cut: 0,
			want: gnosis.WeaknessPeripheral,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := gnosis.ClassifyWeakness(
				tc.durability, tc.inDegree, tc.cut, tc.citedByProvable)
			if got != tc.want {
				t.Errorf("ClassifyWeakness(%v, in=%d, cut=%d, cited=%v) = %v, want %v",
					tc.durability, tc.inDegree, tc.cut, tc.citedByProvable, got, tc.want)
			}
		})
	}
}
