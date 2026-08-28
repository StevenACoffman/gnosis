package standards

import (
	_ "embed"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/skillet/errs"
)

// StrengthFileName is where a corpus may override the claim-strength markers.
const StrengthFileName = "standards/strength.toml"

// The strength roles. StrengthUnset is the zero value and names nothing, so a row
// nobody characterised is refused rather than read as the commoner case.
//
//   - universal asserts without exception, and raises what the evidence must carry.
//   - hedged asserts with one, and lowers it. Hedged words are declared so the check
//     can tell a claim that hedged from one that said nothing either way — different
//     states, and only the second is silent.
const (
	StrengthUnset     Strength = ""
	StrengthUniversal Strength = "universal"
	StrengthHedged    Strength = "hedged"
)

// defaultStrength is the seed, embedded for the same reason the other seeds are: its
// comments carry the argument a reader needs before changing the list.
//
//go:embed strength.toml
var defaultStrength []byte

// Strength is how strongly a marker makes a claim assert.
type Strength string

// Marker is one word or phrase and how strongly it asserts.
type Marker struct {
	Word string   `toml:"word"`
	Role Strength `toml:"role"`
}

// Strengths is the lexical half of SPEC §17.3.1's sufficiency rule.
type Strengths struct {
	Marker []Marker `toml:"marker"`
}

// DefaultStrengths returns the seed a bundle uses when it declares none.
func DefaultStrengths() []byte { return defaultStrength }

// LoadStrengths parses a claim-strength list.
//
// Requires: src is TOML.
// Ensures: every returned word is lower-cased and non-empty with a known role, or an
// EINVALID naming the offending row. Pure.
//
// A row with no role is refused rather than defaulted. Defaulting to universal would
// make a hedge raise the evidence bar, and the finding that produces — a claim reported
// for over-asserting *because* it was careful — is the one most likely to get the check
// switched off.
func LoadStrengths(src []byte) (*Strengths, error) {
	const op = "standards.LoadStrengths"

	var out Strengths
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
	for i := range out.Marker {
		row := &out.Marker[i]
		row.Word = strings.ToLower(strings.TrimSpace(row.Word))
		if row.Word == "" {
			return nil, &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": a marker row carries no word",
			}
		}
		if row.Role != StrengthUniversal && row.Role != StrengthHedged {
			return nil, &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": marker " + row.Word + " has role " + string(row.Role) +
					"; want universal or hedged",
			}
		}
	}
	return &out, nil
}

// Words returns the markers carrying one role, longest first.
//
// Requires: nothing.
// Ensures: sorted longest-first, so a caller matching a phrase finds "must not" before
// "must" and "in most cases" before any single word inside it. Never nil. Pure.
func (s *Strengths) Words(role Strength) []string {
	out := make([]string, 0, len(s.Marker))
	for i := range s.Marker {
		if s.Marker[i].Role == role {
			out = append(out, s.Marker[i].Word)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && len(out[j]) > len(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
