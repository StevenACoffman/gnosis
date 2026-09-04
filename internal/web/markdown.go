package web

import (
	"html/template"
	"strings"
)

// renderBody turns a concept's markdown body into safe HTML.
//
// Requires: body is a concept's markdown, which arrived from a model and is therefore
// the least trustworthy content this server handles (§13: the corpus body is
// model-written by design).
// Ensures: every character of the input is HTML-escaped before any markup is added, so
// no byte of the body can become a tag or an attribute. Pure.
//
// # Why a small renderer rather than a library
//
// TODO:1172's own condition was that inline definitions need "a renderer that produces
// markup, and that renderer arrives with its own sanitiser". A general markdown library
// accepts raw HTML by design — CommonMark requires it — so using one means adding a
// sanitiser with an allow-list of tags and attributes to maintain, and the corpus's body
// is exactly the input an allow-list is worst against.
//
// This inverts the order. It escapes first and then emits markup for the constructs OKF
// bodies actually use, so the safety property is **structural**: there is no path by
// which input becomes markup, because the only markup is what this function writes. A
// construct it does not know renders as its own literal text, which is legible and
// wrong-looking rather than dangerous.
//
// What it handles: ATX headings, paragraphs, unordered and ordered lists, fenced code,
// blockquotes, inline code, bold, italic, and links to a concept. What it deliberately
// does not: tables, footnotes, images, and reference links. Each of those is a feature
// somebody can ask for with a body that needs it, and none is worth the surface today.
func renderBody(body string) template.HTML {
	var b strings.Builder
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")

	for i := 0; i < len(lines); i++ {
		switch {
		case strings.HasPrefix(lines[i], "```"):
			i = writeFence(&b, lines, i)
		case strings.TrimSpace(lines[i]) == "":
		case isHeading(lines[i]):
			writeHeading(&b, lines[i])
		case isBullet(lines[i]):
			i = writeList(&b, lines, i)
		case strings.HasPrefix(lines[i], ">"):
			i = writeQuote(&b, lines, i)
		default:
			i = writeParagraph(&b, lines, i)
		}
	}
	return template.HTML(b.String()) //nolint:gosec // every span was escaped above
}

// writeFence emits a fenced code block and returns the index of its closing fence.
//
// The content is escaped and no language class is emitted: a class attribute derived
// from the fence's info string would be input reaching an attribute, which is the one
// thing this renderer's shape exists to prevent. Syntax highlighting is a client
// concern and this is a knowledge base.
func writeFence(b *strings.Builder, lines []string, start int) int {
	b.WriteString("<pre><code>")
	i := start + 1
	for ; i < len(lines) && !strings.HasPrefix(lines[i], "```"); i++ {
		b.WriteString(template.HTMLEscapeString(lines[i]))
		b.WriteString("\n")
	}
	b.WriteString("</code></pre>\n")
	return i
}

// isHeading reports whether a line is an ATX heading.
func isHeading(line string) bool {
	hashes := len(line) - len(strings.TrimLeft(line, "#"))
	return hashes >= 1 && hashes <= 6 && strings.HasPrefix(line[hashes:], " ")
}

// writeHeading emits a heading, one level deeper than it declares.
//
// **Demoted by one**, because the page already carries the concept's title as its `h1`:
// a body whose own `#` heading became a second `h1` would give the document two titles,
// which is wrong for a screen reader before it is wrong for a stylesheet.
func writeHeading(b *strings.Builder, line string) {
	level := len(line) - len(strings.TrimLeft(line, "#")) + 1
	if level > 6 {
		level = 6
	}
	tag := "h" + string(rune('0'+level))
	b.WriteString("<" + tag + ">")
	b.WriteString(inline(strings.TrimSpace(strings.TrimLeft(line, "#"))))
	b.WriteString("</" + tag + ">\n")
}

// isBullet reports whether a line opens a list item.
func isBullet(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	return strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
		orderedMarker(trimmed) > 0
}

// orderedMarker is the length of an ordered list's marker, or zero.
func orderedMarker(line string) int {
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits+1 >= len(line) {
		return 0
	}
	if line[digits] == '.' && line[digits+1] == ' ' {
		return digits + 2
	}
	return 0
}

// writeList emits a list and returns the index of its last item.
//
// The whole run is one list, and its kind is the first item's: a run that mixed `-` and
// `1.` is a body somebody wrote by accident, and rendering it as two lists would hide
// that rather than showing it.
func writeList(b *strings.Builder, lines []string, start int) int {
	tag := "ul"
	if orderedMarker(strings.TrimLeft(lines[start], " ")) > 0 {
		tag = "ol"
	}
	b.WriteString("<" + tag + ">\n")
	i := start
	for ; i < len(lines) && isBullet(lines[i]); i++ {
		b.WriteString("<li>")
		b.WriteString(inline(itemText(lines[i])))
		b.WriteString("</li>\n")
	}
	b.WriteString("</" + tag + ">\n")
	return i - 1
}

// itemText is a list item's content, without its marker.
func itemText(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if n := orderedMarker(trimmed); n > 0 {
		return trimmed[n:]
	}
	return strings.TrimSpace(trimmed[1:])
}

// writeQuote emits a blockquote and returns the index of its last line.
func writeQuote(b *strings.Builder, lines []string, start int) int {
	var parts []string
	i := start
	for ; i < len(lines) && strings.HasPrefix(lines[i], ">"); i++ {
		parts = append(parts, strings.TrimSpace(strings.TrimPrefix(lines[i], ">")))
	}
	b.WriteString("<blockquote><p>")
	b.WriteString(inline(strings.Join(parts, " ")))
	b.WriteString("</p></blockquote>\n")
	return i - 1
}

// writeParagraph emits a paragraph and returns the index of its last line.
func writeParagraph(b *strings.Builder, lines []string, start int) int {
	var parts []string
	i := start
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" || isHeading(lines[i]) ||
			isBullet(lines[i]) || strings.HasPrefix(lines[i], "```") ||
			strings.HasPrefix(lines[i], ">") {
			break
		}
		parts = append(parts, lines[i])
	}
	b.WriteString("<p>")
	b.WriteString(inline(strings.Join(parts, " ")))
	b.WriteString("</p>\n")
	return i - 1
}
