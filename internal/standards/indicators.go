package standards

import (
	_ "embed"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/skillet/errs"
)

// IndicatorsFileName is where a corpus may override the indicator words.
const IndicatorsFileName = "standards/indicators.toml"

// The two roles an indicator may carry. There is deliberately no "unknown": a row
// with no role is one somebody half-wrote, and the loader refuses it rather than
// guessing.
//
//   - reason marks a clause giving a reason, a condition, or a concession — all
//     three make the clause depend on its neighbour, which is what a cut must not
//     break.
//   - conclusion marks a clause stating what follows. It has no reader yet; §17.4's
//     `lead` check is its consumer and waits on extraction.
const (
	RoleReason     Role = "reason"
	RoleConclusion Role = "conclusion"
)

// defaultIndicators is the seed, embedded for the same reason the other seeds are:
// its comments carry the argument a reader needs before changing the list, and
// marshalling a value back to TOML would drop every one of them.
//
//go:embed indicators.toml
var defaultIndicators []byte

// Role is what an indicator word marks about the clause it opens.
type Role string

// Indicator is one word or phrase and what it marks.
type Indicator struct {
	Word string `toml:"word"`
	Role Role   `toml:"role"`
}

// Indicators is the closed lexical class of SPEC §9.4.1.
type Indicators struct {
	Indicator []Indicator `toml:"indicator"`
}

// DefaultIndicators returns the seed a bundle uses when it declares none.
func DefaultIndicators() []byte { return defaultIndicators }

// LoadIndicators parses an indicator list.
//
// Requires: src is TOML.
// Ensures: every returned word is lower-cased and non-empty with a known role, or an
// EINVALID naming the offending row. Pure.
//
// **A row with no role is refused rather than defaulted.** Defaulting to `reason`
// would silently make a conclusion marker gate a cut, and the failure that produces —
// a sentence that stops being segmented — is invisible in the output. This is the
// same argument `EffectUnset` makes one layer up.
func LoadIndicators(src []byte) (*Indicators, error) {
	const op = "standards.LoadIndicators"

	var out Indicators
	md, err := toml.Decode(string(src), &out)
	if err != nil {
		return nil, &errs.Error{Code: errs.EINVALID, Op: op, Err: err}
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, &errs.Error{
			Code: errs.EINVALID, Op: op,
			Message: op + ": unknown key " + undecoded[0].String(),
		}
	}
	for i := range out.Indicator {
		row := &out.Indicator[i]
		row.Word = strings.ToLower(strings.TrimSpace(row.Word))
		if row.Word == "" {
			return nil, &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": an indicator row carries no word",
			}
		}
		if row.Role != RoleReason && row.Role != RoleConclusion {
			return nil, &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": indicator " + row.Word + " has role " +
					string(row.Role) + "; want reason or conclusion",
			}
		}
	}
	return &out, nil
}

// Words returns the indicators carrying one role, longest first.
//
// Requires: nothing.
// Ensures: sorted longest-first, so a caller matching a prefix finds "for the reason
// that" before "for" and never mistakes the long phrase for the short word. Never
// nil. Pure.
func (in *Indicators) Words(role Role) []string {
	out := make([]string, 0, len(in.Indicator))
	for i := range in.Indicator {
		if in.Indicator[i].Role == role {
			out = append(out, in.Indicator[i].Word)
		}
	}
	// Longest first. A plain lexical sort would put "so" before "some phrase" and
	// a prefix matcher would then stop at the shorter one.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && len(out[j]) > len(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
