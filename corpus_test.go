package main

import (
	"os"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/gnosis/internal/scan"
)

// rulesFile is §9.3's pattern set, relative to this package's directory.
const rulesFile = "internal/scan/rules.toml"

// longFormPages are this repository's own prose documents, read from this package's
// directory the way spec_test.go reads SPEC.md.
//
// They are here because they are the only **real, page-shaped bytes** the repository
// holds: hundreds of kilobytes of markdown with headings, tables, fenced code, inline
// regexes, URLs, and long lines. §9.3's rules are applied to exactly that — a whole
// fetched document — and every case beside them is a single hand-written sentence.
var longFormPages = []string{"SPEC.md", "PLAN.md", "TODO.md", "README.md", "manifesto.md"}

// ruleCase is one rule's id and the sentences it must flag, as the artifact states them.
type ruleCase struct {
	ID       string   `toml:"id"`
	MustFlag []string `toml:"must_flag"`
}

// TestOrdinaryLongFormProseIsNotFlagged is §18.4.1 applied to the scan rules, and it is
// the case their own corpus cannot supply.
//
// Each rule carries a `must_flag` and a `must_not_flag` example and the loader refuses a
// rule failing either, which is the strongest self-test in this codebase. But both are
// **sentences**, and `Ruleset.Patterns` receives a *page*. That is precisely the gap
// §18.4.1 was written about: the operator corpus shipped thirteen patterns and eight
// green cases while the commonest real input — a sentence rather than a phrase — failed
// silently. A green corpus is not coverage of the input space.
//
// **A match here has two readings and the message gives both**, because they need
// opposite responses. Either a rule is too broad and fires on ordinary technical prose —
// in which case its pattern needs narrowing and its `must_not_flag` needs this sentence —
// or one of these documents genuinely contains a specimen, in which case it belongs in
// testdata rather than in prose an agent may be handed.
//
// Measured 2026-08-27: zero matches and zero hidden-character findings across ~1MB.
func TestOrdinaryLongFormProseIsNotFlagged(t *testing.T) {
	t.Parallel()

	set, err := scan.Rules()
	if err != nil {
		t.Fatalf("the shipped ruleset does not load: %v", err)
	}
	if !set.Runs() {
		t.Fatal("the ruleset compiled no patterns; this test would assert nothing")
	}

	read := 0
	for _, name := range longFormPages {
		body, err := os.ReadFile(name)
		if err != nil {
			// A document renamed or removed is not this test's finding, but a run
			// that silently read nothing would pass while asserting nothing.
			t.Logf("%s: %v", name, err)
			continue
		}
		read++
		text := string(body)

		for _, m := range set.Patterns(text) {
			t.Errorf("%s: rule %v fired on ordinary prose; either narrow the pattern "+
				"and add the sentence to its must_not_flag, or move the specimen into "+
				"testdata so the corpus does not carry text a scan must flag", name, m)
		}
		for _, f := range scan.Hidden(text) {
			t.Errorf("%s: hidden character %v; a bidi override or zero-width run in a "+
				"document an agent reads is §9.3's first stage finding its own corpus",
				name, f)
		}
	}
	if read == 0 {
		t.Fatal("no long-form documents were read; the corpus moved and this test " +
			"would pass on nothing")
	}
}

// TestTheRulesAreExercisedByThisCorpus keeps the test above from being vacuous.
//
// Zero matches over a megabyte proves the rules are quiet on real prose only if they can
// fire at all on text of that shape. A ruleset that had silently compiled to nothing, or
// a pattern that only matches near a string boundary, would produce the same clean result
// — and "no findings" would then mean "no scan", which is the one confusion §9.3's
// coverage type exists to prevent.
//
// So each rule's own `must_flag` sentence is embedded **inside** a real page and must
// still be found there. Nothing here is invented: the sentence is the rule's own and the
// surrounding bytes are this repository's.
//
// **The examples are read from the artifact rather than through an accessor.** Adding
// `IDs()` and `MustFlagFor()` to Ruleset would be production API existing only for a
// test, and §18.4.1's own instruction is to take the cases from the artifact — so the
// test decodes the file, which is the artifact.
func TestTheRulesAreExercisedByThisCorpus(t *testing.T) {
	t.Parallel()

	set, err := scan.Rules()
	if err != nil {
		t.Fatalf("the shipped ruleset does not load: %v", err)
	}
	page, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	for _, r := range mustFlagCases(t) {
		if len(r.MustFlag) == 0 {
			t.Errorf("rule %s carries no must_flag case", r.ID)
			continue
		}
		for _, example := range r.MustFlag {
			// Mid-document, so a pattern that depends on the input being short
			// fails here rather than passing on a one-sentence case forever.
			buried := string(page) + "\n\n" + example + "\n\n" + string(page)
			if !fires(set, buried, r.ID) {
				t.Errorf("rule %s flags its own example as a lone sentence and misses "+
					"it inside a page; the pattern depends on the input being short",
					r.ID)
			}
		}
	}
}

// mustFlagCases reads the rules straight out of §9.3's file.
func mustFlagCases(t *testing.T) []ruleCase {
	t.Helper()

	var file struct {
		Rule []ruleCase `toml:"rule"`
	}
	if _, err := toml.DecodeFile(rulesFile, &file); err != nil {
		t.Fatalf("decode %s: %v", rulesFile, err)
	}
	if len(file.Rule) == 0 {
		t.Fatalf("%s declares no rules; this test would assert nothing", rulesFile)
	}
	return file.Rule
}

// fires reports whether the named rule matched text.
func fires(set *scan.Ruleset, text, id string) bool {
	for _, m := range set.Patterns(text) {
		if m.Rule == id {
			return true
		}
	}
	return false
}
