package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestFoldSemanticReviewKeepsFourStatesApart is §17.1's requirement, and the pair worth
// the test is the two ways of having no critic finding: a clean critic produces none, so
// "no critic finding" and "no critic ran" are different facts and only one of them is
// evidence that anybody looked.
func TestFoldSemanticReviewKeepsFourStatesApart(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		categories []string
		critiques  int
		ledgerRead bool
		want       gnosis.SemanticReview
		reviewed   bool
	}{
		"a critic verdict is in hand": {
			categories: []string{"conformance", "critic:scope"}, critiques: 1,
			ledgerRead: true, want: gnosis.SemanticFindings, reviewed: true,
		},
		"a critic ran and found nothing": {
			categories: []string{"conformance"}, critiques: 2,
			ledgerRead: true, want: gnosis.SemanticClean, reviewed: true,
		},
		"nothing semantic has run": {
			categories: []string{"conformance"}, critiques: 0,
			ledgerRead: true, want: gnosis.SemanticStructuralOnly,
		},
		"the ledger could not be read": {
			categories: []string{"conformance"}, ledgerRead: false,
			want: gnosis.SemanticReviewUnknown,
		},
		// A finding is direct evidence of the act and the ledger is a record of it,
		// so evidence beats bookkeeping: a missing .gnosis/ must not turn a corpus
		// with critic verdicts in hand into `unknown`.
		"a verdict outranks an unreadable ledger": {
			categories: []string{"critic:omission"}, ledgerRead: false,
			want: gnosis.SemanticFindings, reviewed: true,
		},
		"no findings at all and no ledger": {
			want: gnosis.SemanticReviewUnknown,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := gnosis.FoldSemanticReview(tc.categories, tc.critiques, tc.ledgerRead)
			if got != tc.want {
				t.Errorf("FoldSemanticReview(%q, %d, %v) = %v, want %v",
					tc.categories, tc.critiques, tc.ledgerRead, got, tc.want)
			}
			if got.Reviewed() != tc.reviewed {
				t.Errorf("%v.Reviewed() = %v, want %v", got, got.Reviewed(), tc.reviewed)
			}
		})
	}
}

// TestTheZeroSemanticReviewClaimsNothing. The failure this state cannot afford is
// asserting a structural pass was all that ran when nobody looked at the record.
func TestTheZeroSemanticReviewClaimsNothing(t *testing.T) {
	t.Parallel()

	var s gnosis.SemanticReview
	if s != gnosis.SemanticReviewUnknown || s.Reviewed() {
		t.Errorf("the zero SemanticReview is %v, reviewed=%v", s, s.Reviewed())
	}
	if got := s.String(); got != "unknown" {
		t.Errorf("it renders as %q", got)
	}
	// None of the words is "verified", which is the one §17.1 forbids for a
	// structural pass.
	for _, state := range []gnosis.SemanticReview{
		gnosis.SemanticReviewUnknown, gnosis.SemanticStructuralOnly,
		gnosis.SemanticClean, gnosis.SemanticFindings, gnosis.SemanticReview(99),
	} {
		if state.String() == "verified" {
			t.Errorf("%d renders as \"verified\"", state)
		}
	}
}
