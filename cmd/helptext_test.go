package cmd_test

import (
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
	"github.com/StevenACoffman/gnosis/internal/lint"
)

// helpString is one user-facing string and where a reader would find it.
type helpString struct {
	// Where names the command and the field — "search ShortHelp" — so a failure says
	// which string to open rather than which package to search.
	Where string
	Text  string
}

// TestHelpTextNamesOnlyCommandsThatResolve is `lint`'s `command` check turned on the tool
// itself.
//
// That check asks whether *a bundle's* AGENTS.md names a command that resolves. Help text
// is not a corpus property: it ships with the binary, it is wrong at build time, and it is
// identical for every user — so a lint check would ask every corpus in the world to
// discover our typo. This is the same question asked where the answer is fixed.
//
// **It uses the same extractor as the check**, `lint.CommandsMentioned`, so the two texts
// cannot come to disagree about what a mention is.
//
// **The failure it guards is one this tool shipped.** A `search --claims` warning advised
// "run `gnosis extract`" — a command that has never existed — and running the binary is
// what caught it, not any test. An agent reading such an instruction does what it is told
// and cannot diagnose the result: it assumes its own invocation was wrong.
//
// There is no live instance today, so this is a guard before the case rather than a
// repair. Its own adversarial test is below.
func TestHelpTextNamesOnlyCommandsThatResolve(t *testing.T) {
	t.Parallel()

	r := root.New(nil, io.Discard, io.Discard)
	cmd.RegisterForTest(r)

	registered := registeredNames(r)
	if len(registered) == 0 {
		t.Fatal("no commands registered; this test would assert nothing")
	}

	for _, s := range helpTexts(r) {
		for _, name := range lint.CommandsMentioned(s.Text) {
			if !registered[name] {
				t.Errorf("%s names `gnosis %s`, which is not a registered command; "+
					"an agent told to run it cannot tell a stale instruction from its "+
					"own mistake", s.Where, name)
			}
		}
	}
}

// TestTheHelpTextScanFindsAStaleCommand is the adversarial case, and the one that matters
// for a guard with nothing to catch: a test that cannot fail is worse than no test,
// because it reports coverage it does not have.
func TestTheHelpTextScanFindsAStaleCommand(t *testing.T) {
	t.Parallel()

	r := root.New(nil, io.Discard, io.Discard)
	cmd.RegisterForTest(r)
	registered := registeredNames(r)

	const stale = "Claims with no lead are covered by `gnosis extract`."
	found := false
	for _, name := range lint.CommandsMentioned(stale) {
		if !registered[name] {
			found = true
		}
	}
	if !found {
		t.Error("a help string naming `gnosis extract` was not reported; the scan " +
			"above would pass on the defect it exists for")
	}
}

// TestOrdinaryProseIsNotAMention keeps the scan from being deleted for noise. Real help
// text in this repository says "gnosis cannot", "gnosis writes", "gnosis asked" — the tool
// as the subject of a sentence — and a bare word scan would report all of it.
func TestOrdinaryProseIsNotAMention(t *testing.T) {
	t.Parallel()

	const prose = "gnosis writes the region itself, and gnosis cannot tell a stale " +
		"instruction from a correct one."
	if got := lint.CommandsMentioned(prose); len(got) != 0 {
		t.Errorf("ordinary prose was read as naming commands: %v", got)
	}
}

// registeredNames is every subcommand name in the tree, at every depth.
//
// A subcommand of a subcommand is named as `gnosis index rebuild`, and the extractor
// reports the word after `gnosis` — so `index` and `rebuild` are both legitimate mentions
// and both belong here.
func registeredNames(r *root.Config) map[string]bool {
	out := map[string]bool{}
	var walk func(cmds []*ff.Command)
	walk = func(cmds []*ff.Command) {
		for _, sub := range cmds {
			if out[sub.Name] {
				continue // also guards the self-referential tree register_test.go names
			}
			out[sub.Name] = true
			walk(sub.Subcommands)
		}
	}
	walk(r.Command.Subcommands)
	return out
}

// helpTexts is every user-facing string the command tree carries.
//
// Usage, ShortHelp and LongHelp are the three a reader sees. **A sorted slice rather than
// a map**, so a run reporting several stale mentions prints them in the same order twice
// — the first version built a map, sorted its keys, and threw the sorted list away.
func helpTexts(r *root.Config) []helpString {
	out := []helpString{
		// The root's own help, which the walk below does not reach.
		{Where: "gnosis Usage", Text: r.Command.Usage},
		{Where: "gnosis ShortHelp", Text: r.Command.ShortHelp},
		{Where: "gnosis LongHelp", Text: r.Command.LongHelp},
	}
	seen := map[*ff.Command]bool{r.Command: true}
	var walk func(cmds []*ff.Command, path string)
	walk = func(cmds []*ff.Command, path string) {
		for _, sub := range cmds {
			if seen[sub] {
				continue // the self-referential tree register_test.go reports
			}
			seen[sub] = true
			name := strings.TrimSpace(path + " " + sub.Name)
			out = append(out,
				helpString{Where: name + " Usage", Text: sub.Usage},
				helpString{Where: name + " ShortHelp", Text: sub.ShortHelp},
				helpString{Where: name + " LongHelp", Text: sub.LongHelp},
			)
			walk(sub.Subcommands, name)
		}
	}
	walk(r.Command.Subcommands, "")

	sort.Slice(out, func(i, j int) bool { return out[i].Where < out[j].Where })
	return out
}
