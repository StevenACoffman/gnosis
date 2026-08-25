package scan

import (
	_ "embed"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/skillet/errs"
)

// The categories SPEC §9.3 names for stages 2 and 3.
//
// CategoryUnset is the zero value and names no category, so a Match nobody
// populated cannot be mistaken for a report about anything.
const (
	CategoryUnset Category = ""

	// CategoryPromptInjection is text addressed to whatever reads the document
	// next. In this system that is an agent holding the team's authority, which is
	// why a poisoned page is worse here than a poisoned page anywhere else.
	CategoryPromptInjection Category = "prompt_injection"

	// CategoryDataExfiltration is text that moves the reader's context somewhere
	// the reader does not control.
	CategoryDataExfiltration Category = "data_exfiltration"

	// CategoryMemoryPoisoning is a directive meant to outlast the session. §9.3
	// names it for exactly this system: a document in a corpus is durable, and it
	// arrives having passed review.
	CategoryMemoryPoisoning Category = "memory_poisoning"

	// CategoryToolMisuse is a complete invocation rather than a request for one —
	// text that needs no interpretation between being read and having happened.
	CategoryToolMisuse Category = "tool_misuse"

	// CategorySecret is a credential in a vendor-documented format. It is stage 3
	// rather than stage 2, and it is the one category where the finding is about
	// the source's author rather than about an attacker.
	CategorySecret Category = "secret"
)

// rules is the embedded ruleset, for the reason the ontology seed is embedded:
// its rationales are the part a reviewer reads before changing a pattern, and
// marshalling a Ruleset back to TOML would drop every one of them.
//
//go:embed rules.toml
var rules []byte

// Category names what kind of thing a rule looks for.
type Category string

// Match is one rule that fired on one text.
//
// One per rule rather than one per occurrence, matching what Hidden does for a
// character class: a document carrying nine copies of an injected instruction has
// one problem, and nine matches would bury it.
type Match struct {
	// Rule is the rule's id, so a refusal names something a reader can look up.
	Rule string `json:"rule"`

	// Category is the §9.3 class this rule belongs to.
	Category Category `json:"category"`

	// Offset is the byte offset of the first match, in the same space as
	// claims.pos and Finding.Offset (§5.5.2).
	Offset int `json:"offset"`
}

// Ruleset is a loaded, compiled, self-tested set of §9.3 stage 2 and 3 rules.
//
// It is immutable once loaded and holds only compiled patterns, which are safe for
// concurrent use — so one Ruleset is meant to be built by the shell and shared by
// every scan in the process, rather than rebuilt per document.
//
// The type exists so the patterns are a dependency the caller supplies rather than
// one the scanner reaches for. That is the same reason `archive.Gates` is a value:
// a scanner that loaded its own rules could not be tested against a ruleset of
// two, and the layering would put a decoder inside a pure function.
type Ruleset struct {
	compiled []compiledRule
}

// compiledRule is one rule and its pattern, ready to run.
type compiledRule struct {
	id       string
	category Category
	re       *regexp.Regexp
}

// rule mirrors one `[[rule]]` entry in rules.toml.
//
// MustFlag and MustNotFlag are not documentation. LoadRules runs them, and a rule
// that fails either takes the whole ruleset down — see LoadRules for why that is
// stricter than a test.
type rule struct {
	ID          string   `toml:"id"`
	Category    Category `toml:"category"`
	Pattern     string   `toml:"pattern"`
	Rationale   string   `toml:"rationale"`
	MustFlag    []string `toml:"must_flag"`
	MustNotFlag []string `toml:"must_not_flag"`
}

// ruleFile mirrors rules.toml as a whole.
type ruleFile struct {
	Rule []rule `toml:"rule"`
}

// Rules returns the ruleset compiled into this binary.
//
// Requires: nothing.
// Ensures: a ruleset that passed its own self-test, or an error — which for the
// embedded file means a build-time defect, and is pinned by a test so it cannot
// reach a user. Callers propagate it rather than ignoring it, because a scan
// running with no rules must not look like a scan that found nothing.
func Rules() (*Ruleset, error) {
	return LoadRules(rules)
}

// LoadRules parses, compiles, and self-tests a ruleset.
//
// Requires: src is TOML in rules.toml's shape.
// Ensures: EINVALID naming every problem it found — a syntax error, an
// unrecognised key, a rule missing a field, an uncompilable pattern, a rule with no
// examples, or a rule whose own examples it fails. On success every rule compiled
// and every rule demonstrated that it discriminates. Pure.
//
// **The self-test is at load rather than in a test, and that is the whole design.**
// §9.3's constants are allowed to block because they are not arguable; a *pattern*
// is arguable, so it has to earn the same standing some other way, and the way is
// that a pattern which cannot show it fires on the thing and not on the near-miss
// cannot be loaded at all. A test would catch the same defect one commit later and
// only for whoever ran it. This is the promote gate's planted-defect argument
// (§9.5) applied to a rule table: the check must be able to fail, demonstrably,
// every time it runs.
//
// It reports every failing rule rather than the first. A ruleset is edited as a
// set, and stopping at the first failure would hide the second from whoever is
// fixing the first.
func LoadRules(src []byte) (*Ruleset, error) {
	const op = "scan.LoadRules"

	var file ruleFile
	if err := decodeRules(op, src, &file); err != nil {
		return nil, err
	}
	if len(file.Rule) == 0 {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": the ruleset declares no rules",
		}
	}

	set := &Ruleset{compiled: make([]compiledRule, 0, len(file.Rule))}
	var problems []string
	seen := map[string]bool{}
	for i := range file.Rule {
		r := &file.Rule[i]
		compiled, why := compileRule(r, seen)
		if why != "" {
			problems = append(problems, why)
			continue
		}
		seen[r.ID] = true
		set.compiled = append(set.compiled, compiled)
	}
	if len(problems) > 0 {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + strings.Join(problems, "; "),
		}
	}
	sort.Slice(set.compiled, func(i, j int) bool {
		return set.compiled[i].id < set.compiled[j].id
	})
	return set, nil
}

// Patterns reports which rules fired on text.
//
// Requires: nothing. Empty text and invalid UTF-8 are both valid inputs.
// Ensures: at most one Match per rule, ordered by rule id so two runs over one
// text produce comparable output. Empty rather than nil when nothing fired, so a
// caller need not distinguish "no matches" from "did not run" — that distinction
// belongs to Coverage. Pure.
func (r *Ruleset) Patterns(text string) []Match {
	out := make([]Match, 0)
	if r == nil {
		return out
	}
	for i := range r.compiled {
		c := &r.compiled[i]
		if at := c.re.FindStringIndex(text); at != nil {
			out = append(out, Match{Rule: c.id, Category: c.category, Offset: at[0]})
		}
	}
	return out
}

// Runs reports whether this ruleset can perform §9.3's stages 2 and 3.
//
// Requires: nothing; a nil or empty Ruleset reports false, which is the truthful
// answer for a caller that could not load rules.
// Ensures: pure. It is what a caller passes to CoverageOf, so the coverage it
// reports is derived from what it holds rather than from what it intended.
func (r *Ruleset) Runs() bool { return r != nil && len(r.compiled) > 0 }

// Describe renders a scan's findings as one line each.
//
// Requires: hidden and matched came from scanning one text.
// Ensures: hidden-character classes first and pattern matches second, each in the
// order its own scanner produced — which is sorted, so two scans of one text
// describe it identically. Empty rather than nil for a clean scan. Pure.
//
// **One renderer, and that is the point.** The promote gate needs these as strings
// for its `security` signal, and `fetch` needs them to explain a refused source. A
// second renderer would let the two describe one problem two ways: an author who saw
// `injection-pattern` from `fetch` and something else from `promote` would have to
// work out that they were the same finding. This is the Repetition red flag with a
// consequence attached, so there is one function.
func Describe(hidden []Finding, matched []Match) []string {
	out := make([]string, 0, len(hidden)+len(matched))
	for _, f := range hidden {
		out = append(out, string(f.Class)+" "+f.Rune+
			" ×"+strconv.Itoa(f.Count)+" at byte "+strconv.Itoa(f.Offset))
	}
	for _, m := range matched {
		out = append(out, string(m.Category)+" "+m.Rule+
			" at byte "+strconv.Itoa(m.Offset))
	}
	return out
}

// compileRule validates one rule and compiles its pattern.
//
// Requires: seen holds the ids already accepted.
// Ensures: an empty reason and a usable compiledRule, or a reason naming what is
// wrong with this rule. It never returns both.
func compileRule(r *rule, seen map[string]bool) (compiledRule, string) {
	switch {
	case strings.TrimSpace(r.ID) == "":
		return compiledRule{}, "a rule declares no id"
	case seen[r.ID]:
		return compiledRule{}, r.ID + " is declared twice"
	case r.Category == CategoryUnset:
		return compiledRule{}, r.ID + " declares no category"
	case strings.TrimSpace(r.Rationale) == "":
		return compiledRule{}, r.ID + " has no rationale"
	case len(r.MustFlag) == 0:
		return compiledRule{}, r.ID + " declares no must_flag case; a rule that " +
			"has never been shown to fire is a rule nobody can trust"
	case len(r.MustNotFlag) == 0:
		return compiledRule{}, r.ID + " declares no must_not_flag case; without " +
			"one there is nothing stopping the pattern being widened until it " +
			"fires on everything"
	}

	re, err := regexp.Compile(r.Pattern)
	if err != nil {
		return compiledRule{}, r.ID + " has an uncompilable pattern: " + err.Error()
	}
	if why := selfTest(r, re); why != "" {
		return compiledRule{}, why
	}
	return compiledRule{id: r.ID, category: r.Category, re: re}, ""
}

// selfTest runs a rule's own examples against its own pattern.
//
// Requires: re is r's compiled pattern.
// Ensures: an empty string when every must_flag matches and no must_not_flag does;
// otherwise a reason naming the rule and the example it got wrong.
//
// Own-pattern only. Whether one rule's negative example is caught by a *different*
// rule is a real question — it is the cross-rule false-positive question — and it
// is asked by a test rather than here, because the answer may legitimately be yes
// and the fix would then be to choose a better example rather than to refuse the
// ruleset.
func selfTest(r *rule, re *regexp.Regexp) string {
	for _, example := range r.MustFlag {
		if !re.MatchString(example) {
			return r.ID + " does not flag its own must_flag case: " + quote(example)
		}
	}
	for _, example := range r.MustNotFlag {
		if re.MatchString(example) {
			return r.ID + " flags its own must_not_flag case: " + quote(example)
		}
	}
	return ""
}

// decodeRules parses strict TOML into dst.
//
// Strict for the reason §5.2 gives: `toml.Decode` reports MetaData.Undecoded, so a
// mistyped `patern` is an error with a key name rather than a rule that silently
// matches nothing. A rule that matches nothing is the worst failure available here,
// because the scan still reports the stage as having run.
func decodeRules(op string, src []byte, dst *ruleFile) error {
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

// quote renders an example in a diagnostic, bounded so one long fixture cannot
// make the message unreadable.
func quote(s string) string {
	const width = 60
	if len(s) > width {
		s = s[:width] + "…"
	}
	return `"` + strings.ReplaceAll(s, "\n", `\n`) + `"`
}
