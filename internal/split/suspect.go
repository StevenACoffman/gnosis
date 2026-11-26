package split

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// reSetextUnderline is a setext heading's underline: a run of = or - alone on its line.
var reSetextUnderline = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)

// reAssignment is heading text that reads as an assignment or a call: `x = f(`,
// `x := y`, `xgb.fit(X, y)`.
var reAssignment = regexp.MustCompile(
	`^[A-Za-z_][\w.]*\s*(:=|=)\s*\S|^[A-Za-z_][\w.]*\([^)]*\)\s*$`,
)

// reChartTick is a line holding one chart axis tick or data label: a year, a
// count, a fiscal period, an amount — `2014`, `120`, `FY20`, `$1.2`.
var reChartTick = regexp.MustCompile(`^\s*[$€£]?[A-Z]{0,4}\d[\d.,%]*\s*$`)

// reChapter is a heading naming a chapter or a part of a book.
var reChapter = regexp.MustCompile(`(?i)^(chapter|part)\s+(\d+|[ivxlc]+)\b`)

// Suspect is a heading split cut at that reads as something other than a heading.
//
// Every one found in agent-orange/kb came from converting a book to Markdown — a chart
// axis label `# of Sales` becoming an H1, terminal output followed by a line of
// dashes becoming a setext H2 — and each made split cut a book at its charts or its
// code instead of its chapters. A suspect is reported, never repaired: whether a
// heading is real is a judgement about the source, and the fix belongs there.
type Suspect struct {
	// Part is the file the heading starts.
	Part string `json:"part"`

	// Heading is the heading's text and Line its 1-based line in the source.
	Heading string `json:"heading"`
	Line    int    `json:"line"`

	// Reason says what the heading reads as instead.
	Reason string `json:"reason"`
}

// frontMatter names headings that open a book rather than enclose it. A chapter
// nested under one means the conversion put the book's headings at the wrong levels.
func frontMatter(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "preface", "foreword", "introduction", "contents", "table of contents",
		"acknowledgments", "acknowledgements", "about this book", "about the author":
		return true
	default:
		return false
	}
}

// suspects returns the cut headings that read as something other than a heading.
//
// Requires: parts tile src in order and each part's Headings ends with the heading
// at its Start when it has one.
// Ensures: at most one Suspect per cut, in source order; the first part is never a
// cut and is never examined. A chapter nested under front matter is reported once per
// enclosing heading, not once per chapter. Pure.
//
// Only cuts are examined, because a false heading the split did not cut at changed
// nothing about the parts.
func suspects(src []byte, parts []Part) []Suspect {
	var out []Suspect
	reported := map[string]bool{}
	for i := 1; i < len(parts); i++ {
		p := &parts[i]
		if len(p.Headings) == 0 {
			continue
		}
		title := p.Headings[len(p.Headings)-1]
		reason := readsAs(src, p.Start, title)
		if reason == "" && len(p.Headings) > 1 {
			parent := p.Headings[len(p.Headings)-2]
			if reChapter.MatchString(title) && frontMatter(parent) && !reported[parent] {
				reported[parent] = true
				reason = "a chapter nested under " + strconv.Quote(parent) +
					": the source's heading levels are probably wrong"
			}
		}
		if reason != "" {
			line := bytes.Count(src[:p.Start], []byte("\n")) + 1
			out = append(out, Suspect{Part: p.Name, Heading: title, Line: line, Reason: reason})
		}
	}
	return out
}

// readsAs says what a heading starting at start reads as instead, or "" when it reads
// as a heading.
func readsAs(src []byte, start int, title string) string {
	if n := setextLines(src, start); n > 1 {
		return "a setext heading spanning " + strconv.Itoa(n) +
			" lines: probably unfenced code or output above a line of dashes"
	}
	text := strings.TrimSpace(title)
	switch {
	case strings.HasPrefix(text, "$ "):
		return "a shell command"
	case reAssignment.MatchString(text):
		return "code"
	case startsLower(text) && reChartTick.Match(lineBefore(src, start)):
		return "a chart label: it starts lower-case and the line above it is chart data"
	default:
		return ""
	}
}

// lineBefore returns the nearest non-blank line ending before start, or nil.
//
// Lower case alone is not enough: some books set their front matter in lower case
// ("preface", "about this book"), and names like "gRPC" start lower-case. What sets
// a converted chart label apart is what surrounds it — `# of Sales` sits between the
// axis ticks of the chart it labelled.
func lineBefore(src []byte, start int) []byte {
	head := bytes.TrimRight(src[:start], " \t\r\n")
	if i := bytes.LastIndexByte(head, '\n'); i >= 0 {
		return head[i+1:]
	}
	return head
}

// setextLines returns how many lines of text a setext heading starting at start
// spans, or 0 when the heading is an ATX heading. It reads only as far as the
// heading's underline or the first blank line, not the rest of the source.
func setextLines(src []byte, start int) int {
	rest := src[start:]
	for i := 0; len(rest) > 0; i++ {
		line, after, _ := bytes.Cut(rest, []byte("\n"))
		switch {
		case i == 0 && bytes.HasPrefix(bytes.TrimLeft(line, " "), []byte("#")):
			return 0
		case i > 0 && reSetextUnderline.Match(line):
			return i
		case len(bytes.TrimSpace(line)) == 0:
			return 0
		}
		rest = after
	}
	return 0
}

func startsLower(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return unicode.IsLower(r)
}
