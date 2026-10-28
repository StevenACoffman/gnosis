package gnosis_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// TestFoldAnswerabilityKeepsThreeRefusalsApart is the property §17.0.1 spends its length
// on, and the one a boolean would destroy.
//
// What I am afraid of is not that the fold says "no" wrongly. It is that it says "no"
// without saying *which* no: only the unresolved case is a conflict waiting to be filed,
// and only the unevidenced case is fixed by ingesting a source. A refusal that lost the
// distinction would send every reader down the same wrong path.
func TestFoldAnswerabilityKeepsThreeRefusalsApart(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		claims []gnosis.Retrieved
		want   gnosis.Answerability
	}{
		"nothing retrieved": {want: gnosis.AnswerabilitySilent},
		"an empty slice is the same as none": {
			claims: []gnosis.Retrieved{}, want: gnosis.AnswerabilitySilent,
		},
		"claims with no evidence": {
			claims: []gnosis.Retrieved{{}, {}},
			want:   gnosis.AnswerabilityUnevidenced,
		},
		"evidence and no contest": {
			claims: []gnosis.Retrieved{{Evidenced: true}},
			want:   gnosis.AnswerabilityReady,
		},
		"one contested claim among evidenced ones": {
			claims: []gnosis.Retrieved{{Evidenced: true}, {Evidenced: true, Contested: true}},
			want:   gnosis.AnswerabilityUnresolved,
		},
		"contested outranks unevidenced too": {
			claims: []gnosis.Retrieved{{Contested: true}},
			want:   gnosis.AnswerabilityUnresolved,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := gnosis.FoldAnswerability(tc.claims)
			if got != tc.want {
				t.Errorf("FoldAnswerability = %v, want %v", got, tc.want)
			}
			if got.Answerable() != (got == gnosis.AnswerabilityReady) {
				t.Errorf("Answerable disagrees with the state it was folded to: %v", got)
			}
		})
	}
}

// TestTheZeroValueRefuses. A fold nobody ran and a corpus that holds nothing must read
// the same way, because the failure they would otherwise share is a prompt emitted for a
// question nothing was retrieved for.
func TestTheZeroValueRefuses(t *testing.T) {
	t.Parallel()

	var zero gnosis.Answerability
	if zero.Answerable() {
		t.Error("the zero value claims the corpus can answer")
	}
	if zero.String() != "silent" {
		t.Errorf("the zero value is called %q", zero.String())
	}
}

// TestEveryStateNamesARemedy, including the answerable one: a caller rendering this
// cannot produce an empty line, and the three refusals must not share a sentence — that
// they differ is the clearest argument that they had to be three states.
func TestEveryStateNamesARemedy(t *testing.T) {
	t.Parallel()

	seen := map[string]gnosis.Answerability{}
	for _, s := range []gnosis.Answerability{
		gnosis.AnswerabilitySilent, gnosis.AnswerabilityUnevidenced,
		gnosis.AnswerabilityUnresolved, gnosis.AnswerabilityReady,
	} {
		remedy := gnosis.Remedy(s)
		if strings.TrimSpace(remedy) == "" {
			t.Errorf("%v names no remedy", s)
		}
		if prior, dup := seen[remedy]; dup {
			t.Errorf("%v and %v share a remedy, so the states are not distinguishable"+
				" to a reader: %q", prior, s, remedy)
		}
		seen[remedy] = s
	}
	// An unrecognised value must not fall through to silence about itself.
	if strings.TrimSpace(gnosis.Remedy(gnosis.Answerability(99))) == "" {
		t.Error("an unrecognised state names no remedy")
	}
}
