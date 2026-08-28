package standards

import (
	_ "embed"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/skillet/errs"
)

// LanguageFileName is where a corpus may override the language markers.
const LanguageFileName = "standards/language.toml"

// The language roles. §10.3's first column, split by what a reader would do about each.
const (
	RoleHedge      = "hedge"
	RoleWeasel     = "weasel"
	RoleComparison = "comparison"
	RoleAssurance  = "assurance"
)

// defaultLanguage is the seed, embedded so its §10.3 argument travels with it.
//
//go:embed language.toml
var defaultLanguage []byte

// LanguageMarker is one phrase and what it signals.
type LanguageMarker struct {
	Phrase    string `toml:"phrase"`
	Role      string `toml:"role"`
	Rationale string `toml:"rationale"`
}

// Language is the lexical half of §10.3.
type Language struct {
	Marker []LanguageMarker `toml:"marker"`
}

// DefaultLanguage returns the seed a bundle uses when it declares none.
func DefaultLanguage() []byte { return defaultLanguage }

// LoadLanguage parses a language marker set.
//
// Requires: src is TOML.
// Ensures: every phrase is lower-cased and non-empty with a known role and a rationale;
// sorted longest first, so a caller matching finds "significantly better" before any
// single word inside it. Pure.
func LoadLanguage(src []byte) (*Language, error) {
	const op = "standards.LoadLanguage"

	var out Language
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
		m := &out.Marker[i]
		m.Phrase = strings.ToLower(strings.TrimSpace(m.Phrase))
		switch {
		case m.Phrase == "":
			return nil, languageProblem(op, "a marker row carries no phrase")
		case !knownLanguageRole(m.Role):
			return nil, languageProblem(op, "marker "+m.Phrase+" has role "+m.Role+
				"; want hedge, weasel, comparison or assurance")
		case strings.TrimSpace(m.Rationale) == "":
			return nil, languageProblem(op, "marker "+m.Phrase+" carries no rationale")
		}
	}
	sort.SliceStable(out.Marker, func(i, j int) bool {
		return len(out.Marker[i].Phrase) > len(out.Marker[j].Phrase)
	})
	return &out, nil
}

// knownLanguageRole reports whether a role is one §10.3's first column names.
func knownLanguageRole(role string) bool {
	switch role {
	case RoleHedge, RoleWeasel, RoleComparison, RoleAssurance:
		return true
	default:
		return false
	}
}

// languageProblem builds the one error shape this loader returns.
func languageProblem(op, why string) error {
	return &errs.Error{Code: errs.EINVALID, Op: op, Message: op + ": " + why}
}
