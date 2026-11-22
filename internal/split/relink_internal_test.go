package split

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// book is three chapters each large enough to be its own part, with one written into
// the first.
func book(one string) string {
	return "# Book\n\n" +
		"## Alpha\n\n" + one + para(2800) +
		"## Beta\n\n" + para(2800) +
		"## Gamma\n\n" + para(2800)
}

func partNamed(t *testing.T, plan *Plan, suffix string) Part {
	t.Helper()
	for i := range plan.Parts {
		if strings.HasSuffix(plan.Parts[i].Name, suffix) {
			return plan.Parts[i]
		}
	}
	t.Fatalf("no part ending %s in %v", suffix, names(plan))
	return Part{}
}

func TestALinkToAnotherPartPointsAtThatPart(t *testing.T) {
	plan := mustSplit(t, book("See [Gamma](#gamma).\n\n"), Options{})
	alpha := partNamed(t, plan, "-01-book.md")
	gamma := partNamed(t, plan, "-gamma.md")
	if !bytes.Contains(alpha.Content, []byte("[Gamma]("+gamma.Name+"#gamma)")) {
		t.Errorf("the link was not pointed at %s:\n%s", gamma.Name, alpha.Content[:400])
	}
	if alpha.Relinked != 1 || !bytes.Contains(alpha.Content, []byte("relinked_links: 1\n")) {
		t.Errorf("relinked = %d, want 1 and recorded in the frontmatter", alpha.Relinked)
	}
}

func TestALinkInsideCodeIsLeftAlone(t *testing.T) {
	plan := mustSplit(
		t,
		book("Write `[x](#gamma)` or:\n\n```md\n[x](#gamma)\n```\n\n"),
		Options{},
	)
	alpha := partNamed(t, plan, "-01-book.md")
	if alpha.Relinked != 0 || bytes.Contains(alpha.Content, []byte(".md#gamma")) {
		t.Errorf("a link written as code was rewritten:\n%s", alpha.Content[:500])
	}
}

func TestAFragmentNamingNoHeadingIsLeftAndListed(t *testing.T) {
	plan := mustSplit(t, book("See [nowhere](#no-such-heading).\n\n"), Options{})
	alpha := partNamed(t, plan, "-01-book.md")
	if !bytes.Contains(alpha.Content, []byte("[nowhere](#no-such-heading)")) {
		t.Error("an unresolved fragment was changed")
	}
	if strings.Join(alpha.Unresolved, ",") != "#no-such-heading" {
		t.Errorf("unresolved = %v, want [#no-such-heading]", alpha.Unresolved)
	}
}

func TestARepeatedHeadingKeepsTheRightAnchorInItsPart(t *testing.T) {
	// The book's second "Notes" is #notes-1; in its own part it is the first, #notes.
	src := "# Book\n\n" +
		"## Alpha\n\n### Notes\n\n" + para(2800) +
		"## Beta\n\nBack to [the second notes](#notes-1).\n\n### Notes\n\n" + para(2800)
	plan := mustSplit(t, src, Options{})
	beta := partNamed(t, plan, "-beta.md")
	if !bytes.Contains(beta.Content, []byte("[the second notes](#notes)")) {
		t.Errorf("the same-part link was not renumbered:\n%s", beta.Content[:500])
	}
}

func TestADefinitionPointingAtAnotherPartIsRelinked(t *testing.T) {
	plan := mustSplit(t, book("See [the end][end].\n\n[end]: #gamma\n\n"), Options{})
	alpha := partNamed(t, plan, "-01-book.md")
	gamma := partNamed(t, plan, "-gamma.md")
	if !bytes.Contains(alpha.Content, []byte("[end]: "+gamma.Name+"#gamma")) {
		t.Errorf("the definition was not relinked:\n%s", alpha.Content[:500])
	}
}

func TestVerifyRefusesAnUnrecordedChange(t *testing.T) {
	plan := mustSplit(t, book("See [Gamma](#gamma).\n\n"), Options{})
	parts := plan.Parts
	alpha := &parts[0]
	alpha.Content = bytes.Replace(alpha.Content, []byte("See "), []byte("Saw "), 1)
	src := []byte(book("See [Gamma](#gamma).\n\n"))
	if err := verify(src, parts); err == nil {
		t.Fatal("a change outside the recorded rewrites was accepted")
	}
	alpha.rewrites = append(alpha.rewrites, rewrite{At: 0, Old: "#x", New: "elsewhere.md#x"})
	if err := verify(src, parts); err == nil || errors.Is(err, ErrRenderChanged) {
		t.Fatalf("err = %v, want a refusal of the bogus rewrite", err)
	}
}

func TestSluggerMatchesGitHub(t *testing.T) {
	s := slugger{}
	for _, c := range []struct{ title, want string }{
		{"Chapter 4: How Go Uses the CPU Resource (or Two)", "chapter-4-how-go-uses-the-cpu-resource-or-two"},
		{"What’s new?", "whats-new"},
		{"Summary", "summary"},
		{"Summary", "summary-1"},
		{"Summary 1", "summary-1-1"},
		{"`go test` -run", "go-test--run"},
	} {
		if got := s.next(c.title); got != c.want {
			t.Errorf("next(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}
