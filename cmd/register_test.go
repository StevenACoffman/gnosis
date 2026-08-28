package cmd_test

import (
	"io"
	"testing"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/gnosis/cmd"
	"github.com/StevenACoffman/gnosis/cmd/root"
)

// TestEveryCommandOwnsItsFlagSet is the check three occurrences of one defect earned.
//
// A command's Config embeds *root.Config, which has `Flags` and `Command`. A Config that
// does not declare its own assigns the **root's** through the same selector — the code
// reads correctly, compiles, and is wrong. It has happened three times: `admitcmd`'s
// `FromStdin` and the schema command's empty command list were wrong messages, and on
// 2026-08-27 a `synthesize` registration made **every** command require `--model`. Only a
// full-suite run caught the third, because nothing about `synthesize` looked broken.
//
// **The tree is walked, not listed.** `register` is what `Run` calls, so this test sees
// what ships; a test with its own list of commands would be a hand-maintained allowlist,
// which is the artifact this codebase has twice replaced after it fell behind.
//
// **The walk detects a cycle, and that is not defensive coding — it is the finding.**
// Registration is `parent.Command.Subcommands = append(…, cfg.Command)`, so a Config that
// shadows `Command` appends the root command to *its own* subcommand list and the tree
// then contains itself. Reintroducing the defect to check that this test fails is what
// surfaced it: the first version recursed forever and reported nothing, which is a worse
// outcome than the defect, because a hanging suite gets diagnosed as a flaky test.
func TestEveryCommandOwnsItsFlagSet(t *testing.T) {
	t.Parallel()

	r := root.New(nil, io.Discard, io.Discard)
	cmd.RegisterForTest(r)

	if len(r.Command.Subcommands) == 0 {
		t.Fatal("no commands registered; RegisterForTest no longer builds the tree")
	}

	visited := map[*ff.Command]bool{r.Command: true}
	names := map[string]bool{}
	var walk func(cmds []*ff.Command, path string)
	walk = func(cmds []*ff.Command, path string) {
		for _, sub := range cmds {
			name := path + sub.Name
			if visited[sub] {
				t.Errorf("%s appears twice in the tree; its Config is missing "+
					"`Command *ff.Command` and registered root.Config's own command "+
					"as a child of itself", name)
				continue
			}
			visited[sub] = true

			if names[name] {
				t.Errorf("two commands are both named %s", name)
			}
			names[name] = true

			if sub.Flags == r.Flags {
				t.Errorf("%s shares root.Config's flag set; declare "+
					"`Flags *ff.FlagSet` on its Config and assign "+
					"ff.NewFlagSet(%q).SetParent(parent.Flags)", name, sub.Name)
			}
			walk(sub.Subcommands, name+" ")
		}
	}
	walk(r.Command.Subcommands, "")
}

// TestEveryRunnableCommandSeesTheRootFlags is the other half, and it keeps the test above
// from being satisfiable by breaking the thing it protects.
//
// Every command inherits `--bundle` and `--jsonl` by parenting its own flag set to the
// root's. A command that owned a flag set with **no** parent would pass the shadowing
// check and silently lose both flags — the opposite failure, equally invisible, and one a
// reader would diagnose as a missing feature rather than as a wiring mistake.
//
// **Group parents are exempt because they run nothing.** `index` and `standards` declare
// no flag set of their own; ff supplies one at parse time, and until then the field is
// nil. Asserting on them would be asserting about a value that does not exist yet, and
// the first version of this test did exactly that and reported two commands as broken.
func TestEveryRunnableCommandSeesTheRootFlags(t *testing.T) {
	t.Parallel()

	r := root.New(nil, io.Discard, io.Discard)
	cmd.RegisterForTest(r)

	visited := map[*ff.Command]bool{r.Command: true}
	var walk func(cmds []*ff.Command, path string)
	walk = func(cmds []*ff.Command, path string) {
		for _, sub := range cmds {
			if visited[sub] {
				continue // reported by the test above; not this one's finding
			}
			visited[sub] = true

			name := path + sub.Name
			switch {
			case sub.Exec == nil:
				// A group parent dispatches and runs nothing.
			case sub.Flags == nil:
				t.Errorf("%s runs and has no flag set", name)
			default:
				if _, ok := sub.Flags.GetFlag("bundle"); !ok {
					t.Errorf("%s cannot see --bundle; its flag set was built without "+
						"`.SetParent(parent.Flags)`", name)
				}
			}
			walk(sub.Subcommands, name+" ")
		}
	}
	walk(r.Command.Subcommands, "")
}
