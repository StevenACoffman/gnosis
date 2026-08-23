package index_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/index"
)

// TestSnippetReducesLinksToTheirText is the whole reason the snippet is re-derived
// rather than taken from FTS5. A hit next to a link used to render most of its
// width as a UUID, which is not what the reader searched for.
func TestSnippetReducesLinksToTheirText(t *testing.T) {
	t.Parallel()
	body := "The service retries three times. See " +
		"[Timeout policy](/c/01932b7c-2a03-7b11-8e44-9f10c2d3e4f5-timeout-policy.md).\n"

	got := index.Snippet(body, "retries")

	if strings.Contains(got, "01932b7c") {
		t.Errorf("snippet still carries the link destination:\n%s", got)
	}
	if !strings.Contains(got, "Timeout policy") {
		t.Errorf("snippet dropped the link text along with the destination:\n%s", got)
	}
}

// TestSnippetBlanksCode: a fenced block is not prose, and a hit inside one should
// not fill the excerpt with source. skillet's markdown.Prose already blanks it;
// this pins that the snippet path uses it.
func TestSnippetBlanksCode(t *testing.T) {
	t.Parallel()
	body := "Prose about retries.\n\n```go\nfunc retries() {}\n```\n"

	if got := index.Snippet(body, "retries"); strings.Contains(got, "func") {
		t.Errorf("code leaked into the snippet:\n%s", got)
	}
}

// TestSnippetWindowsOnTheMatch: the reader needs to see why the document matched,
// so a hit deep in a long body must not return the opening paragraph.
func TestSnippetWindowsOnTheMatch(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("filler words here. ", 60) + "the DISTINCTIVE term. " +
		strings.Repeat("more filler. ", 60)

	got := index.Snippet(body, "distinctive")

	if !strings.Contains(strings.ToLower(got), "distinctive") {
		t.Errorf("snippet does not contain the match:\n%s", got)
	}
	if len(got) > 200 {
		t.Errorf("snippet is %d bytes, want a bounded window", len(got))
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("a snippet cut from the middle does not mark the cut:\n%s", got)
	}
}

// TestSnippetIgnoresQueryOperators: the query reaching FTS5 may be a boolean
// expression, and windowing on the word "AND" would put the excerpt somewhere
// arbitrary.
func TestSnippetIgnoresQueryOperators(t *testing.T) {
	t.Parallel()
	body := "This document mentions AND in passing, then much later the actual subject: caching."

	got := index.Snippet(body, "caching AND retries")

	if !strings.Contains(got, "caching") {
		t.Errorf("snippet windowed on an operator rather than a term:\n%s", got)
	}
}

// TestSnippetHandlesNoMatch: a document can match FTS5 through stemming while no
// literal term appears — searching "retrying" matches "retries" under porter. The
// snippet must still return something readable rather than nothing.
func TestSnippetHandlesNoMatch(t *testing.T) {
	t.Parallel()
	body := "The service retries three times before giving up."

	if got := index.Snippet(body, "retrying"); got == "" {
		t.Error("a stemmed match produced an empty snippet; want the opening prose")
	}
}

// TestSnippetIsPure guards the property the whole design leans on: two calls with
// one input agree, so a search rendered twice reads the same.
func TestSnippetIsPure(t *testing.T) {
	t.Parallel()
	body := "Some prose with a [link](/c/x.md) and a term to find."

	first := index.Snippet(body, "term")
	for range 10 {
		if again := index.Snippet(body, "term"); again != first {
			t.Fatalf("output varies:\n%q\n%q", first, again)
		}
	}
}

// TestSnippetStripsHeadingMarks: goldmark's prose output keeps them, and a snippet
// reading "## Empty section" shows the reader syntax rather than prose.
func TestSnippetStripsHeadingMarks(t *testing.T) {
	t.Parallel()
	body := "Prose about retries.\n\n## A heading\n\nmore prose\n"

	got := index.Snippet(body, "retries")

	if strings.Contains(got, "#") {
		t.Errorf("heading marks leaked into the snippet:\n%s", got)
	}
	if !strings.Contains(got, "A heading") {
		t.Errorf("heading text was dropped along with its marks:\n%s", got)
	}
}
