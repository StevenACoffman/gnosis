package gnosis_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// upstreamText is long enough that each sentence is a checkable passage:
// quotecheck refuses anything under six words, so a fixture written tersely would
// test the refusal rather than the comparison.
const upstreamText = "The cache is cleared on restart and holds nothing across " +
	"sessions. Entries are addressed by the hash of their own content. " +
	"A re-fetch of unchanged bytes writes nothing at all."

// TestTheZeroDriftStateAssertsNothing is the property the type exists for.
func TestTheZeroDriftStateAssertsNothing(t *testing.T) {
	t.Parallel()

	var zero gnosis.DriftState
	if zero != gnosis.DriftUnchecked {
		t.Errorf("the zero DriftState is %v, not unchecked", zero)
	}
	if zero.String() != "drift-unchecked" {
		t.Errorf("the zero state renders as %q", zero.String())
	}
	if zero.Actionable() {
		t.Error("the zero state asks somebody to do something")
	}
	// And the same for the whole value, which is what a caller holds.
	var none gnosis.Drifted
	if none.State != gnosis.DriftUnchecked || none.Missing != nil {
		t.Errorf("the zero Drifted is %+v", none)
	}
}

// TestDriftResolvesToThreeStates is §14.3.2's table, and the reason for the entry:
// today the cheap maintenance case and the loss of upstream support both report as
// `stale`, which puts them in one bucket sized for the cheap one.
func TestDriftResolvesToThreeStates(t *testing.T) {
	t.Parallel()

	const (
		archived = "aaa"
		fetched  = "bbb"
	)
	quoted := "The cache is cleared on restart and holds nothing across sessions."

	for name, tc := range map[string]struct {
		archivedSHA, upstreamSHA, upstream string
		quotes                             []string
		want                               gnosis.DriftState
		wantMissing                        []string
	}{
		"unchanged bytes have not drifted at all": {
			archivedSHA: archived, upstreamSHA: archived,
			upstream: upstreamText, quotes: []string{quoted},
			want: gnosis.DriftNone,
		},
		"the text moved and the passage came with it": {
			archivedSHA: archived, upstreamSHA: fetched,
			// Reformatted, re-wrapped, and extended — the ordinary case, and the
			// one a hash-only check would report as a regression.
			upstream: "# Cache\n\nThe cache is cleared\non restart and holds " +
				"nothing across sessions.\n\nNew section nobody quoted yet.\n",
			quotes: []string{quoted},
			want:   gnosis.DriftBenign,
		},
		"the passage is gone from the new bytes": {
			archivedSHA: archived, upstreamSHA: fetched,
			upstream: "The cache persists across restarts and is never cleared " +
				"by anything.",
			quotes: []string{quoted},
			want:   gnosis.DriftUnsupported,
			// The *passage*, not the quotation: quotecheck splits a quotation on
			// sentence punctuation and drops it, and the passage is the right unit
			// to name — a long quotation with one fabricated sentence in it should
			// point at that sentence rather than at the whole block.
			wantMissing: []string{
				"The cache is cleared on restart and holds nothing across sessions",
			},
		},
		"a source nothing quotes was not checked": {
			archivedSHA: archived, upstreamSHA: fetched,
			upstream: upstreamText,
			want:     gnosis.DriftUnchecked,
		},
		"a quotation too short to check blocks benign": {
			archivedSHA: archived, upstreamSHA: fetched,
			upstream: upstreamText,
			// Under quotecheck.MinPassageWords, so it yields Unchecked. Calling
			// the source benign on the strength of the passages that did check
			// would claim support for a claim nobody verified.
			quotes: []string{"The cache."},
			want:   gnosis.DriftUnchecked,
		},
		"one lost passage is not offset by one that held": {
			archivedSHA: archived, upstreamSHA: fetched,
			upstream: upstreamText,
			quotes: []string{
				"Entries are addressed by the hash of their own content.",
				"Every entry carries the moment it was written down.",
			},
			want:        gnosis.DriftUnsupported,
			wantMissing: []string{"Every entry carries the moment it was written down"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := gnosis.Drift(
				tc.archivedSHA, tc.upstreamSHA, tc.upstream, tc.quotes)
			if got.State != tc.want {
				t.Errorf("state = %v, want %v", got.State, tc.want)
			}
			assertSamePassages(t, got.Missing, tc.wantMissing)
		})
	}
}

// TestDriftRefusesToManufactureACatastrophe is the guard that matters most, and it
// is not a hypothetical: a fetch returning an empty body is an ordinary network
// event, and every recorded passage is genuinely absent from empty text.
//
// Without the guard, one 404 reports `drift-unsupported` — the most serious signal
// §10 can receive — for every claim in the corpus resting on that source.
func TestDriftRefusesToManufactureACatastrophe(t *testing.T) {
	t.Parallel()

	quotes := []string{"The cache is cleared on restart and holds nothing across sessions."}
	for name, tc := range map[string]struct {
		archivedSHA, upstreamSHA, upstream string
	}{
		"the fetch came back empty":       {"aaa", "e3b0c442", ""},
		"the archived hash is unknown":    {"", "bbb", upstreamText},
		"the upstream hash is unknown":    {"aaa", "", upstreamText},
		"neither hash could be computed":  {"", "", upstreamText},
		"empty text and no hashes either": {"", "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := gnosis.Drift(
				tc.archivedSHA, tc.upstreamSHA, tc.upstream, quotes)
			if got.State != gnosis.DriftUnchecked {
				t.Errorf("state = %v, want drift-unchecked", got.State)
			}
			if len(got.Missing) != 0 {
				t.Errorf("it named %v as missing", got.Missing)
			}
		})
	}
}

// TestOnlyUnsupportedDriftIsActionable keeps §14.3.2's third consequence checkable.
//
// `drift-benign` rendered as a warning "would train readers past the state that
// matters", which is the failure the whole three-state split exists to avoid — so the
// predicate every reporter branches on is asserted here rather than in each of them.
func TestOnlyUnsupportedDriftIsActionable(t *testing.T) {
	t.Parallel()

	for state, want := range map[gnosis.DriftState]bool{
		gnosis.DriftUnchecked:   false,
		gnosis.DriftNone:        false,
		gnosis.DriftBenign:      false,
		gnosis.DriftUnsupported: true,
	} {
		if got := state.Actionable(); got != want {
			t.Errorf("%v.Actionable() = %v, want %v", state, got, want)
		}
	}
}

// TestEveryDriftStateRendersAWord is what the machine envelope rests on: an agent
// branches on the word, so a state that marshalled as "invalid" would be a state no
// caller could act on.
func TestEveryDriftStateRendersAWord(t *testing.T) {
	t.Parallel()

	for state, want := range map[gnosis.DriftState]string{
		gnosis.DriftUnchecked:   "drift-unchecked",
		gnosis.DriftNone:        "drift-none",
		gnosis.DriftBenign:      "drift-benign",
		gnosis.DriftUnsupported: "drift-unsupported",
	} {
		text, err := state.MarshalText()
		if err != nil {
			t.Fatalf("marshal %v: %v", state, err)
		}
		if string(text) != want {
			t.Errorf("marshalled as %q, want %q", text, want)
		}
	}
	if got := gnosis.DriftState(99).String(); got != "invalid" {
		t.Errorf("an undeclared state renders as %q, want invalid", got)
	}
}

// assertSamePassages compares a passage list to what was expected, treating nil and
// empty as the same absence — a caller asks whether anything was named.
func assertSamePassages(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("missing = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("missing[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
