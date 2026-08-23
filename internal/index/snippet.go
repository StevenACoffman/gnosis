package index

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/skillet/markdown"
)

// snippetWindow is how many bytes of context a snippet carries around a match.
const snippetWindow = 160

// headingMarks matches the leading `#` run of an ATX heading.
//
// goldmark's prose output keeps them, and a snippet reading "## Empty section" is
// showing the reader syntax rather than prose. Stripped at render time for the same
// reason links are.
var headingMarks = regexp.MustCompile(`(?m)^#{1,6}\s+`)

// inlineLink matches a markdown inline link, capturing its text.
//
// This is a rendering reduction, not a parse, and the difference licenses the
// regex: a snippet is prose shown to a person, so a link whose destination
// contains an unbalanced parenthesis degrades to slightly untidy text rather than
// to a wrong answer. Nothing downstream reads it.
var inlineLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// Snippet renders an excerpt of body around the first occurrence of a query term.
//
// Requires: nothing; an empty body or query yields "".
// Ensures: the result is plain prose — code blanked, inline links reduced to their
// text, whitespace collapsed — and is at most a bounded window around the match.
// It is pure.
//
// # Why this is re-derived rather than taken from FTS5
//
// FTS5's snippet() returns an excerpt of the *indexed* text, which is the document
// as written: a hit near a link renders most of its width as
// `[Timeout](/c/01932b7c-…-timeout-policy.md)`, and the reader gets a UUID instead
// of a sentence. Stripping markdown before indexing is the obvious alternative and
// is worse — someone searching for a slug should find it, and the index is the only
// place that can answer that.
//
// So the body is indexed as written and the snippet is re-derived here. That the
// two no longer share offsets is the point: an offset-mapped snippet would have to
// keep the rendered text and the indexed text in correspondence forever, and this
// has to keep nothing.
func Snippet(body, query string) string {
	prose := markdown.Parse(body).Prose
	prose = headingMarks.ReplaceAllString(prose, "")
	prose = inlineLink.ReplaceAllString(prose, "$1")
	prose = strings.Join(strings.Fields(prose), " ")
	if prose == "" {
		return ""
	}

	at := firstMatch(prose, query)
	if at < 0 || len(prose) <= snippetWindow {
		return truncate(prose, snippetWindow)
	}

	start := max(at-snippetWindow/2, 0)
	end := min(start+snippetWindow, len(prose))
	out := prose[start:end]
	if start > 0 {
		out = "…" + out
	}
	if end < len(prose) {
		out += "…"
	}
	return out
}

// firstMatch returns the byte offset of the earliest query term in prose, or -1.
//
// Terms are matched independently and case-insensitively rather than as a phrase,
// because the query reaching FTS5 may be a boolean expression and the point here is
// to show the reader *why* this document matched, not to reproduce FTS5's parse.
func firstMatch(prose, query string) int {
	lower := strings.ToLower(prose)
	best := -1
	for _, term := range strings.Fields(strings.ToLower(query)) {
		term = strings.Trim(term, `"()*`)
		if term == "" || isOperator(term) {
			continue
		}
		if at := strings.Index(lower, term); at >= 0 && (best < 0 || at < best) {
			best = at
		}
	}
	return best
}

// isOperator reports whether a query token is FTS5 syntax rather than a term.
func isOperator(term string) bool {
	switch term {
	case "and", "or", "not", "near":
		return true
	default:
		return false
	}
}

// truncate bounds a string at n bytes, on a rune boundary, marking the cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}
