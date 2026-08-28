package gnosis

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

// Sample draws n items from population, reproducibly.
//
// Requires: nothing. A non-positive n, an empty population, and an n larger than
// the population are all valid; the first two draw nothing and the third draws
// everything.
// Ensures: the same (seed, n, population) always yields the same items, whatever
// order the population arrived in and whatever Go release is running. The result is
// in the population's own order, not the draw's, so a caller rendering it gets a
// stable listing rather than a shuffled one. Pure — the population is not mutated.
//
// # Why one sampler exists before any of its callers
//
// Three sections specify a reproducible draw independently: §10.5's `critic
// --sample N`, §14.3.1's `stale --unreviewed`, and §6.2.1's random conflict pass,
// which is the one that most needs it — its whole purpose is estimating what the
// deterministic selector misses, and an estimate nobody can reproduce is not a
// measurement. Three separate draws would be three seeds, three algorithms, and
// three answers to "is this reproducible under §18.3", so the sampler is written
// once, before the second caller can disagree with the first.
//
// # Why a keyed hash rather than a shuffle
//
// Each item is ranked by the hash of (seed, item) and the lowest n are taken. That
// is a deterministic function of the inputs alone, and it buys two properties a
// seeded shuffle does not.
//
// It does not depend on the standard library's internals. `math/rand/v2`'s
// generators are specified, but `rand.Shuffle`'s consumption pattern is not a
// documented interface, and §18.3 asks for reproducibility across runs — which
// includes runs on a later Go release. A hash of the bytes is reproducible by
// anything that can compute SHA-256, including a reader checking the draw by hand.
//
// It is independent of input order. A shuffle over a slice gathered by walking a
// directory would draw differently depending on filesystem iteration order, which
// is precisely the non-determinism §18.3 lists separately. Here two callers holding
// the same set in different orders draw the same items.
//
// The seed is a parameter rather than read here, because a function that fetched
// its own configuration could not be tested at two seeds and would put a file read
// inside the one thing in this package that has to stay pure.
func Sample(seed uint64, n int, population []string) []string {
	if n <= 0 || len(population) == 0 {
		return []string{}
	}
	if n >= len(population) {
		out := make([]string, len(population))
		copy(out, population)
		return out
	}

	// Rank by keyed hash. The index is carried so the result can be restored to
	// the population's order, and so two equal strings — which a population may
	// legitimately contain — do not collapse into one draw.
	type ranked struct {
		key   [sha256.Size]byte
		index int
	}
	order := make([]ranked, len(population))
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], seed)
	buf := make([]byte, 0, len(prefix)+64)
	for i, item := range population {
		buf = append(append(buf[:0], prefix[:]...), item...)
		order[i] = ranked{key: sha256.Sum256(buf), index: i}
	}
	sort.Slice(order, func(i, j int) bool {
		// The index breaks a hash tie, so the draw is total rather than dependent
		// on sort stability — which sort.Slice does not offer.
		if c := bytes.Compare(order[i].key[:], order[j].key[:]); c != 0 {
			return c < 0
		}
		return order[i].index < order[j].index
	})

	drawn := order[:n]
	sort.Slice(drawn, func(i, j int) bool { return drawn[i].index < drawn[j].index })
	out := make([]string, 0, n)
	for _, r := range drawn {
		out = append(out, population[r.index])
	}
	return out
}
