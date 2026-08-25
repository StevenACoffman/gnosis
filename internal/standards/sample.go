package standards

import (
	_ "embed"
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
// One value, and a file of its own. `rebuild_floor_fraction` was moved out of
// archive.toml on the grounds that one value did not earn a file; the difference
// here is that there is no existing file it belongs in — archive.toml is about what
// enters tier 0 and promote.toml about what leaves quarantine, and a seed is
// neither.
//
// **There is no CompareSample, deliberately.** A changed seed draws a different
// sample of the same size from the same population, so it is neither a loosening
// nor a tightening — there is no direction in which a seed reduces findings on
// purpose, which is the only thing §6.2's rule is about. What a change does mean is
// that results before and after it are not comparable, and that belongs in log.md
// for its own reason rather than as a loosening it is not. A comparison function
// returning nil would be a mechanism asserting there was something to compare.
type Sample struct {
	// Seed makes a draw repeatable. §18.3 requires reproducibility and §6.2.1
	// requires the specific draw to be inspectable, which a value in the binary
	// would not be.
	Seed Value[uint64] `toml:"seed"`
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
// Ensures: EINVALID naming the problem — a syntax error, an unrecognised key, or a
// value with no rationale. There is no range check: every seed is as good as every
// other, which is the whole point of a seed, and inventing a bound would be a
// threshold with nothing behind it.
func LoadSample(src []byte) (*Sample, error) {
	const op = "standards.LoadSample"

	var s Sample
	if err := decode(op, src, &s); err != nil {
		return nil, err
	}
	if err := checkRationales(op, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
