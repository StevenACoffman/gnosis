package standards

import (
	_ "embed"
	"strconv"

	"github.com/StevenACoffman/skillet/errs"
)

// SampleFileName is where the draw's seed lives, relative to the bundle root.
const SampleFileName = "standards/sample.toml"

// defaultSample is the seed, embedded for the same reason the other two seeds are:
// its rationale carries what a reviewer reads before changing the value, and
// marshalling a Sample back to TOML would drop it.
//
//go:embed sample.toml
var defaultSample []byte

// Sample is the reproducible-draw configuration (§6.2.1, §10.5, §14.3.1).
//
// A file of its own, because there is no existing file these belong in — archive.toml
// is about what enters tier 0 and promote.toml about what leaves quarantine, and a draw
// is neither.
//
// **CompareSample exists now and did not, and the change is worth reading.** The
// original comment argued there was nothing to compare: a changed seed draws a
// different sample of the same size from the same population, so it is neither a
// loosening nor a tightening, and a comparison returning nil would be a mechanism
// asserting there was something to compare. That is still true of the seed. It stopped
// being true of the file when `critic_default` arrived, which has a direction — a
// smaller sample examines less — so the function reports that value and stays silent
// about the seed.
type Sample struct {
	// Seed makes a draw repeatable. §18.3 requires reproducibility and §6.2.1
	// requires the specific draw to be inspectable, which a value in the binary
	// would not be.
	Seed Value[uint64] `toml:"seed"`

	// CriticDefault is how many claims `gnosis critic` draws when `--sample` is
	// omitted (§10.5).
	//
	// Five, and the rationale in the file carries the mathematics: the median of a
	// population lies between the smallest and largest of any five random samples
	// with 93.75% probability. That reasoning does not depend on the corpus, which is
	// why nobody has to re-measure it here — and why a reader tempted to tune it
	// should know they are trading probability for prompts.
	CriticDefault Value[int] `toml:"critic_default"`
}

// DefaultSample returns the seed a new bundle begins with.
//
// Requires: nothing.
// Ensures: accepted by LoadSample — pinned by a test, because a seed its own loader
// rejects would break every bundle created from it. The returned slice is a copy,
// so a caller cannot corrupt the seed for the next one.
func DefaultSample() []byte {
	out := make([]byte, len(defaultSample))
	copy(out, defaultSample)
	return out
}

// LoadSample parses and validates the draw configuration.
//
// Requires: src is TOML.
// Ensures: EINVALID naming the problem — a syntax error, an unrecognised key, a value
// with no rationale, or a critic default that is not positive.
//
// **The seed has no range check and the sample size does.** Every seed is as good as
// every other, which is the whole point of a seed, and inventing a bound would be a
// threshold with nothing behind it. A draw of zero or fewer is different: it is a
// critic that examines nothing while reporting that it ran, which is the silence §10.5
// is written against.
func LoadSample(src []byte) (*Sample, error) {
	const op = "standards.LoadSample"

	var s Sample
	if err := decode(op, src, &s); err != nil {
		return nil, err
	}
	if err := checkRationales(op, &s); err != nil {
		return nil, err
	}
	if s.CriticDefault.Value <= 0 {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": critic_default must be positive; a draw of none is a" +
				" critic that examines nothing and reports that it ran",
		}
	}
	return &s, nil
}

// CompareSample reports whether old → cur shrank the critic's default draw.
//
// Requires: both are loaded.
// Ensures: at most one Loosening, and none for a changed seed — see the type's comment
// for why a seed has no direction.
//
// **A smaller sample examines less**, so smaller is looser. It is the same shape as
// `staleness_days` widening and carries the same hazard: a corpus can be made quieter
// by asking fewer questions, and every run afterwards is perfectly reproducible.
func CompareSample(old, cur *Sample) []Loosening {
	if old == nil || cur == nil ||
		cur.CriticDefault.Value >= old.CriticDefault.Value {
		return nil
	}
	return []Loosening{{
		Key:       "critic_default",
		From:      strconv.Itoa(old.CriticDefault.Value),
		To:        strconv.Itoa(cur.CriticDefault.Value),
		Rationale: cur.CriticDefault.Rationale,
	}}
}
