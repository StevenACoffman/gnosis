package web

import (
	"html/template"
	"strings"
)

// inline renders a run of markdown text, escaping before it emits anything.
//
// Requires: text is one logical line or paragraph of a concept's body.
// Ensures: the result contains no markup derived from the input — every byte of `text`
// is HTML-escaped first, and the only tags are the ones this function writes. Pure.
//
// **Escape first, then decorate.** The order is the safety property: a renderer that
// emitted markup and escaped afterwards would escape its own tags, and one that decided
// per span which parts to escape is one span away from a mistake. Here the input is
// escaped once, up front, and every construct below matches on characters that survive
// escaping — so there is no path by which a `<script>` in a body becomes a tag.
func inline(text string) string {
	escaped := template.HTMLEscapeString(text)
	escaped = spans(escaped, "`", "<code>", "</code>")
	escaped = spans(escaped, "**", "<strong>", "</strong>")
	escaped = spans(escaped, "_", "<em>", "</em>")
	return links(escaped)
}

// spans wraps balanced runs of a delimiter in a tag pair.
//
// Requires: text is already HTML-escaped; openTag and closeTag are literal tags.
// Ensures: an unbalanced delimiter is left as the literal character rather than opening
// a tag that never closes — which would otherwise let a stray backtick swallow the rest
// of a paragraph into a code span. Pure.
func spans(text, delim, openTag, closeTag string) string {
	var b strings.Builder
	rest := text
	for {
		start := strings.Index(rest, delim)
		if start < 0 {
			break
		}
		after := start + len(delim)
		end := strings.Index(rest[after:], delim)
		if end < 0 {
			break
		}
		b.WriteString(rest[:start])
		b.WriteString(openTag)
		b.WriteString(rest[after : after+end])
		b.WriteString(closeTag)
		rest = rest[after+end+len(delim):]
	}
	b.WriteString(rest)
	return b.String()
}

// links renders `[text](href)` for the hrefs a corpus may point at, and nothing else.
//
// Requires: text is already HTML-escaped.
// Ensures: an href that is not a corpus-relative path is dropped and its label kept, so
// a `javascript:` or `data:` URI cannot become an anchor. Pure.
//
// # An allow-list of one shape, not a deny-list of schemes
//
// A deny-list has to anticipate every dangerous scheme, and the interesting ones are the
// ones nobody thought of. This admits exactly what §8.3 requires rendered — a link to
// another concept — and drops everything else. An external reference belongs in a
// concept's `sources`, where §5.1 records what it is and tier 0 archives it; a body
// linking out to a page nobody kept is the unreproducible reference §4.1 refuses.
func links(text string) string {
	var b strings.Builder
	rest := text
	for {
		open := strings.Index(rest, "[")
		if open < 0 {
			break
		}
		label, href, width, ok := linkAt(rest[open:])
		if !ok {
			b.WriteString(rest[:open+1])
			rest = rest[open+1:]
			continue
		}
		b.WriteString(rest[:open])
		if internalHref(href) {
			b.WriteString(`<a href="` + href + `">` + label + `</a>`)
		} else {
			// The label survives and the link does not. A reader sees the words
			// somebody wrote rather than a hole, and cannot be sent anywhere by them.
			b.WriteString(label)
		}
		rest = rest[open+width:]
	}
	b.WriteString(rest)
	return b.String()
}

// linkAt parses `[label](href)` at the start of text.
//
// Ensures: ok is false for anything that is not exactly that shape, in which case the
// caller emits the bracket literally. Pure.
func linkAt(text string) (label, href string, width int, ok bool) {
	closeLabel := strings.Index(text, "](")
	if closeLabel < 0 {
		return "", "", 0, false
	}
	closeHref := strings.Index(text[closeLabel:], ")")
	if closeHref < 0 {
		return "", "", 0, false
	}
	label = text[1:closeLabel]
	href = text[closeLabel+2 : closeLabel+closeHref]
	if strings.ContainsAny(label, "[]") || strings.ContainsAny(href, "\"'<> ") {
		// A quote or a space in the href would break out of the attribute, and a
		// bracket in the label means the parse guessed wrong about where it ended.
		return "", "", 0, false
	}
	return label, href, closeLabel + closeHref + 1, true
}

// internalHref reports whether an href points inside this corpus.
//
// Requires: href came from linkAt, so it holds no quote, angle bracket or space.
// Ensures: true only for a rooted path with no scheme and no protocol-relative prefix.
// Pure.
//
// `//host/path` is refused explicitly: it has no scheme, so a check for a colon would
// admit it, and a browser reads it as a link to another host.
func internalHref(href string) bool {
	return strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") &&
		!strings.Contains(href, ":")
}
