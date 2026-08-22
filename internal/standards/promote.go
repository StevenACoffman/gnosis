package standards

import (
	_ "embed"
)

// PromoteFileName is where the gate's thresholds live, relative to the bundle
// root.
const PromoteFileName = "standards/promote.toml"

// defaultPromote is the seed, embedded for the same reason the archive seed is:
// its comments carry the basis a reviewer reads before changing a number, and
// marshalling a Promote back to TOML would drop every one of them.
//
//go:embed promote.toml
var defaultPromote []byte

// Promote is the promote gate's tunable thresholds (§9.5).
//
// Separate from Archive because the two answer different questions and are edited
// by different people at different times: Archive decides what may enter tier 0,
// Promote decides what may leave quarantine. A single file would make a change to
// one look like a change to the other in every diff and every loosening report.
//
// A bundle may have one file and not the other, which is why they load
// independently rather than through a combined type.
type Promote struct {
	// HedgingMax is how many distinct softening phrases a body may carry before
	// the hedging signal fails it.
	//
	// It lived as a literal in the shell until now, which is precisely the
	// hardcoded threshold §6.5 forbids — and worse than most, because it had no
	// rationale attached and so no reader could tell whether three was measured or
	// guessed. It was guessed.
	HedgingMax Value[int] `toml:"hedging_max"`

	// RebuildFloorFraction is the share of the previously indexed document count
	// below which `index rebuild` refuses (§4.5).
	//
	// It lodged in archive.toml when the floor was built, because one value did
	// not earn a file. This is that file, and a rebuild is not an admission gate.
	RebuildFloorFraction Value[float64] `toml:"rebuild_floor_fraction"`
}

// DefaultPromote returns the thresholds a new bundle begins with.
//
// Requires: nothing.
// Ensures: accepted by LoadPromote — pinned by a test, because a seed its own
// loader rejects would break every bundle created from it. The returned slice is a
// copy, so a caller cannot corrupt the seed for the next one.
func DefaultPromote() []byte {
	out := make([]byte, len(defaultPromote))
	copy(out, defaultPromote)
	return out
}

// LoadPromote parses and validates the gate's thresholds.
//
// Requires: src is TOML.
// Ensures: EINVALID naming the problem — a syntax error, an unrecognised key, a
// value with no rationale, or a threshold outside the range in which it means
// anything. On success every value is populated and justified.
func LoadPromote(src []byte) (*Promote, error) {
	const op = "standards.LoadPromote"

	var p Promote
	if err := decode(op, src, &p); err != nil {
		return nil, err
	}
	if err := checkRationales(op, &p); err != nil {
		return nil, err
	}
	if err := p.validate(op); err != nil {
		return nil, err
	}
	return &p, nil
}
