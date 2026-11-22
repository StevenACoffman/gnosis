package split

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// cutBody builds one part's body: the source slice with trailing whitespace trimmed,
// followed by any link definitions it needs from elsewhere. The recorded range is
// the untrimmed slice, so the ranges still tile the source.
func cutBody(src []byte, s span, n, width int, stem string, defs []definition) Part {
	body := bytes.TrimRight(src[s.start:s.end], " \t\r\n")
	copied := needed(body, defs, s)

	var buf bytes.Buffer
	buf.Write(body)
	buf.WriteByte('\n')
	if len(copied) > 0 {
		buf.WriteByte('\n')
		for _, d := range copied {
			buf.WriteString(d.line)
			buf.WriteByte('\n')
		}
	}

	slugFrom := "front"
	if len(s.trail) > 0 {
		slugFrom = s.trail[len(s.trail)-1]
	}
	startLine, endLine := lineRange(src, s.start, s.end)
	return Part{
		Name:      fmt.Sprintf("%s-%0*d-%s.md", stem, width, n, slug(slugFrom)),
		Start:     s.start,
		End:       s.end,
		StartLine: startLine,
		EndLine:   endLine,
		Headings:  s.trail,
		Copied:    labels(copied),
		Content:   buf.Bytes(),
		number:    n,
	}
}

// addFrontmatter puts the provenance block on top of a part's body.
func addFrontmatter(p *Part, total int, sum string, opt Options) error {
	rec := provenance{Split: splitRecord{
		Source:     opt.Source,
		SHA256:     sum,
		Part:       p.number,
		Parts:      total,
		Bytes:      strconv.Itoa(p.Start) + "-" + strconv.Itoa(p.End),
		Lines:      strconv.Itoa(p.StartLine) + "-" + strconv.Itoa(p.EndLine),
		Headings:   p.Headings,
		Copied:     p.Copied,
		MinBytes:   opt.MinBytes,
		Relinked:   p.Relinked,
		Unresolved: p.Unresolved,
		Version:    Version,
	}}
	fm, err := yaml.Marshal(rec)
	if err != nil {
		return fmt.Errorf("render %s: %w", p.Name, err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	buf.WriteString("---\n\n")
	buf.Write(p.Content)
	p.Content = buf.Bytes()
	return nil
}

// lineRange returns the 1-based inclusive line range of src[start:end], ignoring
// trailing newlines so a part ending in blank lines reports its last line of text.
func lineRange(src []byte, start, end int) (first, last int) {
	first = bytes.Count(src[:start], []byte("\n")) + 1
	trimmed := bytes.TrimRight(src[start:end], "\n")
	last = first + bytes.Count(trimmed, []byte("\n"))
	return first, last
}

// slug turns a heading into a file-name fragment: lower-case ASCII letters and
// digits, runs of anything else collapsed to one hyphen.
func slug(title string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			hyphen = false
		case !hyphen && b.Len() > 0:
			b.WriteByte('-')
			hyphen = true
		}
		if b.Len() >= maxSlug {
			break
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "section"
	}
	return out
}
