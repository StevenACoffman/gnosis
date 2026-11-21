package split

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// limit is large enough to hold the frontmatter reserve plus a few small sections.
const limit = frontmatterReserve + 3000

// para returns n bytes of paragraph text ending in a blank line, so tests can size
// sections precisely without the content mattering.
func para(n int) string {
	line := strings.Repeat("word ", 15) + "\n"
	var b strings.Builder
	for b.Len() < n {
		b.WriteString(line)
	}
	b.WriteString("\n")
	return b.String()
}

func mustSplit(t *testing.T, src string, opt Options) *Plan {
	t.Helper()
	if opt.Source == "" {
		opt.Source = "book.md"
	}
	if opt.Limit == 0 {
		opt.Limit = limit
	}
	plan, err := Split([]byte(src), opt)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	return plan
}

func names(plan *Plan) []string {
	out := make([]string, 0, len(plan.Parts))
	for _, p := range plan.Parts {
		out = append(out, p.Name)
	}
	return out
}

func TestASourceUnderTheLimitIsNotSplit(t *testing.T) {
	plan := mustSplit(t, "# Title\n\n"+para(100), Options{})
	if len(plan.Parts) != 0 {
		t.Fatalf("got %d parts for a source under the limit, want 0", len(plan.Parts))
	}
	if plan.SHA256 == "" {
		t.Error("the digest is recorded even when nothing is split")
	}
}

func TestChaptersAreCutAndTheTrailNamesTheBook(t *testing.T) {
	src := "# Book\n\nfront\n\n" +
		"## One\n\n" + para(2000) +
		"## Two\n\n" + para(2000) +
		"## Three\n\n" + para(2000)
	plan := mustSplit(t, src, Options{})

	want := []string{"book-01-book.md", "book-02-two.md", "book-03-three.md"}
	if got := names(plan); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf(
			"names = %v, want %v (the lead joins the first chapter and names the part)",
			got,
			want,
		)
	}
	if got := plan.Parts[1].Headings; strings.Join(got, "/") != "Book/Two" {
		t.Errorf("trail = %v, want [Book Two]", got)
	}
	if plan.Parts[0].Start != 0 || plan.Parts[len(plan.Parts)-1].End != len(src) {
		t.Error("the parts do not span the source")
	}
}

func TestAnOversizeChapterIsCutAtItsSections(t *testing.T) {
	src := "# Book\n\n" +
		"## One\n\n" + para(500) +
		"## Two\n\n" + para(200) +
		"### Two A\n\n" + para(2500) +
		"### Two B\n\n" + para(2500)
	plan := mustSplit(t, src, Options{})

	var trails []string
	for _, p := range plan.Parts {
		trails = append(trails, strings.Join(p.Headings, "/"))
	}
	got := strings.Join(trails, " | ")
	if !strings.Contains(got, "Book/Two | ") || !strings.Contains(got, "Book/Two/Two B") {
		t.Fatalf("trails = %s; Two should have been cut, its lead joined to Two A", got)
	}
	if len(plan.Oversize) != 0 {
		t.Errorf("oversize = %v, want none", plan.Oversize)
	}
}

func TestAHeadingInsideAFenceIsNotACutPoint(t *testing.T) {
	src := "# Book\n\n" +
		"## One\n\n" + para(1500) + "```sh\n# not a heading\n```\n\n" + para(1500) +
		"## Two\n\n" + para(1500)
	plan := mustSplit(t, src, Options{})
	for _, p := range plan.Parts {
		if strings.Contains(p.Name, "not-a-heading") {
			t.Fatalf("cut at a fenced comment: %v", names(plan))
		}
	}
}

func TestASetextHeadingIsACutPoint(t *testing.T) {
	src := "Book\n====\n\n" +
		"One\n---\n\n" + para(2800) +
		"Two\n---\n\n" + para(2800)
	plan := mustSplit(t, src, Options{})
	if got := names(plan); len(got) != 2 || !strings.HasSuffix(got[1], "-two.md") {
		t.Fatalf("names = %v, want a part per setext section", got)
	}
}

func TestADefinitionUsedInAnotherPartIsCopiedThere(t *testing.T) {
	src := "# Book\n\n" +
		"## One\n\n" + "See [the spec][spec].\n\n[spec]: https://example.org/spec \"Spec\"\n\n" +
		para(2500) +
		"## Two\n\n" + "Again, [the spec][spec].\n\n" + para(2500)
	plan := mustSplit(t, src, Options{})
	if len(plan.Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(plan.Parts))
	}
	two := plan.Parts[1]
	if strings.Join(two.Copied, ",") != "spec" {
		t.Errorf("copied = %v, want [spec]", two.Copied)
	}
	if !bytes.Contains(two.Content, []byte("[spec]: https://example.org/spec \"Spec\"\n")) {
		t.Error("the definition was not copied verbatim")
	}
	if len(plan.Parts[0].Copied) != 0 {
		t.Error("the part holding the definition should not copy it")
	}
}

func TestLevelCutsChaptersThatWouldFitTogether(t *testing.T) {
	src := "# Book\n\n## One\n\n" + para(100) + "## Two\n\n" + para(100)
	plan := mustSplit(t, src, Options{Level: 2})
	if got := names(plan); len(got) != 3 {
		t.Fatalf("names = %v, want the lead and one part per chapter", got)
	}
}

func TestAStretchWithNoHeadingIsPlannedAndReportedOversize(t *testing.T) {
	src := "# Book\n\n## One\n\n" + para(limit+500) + "## Two\n\n" + para(100)
	plan := mustSplit(t, src, Options{})
	if len(plan.Oversize) != 1 || !strings.HasSuffix(plan.Oversize[0], "-one.md") {
		t.Fatalf("oversize = %v, want the one heading-less chapter", plan.Oversize)
	}
}

func TestFrontmatterInTheSourceIsRefused(t *testing.T) {
	_, err := Split([]byte("---\ntitle: x\n---\n\n# Book\n"), Options{Source: "b.md", Limit: 10})
	if !errors.Is(err, ErrFrontmatter) {
		t.Fatalf("err = %v, want ErrFrontmatter", err)
	}
}

func TestSplittingIsDeterministic(t *testing.T) {
	src := "# Book\n\n## One\n\n" + para(2000) + "## Two\n\n" + para(2000) +
		"## Three\n\n" + para(2000)
	a := mustSplit(t, src, Options{})
	b := mustSplit(t, src, Options{})
	if len(a.Parts) != len(b.Parts) {
		t.Fatal("two runs planned different part counts")
	}
	for i := range a.Parts {
		if a.Parts[i].Name != b.Parts[i].Name ||
			!bytes.Equal(a.Parts[i].Content, b.Parts[i].Content) {
			t.Fatalf("part %d differs between runs", i+1)
		}
	}
}

func TestEveryPartRecordsWhereItCameFrom(t *testing.T) {
	src := "# Book\n\n## One\n\n" + para(2800) + "## Two\n\n" + para(2800)
	plan := mustSplit(t, src, Options{})
	p := plan.Parts[1]
	for _, want := range []string{
		"source: book.md\n",
		"source_sha256: " + plan.SHA256 + "\n",
		"part: 2\n",
		"parts: 2\n",
		"splitter_version: 1\n",
	} {
		if !bytes.Contains(p.Content, []byte(want)) {
			t.Errorf("frontmatter lacks %q", want)
		}
	}
	if !strings.HasPrefix(src[p.Start:], "## Two") {
		t.Errorf("part 2 starts at byte %d, which is not its heading", p.Start)
	}
}

// The checks verify runs are the package's promise, so each is shown able to fail.
func TestVerifyRefusesAPartThatChangesTheRendering(t *testing.T) {
	src := []byte("# A\n\ntext\n\n# B\n\nmore\n")
	good := []Part{
		{Start: 0, End: 11, Content: []byte("---\nx: 1\n---\n\n# A\n\ntext\n")},
		{Start: 11, End: len(src), Content: []byte("---\nx: 2\n---\n\n# B\n\nmore\n")},
	}
	if err := verify(src, good); err != nil {
		t.Fatalf("a faithful split was refused: %v", err)
	}

	gap := []Part{good[0], {Start: 12, End: len(src), Content: good[1].Content}}
	if err := verify(src, gap); err == nil {
		t.Error("a gap between parts was accepted")
	}

	altered := []Part{good[0], {
		Start: 11, End: len(src),
		Content: []byte("---\nx: 2\n---\n\n# B\n\nmore\n\n[x]: /y\n\nextra\n"),
	}}
	if err := verify(src, altered); !errors.Is(err, ErrRenderChanged) {
		t.Errorf("err = %v, want ErrRenderChanged for added content", err)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Chapter 8: Transactions":  "chapter-8-transactions",
		"What You Won’t Find Here": "what-you-won-t-find-here",
		"  ":                       "section",
		strings.Repeat("abc ", 40): "abc-abc-abc-abc-abc-abc-abc-abc-abc-abc-abc-abc",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
