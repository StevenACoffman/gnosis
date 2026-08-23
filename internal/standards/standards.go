// Package standards loads the tunable thresholds as data, so a threshold change
// is a reviewable diff rather than a recompile (SPEC §6.5).
//
// Two properties distinguish this from ordinary configuration, and both come
// from SPEC §6.2.
//
// Every value carries its justification, structurally. A threshold is not a bare
// scalar here but a Value: the number and the reason it is that number. There is
// no way to express one without the other, which is the point — a rationale that
// were merely conventional would be the first thing dropped by whoever was in a
// hurry.
//
// The direction that loosens a knob is declared in Go, not in this file. That
// asymmetry is deliberate. `standards/` exists so two runs over one corpus agree,
// and that property is silent about whether the thresholds are any good: a corpus
// can be made to lint clean by widening a cap, and every run afterwards is
// perfectly reproducible and perfectly quiet. Compare reports which values moved
// in the finding-reducing direction — and if the file declared its own direction,
// concealing a loosening would take nothing more than flipping that field. Editing
// Go to hide it is a different diff, read by different reviewers.
//
// Everything here is pure. Load takes bytes; the caller reads the file.
package standards

import (
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/skillet/errs"
)

// Value is one tunable and the reason it holds that value.
//
// Rationale is required and checked at load. It answers a question the git diff
// cannot: whether a threshold was wrong or merely inconvenient.
type Value[T any] struct {
	Value     T      `toml:"value"`
	Rationale string `toml:"rationale"`
}

// justified is implemented by every Value, whatever it holds. The loader walks
// for this interface rather than for a field named "Rationale", so a value added
// to a standards file cannot escape the check by being a new type.
type justified interface {
	justification() string
}

func (v Value[T]) justification() string { return v.Rationale }

// decode parses strict TOML into dst.
//
// Requires: dst is a pointer to a struct mirroring the file's shape.
// Ensures: returns EINVALID naming the offending keys on a syntax error or on any
// key the decoder did not consume. An unconsumed key is almost always a typo, and
// ignoring it silently would leave a threshold the author believes they changed.
func decode(op string, src []byte, dst any) error {
	md, err := toml.Decode(string(src), dst)
	if err != nil {
		return &errs.Error{Code: errs.EINVALID, Message: op + ": " + err.Error()}
	}
	undecoded := md.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}
	keys := make([]string, 0, len(undecoded))
	for _, k := range undecoded {
		keys = append(keys, k.String())
	}
	sort.Strings(keys)
	return &errs.Error{
		Code:    errs.EINVALID,
		Message: op + ": unrecognised key(s): " + strings.Join(keys, ", "),
	}
}
