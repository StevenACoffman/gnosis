package split

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// definition is one link reference definition: its normalized label, the source
// bytes it was written as, and where it starts.
type definition struct {
	key   string
	line  string
	start int
}

// newMarkdown returns the parser and renderer the equivalence check uses: goldmark
// with GFM, the configuration skillet/markdown parses with, so "renders the same"
// means the same thing as every other reading of a body in this family.
//
// This is a second parse of the source, which §7.1 otherwise avoids. It is not a
// second reading of the structure — sections come from skillet/markdown alone —
// but the oracle that checks the cut, and an oracle that shared the cutter's
// parse would share its mistakes.
func newMarkdown() goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(extension.GFM))
}

// definitions returns every link reference definition in src that is in force —
// the first for each label, as CommonMark resolves duplicates — in source order.
//
// Each is copied verbatim from the source rather than reconstructed from its parsed
// label, destination and title: a reconstruction would have to re-escape all three
// and would differ from the original in exactly the cases where escaping is subtle.
func definitions(src []byte) ([]definition, error) {
	root := newMarkdown().Parser().Parse(text.NewReader(src))
	seen := map[string]bool{}
	var out []definition
	err := ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		ref, ok := n.(*ast.LinkReferenceDefinition)
		if !entering || !ok || ref.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}
		key := foldLabel(string(ref.Label))
		if seen[key] {
			return ast.WalkSkipChildren, nil
		}
		seen[key] = true
		lines := ref.Lines()
		start := lineStartOf(src, lines.At(0).Start)
		stop := lines.At(lines.Len() - 1).Stop
		out = append(out, definition{
			key:   key,
			line:  strings.TrimRight(string(src[start:stop]), " \t\r\n"),
			start: start,
		})
		return ast.WalkSkipChildren, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect link definitions: %w", err)
	}
	return out, nil
}

// needed returns the definitions a part must carry: those defined outside the part
// whose label the part's body mentions in brackets.
//
// The textual test over-includes — a bracketed phrase that is not a link still
// matches — and that is harmless, because a definition renders to nothing. What it
// must never do is under-include, and verify is what would catch it.
func needed(body []byte, defs []definition, s span) []definition {
	if len(defs) == 0 {
		return nil
	}
	folded := foldLabel(string(body))
	var out []definition
	for _, d := range defs {
		if d.start >= s.start && d.start < s.end {
			continue
		}
		if strings.Contains(folded, "["+d.key+"]") {
			out = append(out, d)
		}
	}
	return out
}

// labels returns the labels of defs, sorted, or nil for none so the frontmatter
// field is omitted rather than written empty.
func labels(defs []definition) []string {
	if len(defs) == 0 {
		return nil
	}
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.key)
	}
	sort.Strings(out)
	return out
}

// foldLabel normalizes a label as CommonMark matches one: case-insensitively, with
// runs of whitespace collapsed. strings.ToLower stands in for Unicode case folding,
// which differs only for a handful of characters no link label here is likely to
// use; verify would catch the miss.
func foldLabel(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// lineStartOf returns the offset just after the newline preceding pos, or 0.
func lineStartOf(src []byte, pos int) int {
	for pos > 0 && src[pos-1] != '\n' {
		pos--
	}
	return pos
}

// verify checks the three properties the package promises: the parts tile the
// source, each body is a verbatim slice of it, and the parts render to what the
// source renders to.
//
// Requires: parts came from render over src.
// Ensures: nil exactly when all three hold. The rendering comparison is the one that
// matters and the one that is not true by construction: it is what proves cutting
// at heading lines was safe for this source and that copying definitions was enough.
func verify(src []byte, parts []Part) error {
	pos := 0
	for i := range parts {
		p := &parts[i]
		if p.Start != pos || p.End < p.Start {
			return fmt.Errorf("part %d covers [%d, %d), expected to start at %d",
				i+1, p.Start, p.End, pos)
		}
		want := bytes.TrimRight(src[p.Start:p.End], " \t\r\n")
		if !bytes.Contains(p.Content, want) {
			return fmt.Errorf("part %d does not carry bytes [%d, %d) verbatim", i+1, p.Start, p.End)
		}
		pos = p.End
	}
	if pos != len(src) {
		return fmt.Errorf("the parts end at byte %d of %d", pos, len(src))
	}

	md := newMarkdown()
	var whole, joined bytes.Buffer
	if err := md.Convert(src, &whole); err != nil {
		return fmt.Errorf("render the source: %w", err)
	}
	for i := range parts {
		if err := md.Convert(partBody(&parts[i]), &joined); err != nil {
			return fmt.Errorf("render part %d: %w", i+1, err)
		}
	}
	if !bytes.Equal(whole.Bytes(), joined.Bytes()) {
		at := firstDifference(whole.Bytes(), joined.Bytes())
		return fmt.Errorf("%w: first difference at rendered byte %d", ErrRenderChanged, at)
	}
	return nil
}

// partBody returns a part's content without its frontmatter.
//
// The frontmatter is for GitHub and for readers; goldmark without a metadata
// extension would render it as a thematic break and a heading, which is a fact
// about the comparison's parser and not about the part.
func partBody(p *Part) []byte {
	const closing = "\n---\n\n"
	if i := bytes.Index(p.Content, []byte(closing)); i >= 0 {
		return p.Content[i+len(closing):]
	}
	return p.Content
}

// firstDifference returns the index of the first byte at which a and b differ.
func firstDifference(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
