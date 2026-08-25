package segment_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/segment"
)

// TestTheWorkedExample is SPEC §5.5's own case, and the reason this package
// exists: one sentence, two assertions, and a quote validating only the first half
// must not report the second supported.
func TestTheWorkedExample(t *testing.T) {
	t.Parallel()
	const sentence = "The cache is enabled by default, but it is not shared across sessions."

	got := segment.Claims(sentence, nil)
	if len(got) != 2 {
		t.Fatalf("got %d claims, want 2:\n%+v", len(got), got)
	}

	if !strings.Contains(got[1].Text, "The cache") {
		t.Errorf("the second claim did not recover its subject: %q", got[1].Text)
	}
	if !got[1].Substituted {
		t.Error("the second claim recovered a subject but does not say so")
	}
	// The anchor must still be findable in the document; the text need not be.
	if !strings.Contains(sentence, got[1].Anchor) {
		t.Errorf("anchor %q does not appear in the source sentence", got[1].Anchor)
	}
	if got[1].Anchor == got[1].Text {
		t.Error("anchor and text are identical after substitution; one of them is wrong")
	}
}

// TestRefusesTheCutItCannotMake is the guarantee. A clause whose subject cannot be
// recovered must leave the sentence whole — a coarse claim validates honestly,
// while a subjectless one validates against anything.
func TestRefusesTheCutItCannotMake(t *testing.T) {
	t.Parallel()
	// No copula in the left clause, so no subject can be recovered for "it".
	const sentence = "Deploy on Friday, but it rarely ends well."

	got := segment.Claims(sentence, nil)
	if len(got) != 1 {
		t.Fatalf("cut a sentence whose subject could not be recovered:\n%+v", got)
	}
	if got[0].Substituted {
		t.Error("a refused cut is marked substituted")
	}
}

// TestSplitterTraps covers each failure case SPEC §9.4 names by name. Every one of
// these is a place a naive splitter cuts a token in half.
func TestSplitterTraps(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   string
		want int
	}{
		"decimal":       {"The timeout is 2.5 seconds and that is fixed.", 1},
		"filename":      {"Edit README.md before shipping.", 1},
		"call":          {"Use foo.bar() to reach it.", 1},
		"url":           {"Fetch https://example.com/a.html for details.", 1},
		"abbrev lower":  {"Use a cache, e.g. the shared one.", 1},
		"abbrev upper":  {"Use a cache, e.g. Redis is fine.", 1},
		"initial":       {"A. Turing wrote it.", 1},
		"two sentences": {"The cache is warm. The queue is empty.", 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := segment.Sentences(tc.in); len(got) != tc.want {
				t.Errorf("got %d sentences, want %d: %q", len(got), tc.want, got)
			}
		})
	}
}

// TestNoAssertionIsDropped is the property that makes over-splitting safe: whatever
// the cut, every word of the original survives in some anchor.
func TestNoAssertionIsDropped(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"The cache is enabled by default, but it is not shared across sessions.",
		"The queue is bounded and the workers are idle.",
		"One sentence. Another sentence, but it is short.",
		"No terminator here",
		"",
	}
	for _, in := range inputs {
		anchors := make([]string, 0)
		for _, c := range segment.Claims(in, nil) {
			anchors = append(anchors, c.Anchor)
		}
		// Joined with a space, because the cut consumes the separator it cut on.
		lost := missingWords(in, strings.Join(anchors, " "))
		if len(lost) > 0 {
			t.Errorf("input %q dropped %v", in, lost)
		}
	}
}

// missingWords returns the words of in that do not appear in got, ignoring the
// separators a cut necessarily consumes.
func missingWords(in, got string) []string {
	present := map[string]bool{}
	for _, w := range strings.Fields(got) {
		present[strings.Trim(strings.ToLower(w), `,.;:`)] = true
	}
	var lost []string
	for _, w := range strings.Fields(in) {
		w = strings.Trim(strings.ToLower(w), `,.;:`)
		switch w {
		case "", "but", "and", "however":
			continue
		}
		if !present[w] {
			lost = append(lost, w)
		}
	}
	return lost
}

// TestEveryClaimStandsAlone is the rule itself, asserted structurally: a claim
// produced by a cut may not open with an anaphoric pronoun, because that is exactly
// a claim whose subject sits in the half that was cut away. A sentence left whole is
// exempt — "It is not shared" is the author's own wording, not a cut we made.
func TestEveryClaimStandsAlone(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"The cache is enabled by default, but it is not shared across sessions.",
		"The retry budget is three; however, it resets hourly.",
		"The queue is bounded, and they are drained nightly.",
		"Latency is low but it varies.",
	}
	for _, in := range inputs {
		claims := segment.Claims(in, nil)
		if len(claims) < 2 {
			continue // the cut was refused; nothing was promoted
		}
		for _, c := range claims {
			if startsWithPronoun(c.Text) {
				t.Errorf("input %q emitted a dangling claim: %q", in, c.Text)
			}
		}
	}
}

// startsWithPronoun is the test's own reading of the rule, deliberately not the
// package's — a test that reuses the implementation's predicate proves only that the
// code agrees with itself.
func startsWithPronoun(claim string) bool {
	fields := strings.Fields(claim)
	if len(fields) == 0 {
		return false
	}
	first := strings.ToLower(strings.Trim(fields[0], `,.;:`))
	for _, p := range []string{"it", "they", "this", "these", "that", "those", "he", "she", "we"} {
		if first == p {
			return true
		}
	}
	return false
}

// TestIsPure: the segmenter feeds an admission gate, so two runs over one input
// must agree exactly.
func TestIsPure(t *testing.T) {
	t.Parallel()
	const in = "The cache is enabled by default, but it is not shared across sessions."

	first := segment.Claims(in, nil)
	for range 20 {
		again := segment.Claims(in, nil)
		if len(again) != len(first) {
			t.Fatalf("count varies: %d then %d", len(first), len(again))
		}
		for i := range first {
			if again[i] != first[i] {
				t.Fatalf("claim %d varies:\n%+v\n%+v", i, first[i], again[i])
			}
		}
	}
}
