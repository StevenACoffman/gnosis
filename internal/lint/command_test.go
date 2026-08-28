package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/lint"
)

// agentsDoc builds a snapshot around one AGENTS.md text.
func agentsDoc(text string) *lint.Snapshot {
	return &lint.Snapshot{
		SchemaDoc: text,
		Commands:  []string{"lint", "schema", "search"},
	}
}

// TestTheGeneratedRegionIsNeverTheFinding is the adversarial case, and it is what makes
// this check non-redundant rather than a second `gnosis schema --check`.
//
// The command region is written *from* the registry, so a name inside it resolves by
// construction and a region that has fallen behind is already reported elsewhere. If this
// check read the region it would either never fire or duplicate that finding. It must
// read only the prose §5.7.1 guarantees gnosis never touches.
func TestTheGeneratedRegionIsNeverTheFinding(t *testing.T) {
	t.Parallel()

	// A region naming a command that does not resolve — which cannot happen in
	// practice, and must be ignored here even so.
	const doc = "# Agents\n\n<!-- gnosis:begin commands -->\n" +
		"- `gnosis frobnicate` — a command that does not exist\n" +
		"<!-- gnosis:end commands -->\n\nRun `gnosis lint` to check.\n"

	if got := runNamed(t, agentsDoc(doc), "command"); len(got) != 0 {
		t.Errorf("a name inside the generated region was reported:\n%s",
			strings.Join(got, "\n"))
	}
}

// TestProseNamingAnUnresolvableCommandIsReported is the check: somebody's own notes have
// gone stale, and an agent reading them will run what it is told.
func TestProseNamingAnUnresolvableCommandIsReported(t *testing.T) {
	t.Parallel()
	const doc = "# Agents\n\n## My notes\n\nRun `gnosis frobnicate` first, then `gnosis lint`.\n"

	got := runNamed(t, agentsDoc(doc), "command")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}
	for _, want := range []string{"frobnicate", "your own prose", "cannot tell"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not carry %q:\n%s", want, got[0])
		}
	}
	if strings.Contains(got[0], "lint") {
		t.Errorf("a resolvable command was reported:\n%s", got[0])
	}
}

// TestThreeStaleInstructionsAreOneFinding keeps the report about the document. A page
// with three stale instructions has one problem — nobody has revisited it — and three
// findings would make the report about the words.
func TestThreeStaleInstructionsAreOneFinding(t *testing.T) {
	t.Parallel()
	const doc = "Run `gnosis alpha`, then `gnosis beta`, then `gnosis gamma`.\n"
	got := runNamed(t, agentsDoc(doc), "command")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %d", len(got))
	}
	for _, want := range []string{"alpha", "beta", "gamma", "3 unresolvable commands"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the finding does not carry %q:\n%s", want, got[0])
		}
	}
}

// TestABareBinaryNameIsNotACommand keeps `gnosis` and `gnosis --jsonl` out of it: the
// first names the binary and the second names a root flag, and neither is a subcommand
// that could fail to resolve.
//
// The check **skips** rather than running and finding nothing, and that is the honest
// answer: a document naming no command has nothing for this to resolve, and "no
// unresolvable commands" would be a clean bill for a question that was not asked. The
// distinction is the one §12 makes everywhere, and it is why this asserts the skip.
func TestABareBinaryNameIsNotACommand(t *testing.T) {
	t.Parallel()
	const doc = "Install `gnosis`, then run `gnosis --jsonl` for machine output.\n"
	reason := skipReason(t, agentsDoc(doc), "command")
	if !strings.Contains(reason, "names no gnosis command") {
		t.Errorf("the skip does not say why: %q", reason)
	}
}

// TestNoSchemaDocumentSkipsRatherThanPasses is the absence case: a bundle that has not
// run `gnosis schema` has no document to check, and saying it is clean would be a
// statement about nothing.
func TestNoSchemaDocumentSkipsRatherThanPasses(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{Commands: []string{"lint"}}
	if reason := skipReason(t, snap, "command"); !strings.Contains(reason, "AGENTS.md") {
		t.Errorf("the skip does not name the missing document: %q", reason)
	}
}

// TestNoCommandListSkipsRatherThanCondemns is the absence-of-the-ruler case, and it
// matters because the list is filled by the command layer rather than by `bundle`: a
// caller that forgot would otherwise have every command in its own document reported as
// unresolvable.
func TestNoCommandListSkipsRatherThanCondemns(t *testing.T) {
	t.Parallel()
	snap := &lint.Snapshot{SchemaDoc: "Run `gnosis lint`.\n"}
	if reason := skipReason(t, snap, "command"); !strings.Contains(reason, "no command list") {
		t.Errorf("the skip does not say the list is missing: %q", reason)
	}
}
