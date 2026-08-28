package gnosis_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// population builds n distinct items, which is enough shape for the properties
// below and keeps the failure output readable.
func population(n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, "c/doc-"+strconv.Itoa(i)+".md")
	}
	return out
}

// TestTheSameSeedDrawsTheSameSample is the property §18.3 requires and the reason
// this function exists at all: a draw nobody can repeat is not a measurement.
func TestTheSameSeedDrawsTheSameSample(t *testing.T) {
	t.Parallel()

	pop := population(50)
	first := gnosis.Sample(20260822, 5, pop)
	second := gnosis.Sample(20260822, 5, pop)
	if strings.Join(first, ",") != strings.Join(second, ",") {
		t.Errorf("two draws at one seed differ:\n%v\n%v", first, second)
	}
}

// TestADifferentSeedDrawsADifferentSample, or the seed is decorative and §6.2.1's
// "change the seed and look again" is not available.
//
// One seed pair could coincide, so several are tried and the assertion is that not
// all of them match. That is weaker than "every seed differs" and it is the true
// property: a sampler is allowed to draw the same five twice by chance.
func TestADifferentSeedDrawsADifferentSample(t *testing.T) {
	t.Parallel()

	pop := population(50)
	base := strings.Join(gnosis.Sample(1, 5, pop), ",")
	for seed := uint64(2); seed < 8; seed++ {
		if strings.Join(gnosis.Sample(seed, 5, pop), ",") != base {
			return
		}
	}
	t.Error("six seeds all drew the same sample; the seed is not reaching the draw")
}

// TestTheDrawIsIndependentOfInputOrder. A population gathered by walking a
// directory arrives in whatever order the filesystem produced, which SPEC §18.3
// lists as its own source of non-determinism. A shuffle would inherit it.
func TestTheDrawIsIndependentOfInputOrder(t *testing.T) {
	t.Parallel()

	pop := population(40)
	reversed := slices.Clone(pop)
	slices.Reverse(reversed)

	forward := gnosis.Sample(7, 6, pop)
	backward := gnosis.Sample(7, 6, reversed)

	slices.Sort(forward)
	slices.Sort(backward)
	if strings.Join(forward, ",") != strings.Join(backward, ",") {
		t.Errorf("the draw depends on input order:\n%v\n%v", forward, backward)
	}
}

// TestTheResultIsInPopulationOrder, so a caller rendering a sample gets a stable
// listing rather than one that looks shuffled every time.
func TestTheResultIsInPopulationOrder(t *testing.T) {
	t.Parallel()

	pop := population(30)
	got := gnosis.Sample(3, 8, pop)
	for i := 1; i < len(got); i++ {
		if slices.Index(pop, got[i-1]) >= slices.Index(pop, got[i]) {
			t.Errorf("the sample is not in population order: %v", got)
			break
		}
	}
}

// TestEveryDrawnItemCameFromThePopulation, and no item is drawn twice. A sampler
// that could invent or duplicate an item would make a §10.5 critic sample report
// on a document that is not there.
func TestEveryDrawnItemCameFromThePopulation(t *testing.T) {
	t.Parallel()

	pop := population(25)
	got := gnosis.Sample(11, 7, pop)
	seen := map[string]bool{}
	for _, item := range got {
		if !slices.Contains(pop, item) {
			t.Errorf("%q is not in the population", item)
		}
		if seen[item] {
			t.Errorf("%q was drawn twice", item)
		}
		seen[item] = true
	}
}

// TestTheBoundaries. Each of these is a state a real caller reaches — a corpus
// smaller than the sample, a flag left at zero — and none of them should be an
// error, because a sampler that refused an empty population would make every
// caller check first.
func TestTheBoundaries(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		n    int
		pop  []string
		want int
	}{
		"n larger than the population draws all of it": {n: 10, pop: population(3), want: 3},
		"n equal to the population draws all of it":    {n: 3, pop: population(3), want: 3},
		"zero draws nothing":                           {n: 0, pop: population(3), want: 0},
		"a negative n draws nothing":                   {n: -1, pop: population(3), want: 0},
		"an empty population draws nothing":            {n: 5, pop: nil, want: 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := gnosis.Sample(1, tc.n, tc.pop)
			if len(got) != tc.want {
				t.Errorf("drew %d, want %d: %v", len(got), tc.want, got)
			}
			if got == nil {
				t.Error("drew nil rather than an empty slice")
			}
		})
	}
}

// TestTheDrawDoesNotMutateThePopulation. It is pure, and a caller that handed over
// its own slice must be able to keep using it — the ranking sorts a copy, not the
// argument.
func TestTheDrawDoesNotMutateThePopulation(t *testing.T) {
	t.Parallel()

	pop := population(20)
	before := strings.Join(pop, ",")
	gnosis.Sample(5, 6, pop)
	if strings.Join(pop, ",") != before {
		t.Error("the population was reordered by the draw")
	}
}

// TestDuplicateItemsAreDrawnIndependently. A population may legitimately hold two
// equal strings — two claims with the same anchor, say — and collapsing them would
// make the sample smaller than the caller asked for without saying so.
func TestDuplicateItemsAreDrawnIndependently(t *testing.T) {
	t.Parallel()

	pop := []string{"same", "same", "same", "other"}
	got := gnosis.Sample(2, 3, pop)
	if len(got) != 3 {
		t.Errorf("drew %d from a population of four, want 3: %v", len(got), got)
	}
}
