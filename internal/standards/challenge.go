package standards

import (
	_ "embed"
	"strconv"

	"github.com/StevenACoffman/skillet/errs"
)

// ChallengeFileName is where the challenge window lives, relative to the bundle root.
const ChallengeFileName = "standards/challenge.toml"

// defaultChallenge is the seed, embedded for the same reason the others are: its
// rationale carries what a reviewer reads before changing the number, and marshalling
// a Challenge back to TOML would drop it.
//
//go:embed challenge.toml
var defaultChallenge []byte

// Challenge is the window after which an unanswered challenge is reported (§10.7.3).
//
// One value, and a file of its own on sample.toml's reasoning: it belongs neither in
// archive.toml, which governs what enters tier 0, nor in promote.toml, which governs
// what leaves quarantine.
type Challenge struct {
	// UnansweredDays is how long a challenge may sit open before `lint` reports it.
	//
	// **The direction that loosens is up**, and that is declared in Go rather than in
	// the file for the reason §6.2 gives: a corpus can be made to lint clean by
	// widening a window, and a file that declared its own direction would let somebody
	// conceal that by flipping a field.
	UnansweredDays Value[int] `toml:"unanswered_days"`
}

// DefaultChallenge returns the window a new bundle begins with.
//
// Requires: nothing.
// Ensures: accepted by LoadChallenge — pinned by a test, because a seed its own loader
// rejects would break every bundle created from it. The returned slice is a copy.
func DefaultChallenge() []byte {
	out := make([]byte, len(defaultChallenge))
	copy(out, defaultChallenge)
	return out
}

// LoadChallenge parses and validates the challenge window.
//
// Requires: src is TOML.
// Ensures: EINVALID naming the problem — a syntax error, an unrecognised key, a value
// with no rationale, or a window that is not positive. A window of zero or less would
// report every challenge the moment it was filed, which is indistinguishable from
// having no window and teaches a reader to ignore the category.
func LoadChallenge(src []byte) (*Challenge, error) {
	const op = "standards.LoadChallenge"

	var c Challenge
	if err := decode(op, src, &c); err != nil {
		return nil, err
	}
	if err := checkRationales(op, &c); err != nil {
		return nil, err
	}
	if c.UnansweredDays.Value <= 0 {
		return nil, &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": unanswered_days must be positive; a window of zero" +
				" reports every challenge the moment it is filed",
		}
	}
	return &c, nil
}

// CompareChallenge reports whether old → cur widened the unanswered window.
//
// Requires: both are loaded.
// Ensures: at most one Loosening, and none when the window narrowed or only its
// rationale changed. A sibling of CompareArchive and ComparePromote, and separate for
// the same reason they are separate: each answers about one file, and a caller that
// wants all of them asks all of them.
//
// **A longer window reports fewer challenges**, so wider is looser — the same shape as
// `staleness_days`, and the same hazard: a corpus can be made to lint clean by giving
// everybody another month to answer.
func CompareChallenge(old, cur *Challenge) []Loosening {
	if old == nil || cur == nil || cur.UnansweredDays.Value <= old.UnansweredDays.Value {
		return nil
	}
	return []Loosening{{
		Key:       "unanswered_days",
		From:      strconv.Itoa(old.UnansweredDays.Value),
		To:        strconv.Itoa(cur.UnansweredDays.Value),
		Rationale: cur.UnansweredDays.Rationale,
	}}
}
