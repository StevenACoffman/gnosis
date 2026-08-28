package standards

import (
	_ "embed"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/skillet/errs"
)

// RegistersFileName is where a corpus may override the causal registers.
const RegistersFileName = "standards/registers.toml"

// The causal registers. RegisterUnset is the zero value and names nothing, so a row
// nobody characterised is refused rather than read as the commoner case.
//
//   - intervention asserts that something makes something else happen.
//   - association asserts that two things move together, which is what observational
//     evidence supports. Declared so the check can tell a quotation that *observed*
//     from one that said nothing about causation either way — different states, and
//     only the second is silent.
//
// Pearl's third rung has no role here. A counterfactual has no lexical class this
// check could read, and a role with no reliable marker would report the corpus.
const (
	RegisterUnset        Register = ""
	RegisterIntervention Register = "intervention"
	RegisterAssociation  Register = "association"
)

// defaultRegisters is the seed, embedded for the same reason the other seeds are: its
// comments carry the argument a reader needs before changing the list.
//
//go:embed registers.toml
var defaultRegisters []byte

// Register is the causal rung a marker puts a statement on.
type Register string

// RegisterWord is one word or phrase and the rung it marks.
type RegisterWord struct {
	Word string   `toml:"word"`
	Role Register `toml:"role"`
}

// Registers is the lexical half of SPEC §17.3.1.1's rung check.
type Registers struct {
	Register []RegisterWord `toml:"register"`
}

// DefaultRegisters returns the seed a bundle uses when it declares none.
func DefaultRegisters() []byte { return defaultRegisters }

// LoadRegisters parses a causal-register list.
//
// Requires: src is TOML.
// Ensures: every returned word is lower-cased and non-empty with a known role, or an
// EINVALID naming the offending row. Pure.
//
// A row with no role is refused rather than defaulted, for the reason LoadStrengths
// gives one axis over: defaulting to `intervention` would make an observational phrase
// raise the evidence bar, and a claim reported for overclaiming *because* it was
// careful is the finding most likely to get a check switched off.
func LoadRegisters(src []byte) (*Registers, error) {
	const op = "standards.LoadRegisters"

	var out Registers
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
	for i := range out.Register {
		row := &out.Register[i]
		row.Word = strings.ToLower(strings.TrimSpace(row.Word))
		if row.Word == "" {
			return nil, &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": a register row carries no word",
			}
		}
		if row.Role != RegisterIntervention && row.Role != RegisterAssociation {
			return nil, &errs.Error{
				Code: errs.EINVALID, Op: op,
				Message: op + ": register " + row.Word + " has role " + string(row.Role) +
					"; want intervention or association",
			}
		}
	}
	return &out, nil
}

// Words returns the markers carrying one role, longest first.
//
// Requires: nothing.
// Ensures: sorted longest-first, so a caller matching a phrase finds "is associated
// with" before "associated with". Never nil. Pure.
func (r *Registers) Words(role Register) []string {
	out := make([]string, 0, len(r.Register))
	for i := range r.Register {
		if r.Register[i].Role == role {
			out = append(out, r.Register[i].Word)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}
