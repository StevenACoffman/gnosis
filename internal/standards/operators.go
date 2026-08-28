package standards

import (
	_ "embed"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/skillet/errs"
)

// OperatorsFileName is where a corpus may override the operator patterns.
const OperatorsFileName = "standards/operators.toml"

// The operators a pattern may declare. There is deliberately no strict form: a
// constraint one step wider than its prose cannot manufacture a conflict the prose
// does not support, and the opposite error can (§10.2.2's noise argument).
const (
	OpAtMost  = "<="
	OpAtLeast = ">="
	OpExactly = "=="
)

// defaultOperators is the seed, embedded so its comments travel with it: they carry the
// inversion argument a reader needs before adding a phrase.
//
//go:embed operators.toml
var defaultOperators []byte

// Pattern is one phrasing that introduces a bounded quantity.
type Pattern struct {
	ID        string `toml:"id"`
	Phrase    string `toml:"phrase"`
	Op        string `toml:"op"`
	Rationale string `toml:"rationale"`
}

// Operators is the pattern set of SPEC §10.2.
type Operators struct {
	Pattern []Pattern `toml:"pattern"`
}

// DefaultOperators returns the seed a bundle uses when it declares none.
func DefaultOperators() []byte { return defaultOperators }

// LoadOperators parses an operator pattern set.
//
// Requires: src is TOML.
// Ensures: every pattern has an id, a lower-cased non-empty phrase, a known operator,
// and a rationale; ids and phrases are unique. Sorted longest-phrase-first, so a caller
// matching a prefix finds "no fewer than" before "fewer than" and cannot read a floor as
// a ceiling. Pure.
//
// **A rationale is required, as it is for every other `standards/` value** (§6.2). Here
// it carries more than a number's would: a phrase is a claim about how people write, and
// the reason it was added is the only thing that distinguishes a pattern somebody
// observed from one somebody imagined.
//
// **Duplicate phrases are refused rather than deduplicated.** Two rows claiming one
// phrase with different operators would make the reading depend on sort stability, and
// the failure — a floor read as a ceiling on some builds — is the worst kind.
func LoadOperators(src []byte) (*Operators, error) {
	const op = "standards.LoadOperators"

	var out Operators
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
	if err := validateOperators(op, &out); err != nil {
		return nil, err
	}
	// Longest first, so longest-wins is a property of the data rather than of the
	// order somebody wrote the rows in.
	sort.SliceStable(out.Pattern, func(i, j int) bool {
		return len(out.Pattern[i].Phrase) > len(out.Pattern[j].Phrase)
	})
	return &out, nil
}

// validateOperators refuses a pattern set that cannot be read unambiguously.
func validateOperators(op string, in *Operators) error {
	ids, phrases := map[string]bool{}, map[string]bool{}
	for i := range in.Pattern {
		p := &in.Pattern[i]
		p.Phrase = strings.ToLower(strings.TrimSpace(p.Phrase))
		switch {
		case strings.TrimSpace(p.ID) == "":
			return operatorProblem(op, "a pattern carries no id")
		case p.Phrase == "":
			return operatorProblem(op, "pattern "+p.ID+" carries no phrase")
		case p.Op != OpAtMost && p.Op != OpAtLeast && p.Op != OpExactly:
			return operatorProblem(op, "pattern "+p.ID+" has op "+p.Op+
				"; want <=, >= or ==")
		case strings.TrimSpace(p.Rationale) == "":
			return operatorProblem(op, "pattern "+p.ID+" carries no rationale; say why "+
				"this phrasing was added and what claim it read wrongly without it")
		case ids[p.ID]:
			return operatorProblem(op, "two patterns share the id "+p.ID)
		case phrases[p.Phrase]:
			return operatorProblem(op, "two patterns claim the phrase "+
				strconv.Quote(p.Phrase)+"; which operator applies would depend on sort "+
				"order, and a floor read as a ceiling is the worst failure here")
		}
		ids[p.ID], phrases[p.Phrase] = true, true
	}
	return nil
}

// operatorProblem builds the one error shape this loader returns.
func operatorProblem(op, why string) error {
	return &errs.Error{Code: errs.EINVALID, Op: op, Message: op + ": " + why}
}
