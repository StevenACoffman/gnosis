package web

// White-box, and the filename is what the linter sanctions for it. The renderer is
// unexported because nothing outside this package may render a corpus body — the whole
// safety argument is that the only markup on a page is what this file writes, and an
// exported renderer would be an invitation to a second caller with a second policy.

import (
	"sort"
	"strings"
	"testing"
)

// allowedTags is every tag the renderer may emit.
//
// The list is the safety property written down: anything else in the output came from
// the body, which means the escape-first order broke.
var allowedTags = []string{
	"<p>", "</p>", "<h2>", "</h2>", "<h3>", "</h3>", "<h4>", "</h4>", "<h5>", "</h5>",
	"<h6>", "</h6>", "<ul>", "</ul>", "<ol>", "</ol>", "<li>", "</li>",
	"<pre><code>", "</code></pre>", "<code>", "</code>", "<strong>", "</strong>",
	"<em>", "</em>", "<blockquote><p>", "</p></blockquote>",
}

// TestNoBodyCanBecomeMarkup is the property the renderer's shape exists for, and the one
// whose failure is worst: the corpus body is model-written (§13), so it is the least
// trustworthy content this server handles, and a body that could emit a tag could emit a
// script.
//
// Every case here is something a real markdown library would either pass through or need
// a sanitiser to strip. This renderer escapes first and emits markup afterwards, so none
// of them has a path to becoming markup at all.
func TestNoBodyCanBecomeMarkup(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"a script tag":                  `<script>alert(1)</script>`,
		"an image with onerror":         `<img src=x onerror=alert(1)>`,
		"a javascript link":             `[click](javascript:alert(1))`,
		"a data URI link":               `[click](data:text/html,<script>alert(1)</script>)`,
		"a protocol-relative link":      `[click](//evil.example/x)`,
		"an http link":                  `[click](http://evil.example/x)`,
		"a quote breaking an attribute": `[click](/c/a" onmouseover="alert(1))`,
		"markup inside a code fence":    "```\n<script>alert(1)</script>\n```",
		"markup inside a heading":       `# <script>alert(1)</script>`,
		"markup inside emphasis":        `**<script>alert(1)</script>**`,
		"markup inside a list":          `- <script>alert(1)</script>`,
		"markup inside a quote":         `> <script>alert(1)</script>`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertOnlyOurMarkup(t, string(renderBody(body)))
		})
	}
}

// assertOnlyOurMarkup checks that every tag in the output is one the renderer wrote.
//
// **The assertion is on the absence of an angle bracket, not on the absence of a
// keyword.** The first version of this test forbade substrings like "onerror" anywhere,
// and failed on output that was completely safe: `&lt;img src=x onerror=alert(1)&gt;`
// is escaped text a reader sees as the words the author typed. Forbidding the word
// would have made the test pass only when the renderer *deleted* content, which is a
// different and worse behaviour.
//
// So this strips the tags the renderer is allowed to emit, allows an anchor whose href
// is corpus-relative, and then requires that no `<` survives. What is left after that
// could only have come from the body.
func assertOnlyOurMarkup(t *testing.T, got string) {
	t.Helper()

	// Longest first, so a compound like `<blockquote><p>` is stripped whole rather
	// than having its `<p>` removed and leaving a bare `<blockquote>` that looks like
	// markup nobody declared.
	tags := make([]string, len(allowedTags))
	copy(tags, allowedTags)
	sort.Slice(tags, func(i, j int) bool { return len(tags[i]) > len(tags[j]) })

	stripped := got
	for _, tag := range tags {
		stripped = strings.ReplaceAll(stripped, tag, "")
	}
	for {
		open := strings.Index(stripped, `<a href="/`)
		if open < 0 {
			break
		}
		end := strings.Index(stripped[open:], ">")
		if end < 0 {
			break
		}
		href := stripped[open+len(`<a href="/`) : open+end]
		if strings.ContainsAny(href, `"'<> :`) && !strings.HasSuffix(href, `"`) {
			t.Errorf("an anchor's href carries something it should not: %q", href)
		}
		stripped = stripped[:open] + stripped[open+end+1:]
	}
	stripped = strings.ReplaceAll(stripped, "</a>", "")

	if strings.Contains(stripped, "<") {
		t.Errorf("markup the renderer did not write reached the page:\n%s\n"+
			"after stripping what it may emit: %s", got, stripped)
	}
}

// TestTheConstructsABodyActuallyUsesRender. The renderer is only worth its safety
// argument if it produces something readable — a sanitiser that stripped everything
// would pass the test above and make the viewer useless.
func TestTheConstructsABodyActuallyUsesRender(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"# Heading",
		"",
		"A paragraph with **bold**, _italic_ and `code`.",
		"It continues on a second line.",
		"",
		"- first",
		"- second",
		"",
		"1. one",
		"2. two",
		"",
		"> a quoted passage",
		"",
		"```",
		"literal text",
		"```",
		"",
		"A [link](/c/abc) to another concept.",
	}, "\n")

	got := string(renderBody(body))
	for _, want := range []string{
		"<h2>Heading</h2>", // demoted: the page's title is the h1
		"<strong>bold</strong>",
		"<em>italic</em>",
		"<code>code</code>",
		"It continues on a second line.", // the paragraph joined its lines
		"<ul>", "<li>first</li>",
		"<ol>", "<li>one</li>",
		"<blockquote><p>a quoted passage</p></blockquote>",
		"<pre><code>literal text\n</code></pre>",
		`<a href="/c/abc">link</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the rendered body is missing %q:\n%s", want, got)
		}
	}
}

// TestAHeadingNeverBecomesASecondPageTitle. A body's own `#` rendered as `h1` would give
// the document two titles, which is wrong for a screen reader before it is wrong for a
// stylesheet.
func TestAHeadingNeverBecomesASecondPageTitle(t *testing.T) {
	t.Parallel()

	if got := string(renderBody("# Top")); strings.Contains(got, "<h1>") {
		t.Errorf("a body heading became the page's second h1:\n%s", got)
	}
	// And a heading already at the deepest level stays there rather than overflowing
	// into a tag that does not exist.
	if got := string(renderBody("###### Deep")); !strings.Contains(got, "<h6>Deep</h6>") {
		t.Errorf("a level-six heading did not survive:\n%s", got)
	}
}

// TestAnUnbalancedDelimiterStaysLiteral. A stray backtick that opened a code span would
// swallow the rest of a paragraph, which is the kind of rendering bug a reader blames on
// the author.
func TestAnUnbalancedDelimiterStaysLiteral(t *testing.T) {
	t.Parallel()

	got := string(renderBody("a ` stray backtick and then some prose"))
	if strings.Contains(got, "<code>") {
		t.Errorf("an unbalanced delimiter opened a tag:\n%s", got)
	}
	if !strings.Contains(got, "stray backtick and then some prose") {
		t.Errorf("the text did not survive:\n%s", got)
	}
}

// TestDefinedTermsListOnlyWhatThePageUses. "A glossary nobody opens is not an ontology":
// a panel carrying the whole vocabulary is one a reader scrolls past, and the point is
// that the definition is where the term is.
func TestDefinedTermsListOnlyWhatThePageUses(t *testing.T) {
	t.Parallel()

	declared := map[string]string{
		"retry budget":       "how many times a failed call is retried",
		"request timeout":    "wall-clock limit on one outbound request",
		"retry.max_attempts": "how many times a failed call is retried",
	}
	got := definedTerms("The retry budget is three. Nothing here bounds a deadline.", declared)
	if len(got) != 1 {
		t.Fatalf("got %+v, want only the term the body uses", got)
	}
	if got[0].Key != "retry budget" {
		t.Errorf("key = %q, want the alias the author actually wrote", got[0].Key)
	}
	// Matched on the corpus's own fold, so capitalisation is not a second definition.
	if len(definedTerms("The Retry Budget is three.", declared)) != 1 {
		t.Error("the match is case-sensitive, so a capitalised term goes undefined")
	}
	if definedTerms("nothing declared here", nil) != nil {
		t.Error("a corpus with no vocabulary produced a panel")
	}
}
