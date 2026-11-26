package split

import (
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/StevenACoffman/skillet/markdown"
)

// reFragmentLink finds the destination of an inline link or image whose destination is
// an in-document fragment: `](#anchor` with optional spaces after the parenthesis.
var reFragmentLink = regexp.MustCompile(`\]\([ \t]*(#[^\s)]*)`)

// reFragmentDef finds the destination of a link reference definition pointing at an
// in-document fragment.
var reFragmentDef = regexp.MustCompile(`(?m)^ {0,3}\[[^\]\n]+\]:[ \t]*(#\S+)`)

// rewrite is one link destination a part changed: at offset At in the part's body as
// cut, the fragment Old became New.
type rewrite struct {
	At       int
	Old, New string
}

// target is where a heading anchor lives once the source is split: the part holding
// the heading, and the anchor the heading has within that part.
type target struct {
	part   int
	anchor string
}

// slugger issues GitHub's heading anchors for one document, in order.
//
// The rule is github-slugger's, which is what GitHub renders and what rumdl's MD051
// checks against: lower-case, drop every character that is not a letter, mark, number,
// connector punctuation, space or hyphen, turn each space into a hyphen, and suffix
// `-1`, `-2`, … onto an anchor already issued in this document.
type slugger map[string]int

// anchors maps every heading's anchor in the whole source to where it lives after the
// split.
//
// An in-document link names a heading by the anchor GitHub gives it in the *whole*
// source; once split, the heading is in one part, under the anchor GitHub gives it in
// *that* part. The two differ only when a heading text repeats, because a repeated
// anchor takes a `-1`, `-2` suffix counted per document — the third "Summary" of a
// book is `#summary-2` in the book and may be `#summary` in its part. Both are
// computed here so a link can be pointed at the second.
//
// Requires: parts tile the source, in order.
func anchors(sections []markdown.Section, parts []Part) map[string]target {
	whole := slugger{}
	perPart := make([]slugger, len(parts))
	out := map[string]target{}
	p := 0
	for i := range sections {
		h := &sections[i]
		for p < len(parts)-1 && h.Start >= parts[p].End {
			p++
		}
		if perPart[p] == nil {
			perPart[p] = slugger{}
		}
		out[whole.next(h.Title)] = target{part: p, anchor: perPart[p].next(h.Title)}
	}
	return out
}

// relink points every in-document link in body at where its heading now lives.
//
// A link whose heading is in another part becomes `<that part's file>#anchor`; one
// whose heading is in this part keeps its fragment, corrected for the part's own
// anchor numbering. A fragment naming no heading is left exactly as written and
// returned as unresolved: it was broken in the source, and guessing a target would
// invent a link the author never made.
//
// Only real link destinations are touched. The regular expressions propose
// candidates; a candidate counts only when goldmark parsed a link with that
// destination and the candidate does not sit inside code, so `](#x)` written in a
// code sample is never rewritten.
//
// Ensures: applying the returned rewrites to body yields the result.
func relink(body []byte, self int, names []string, where map[string]target) (
	[]byte, []rewrite, []string,
) {
	dests, code := linkFacts(body)
	if len(dests) == 0 {
		return body, nil, nil
	}

	var rewrites []rewrite
	unresolved := map[string]bool{}
	for _, re := range []*regexp.Regexp{reFragmentLink, reFragmentDef} {
		for _, m := range re.FindAllSubmatchIndex(body, -1) {
			at, frag := m[2], string(body[m[2]:m[3]])
			if !dests[frag] || inside(code, at) {
				continue
			}
			r, ok := retarget(frag, at, self, names, where)
			switch {
			case !ok:
				unresolved[frag] = true
			case r != nil:
				rewrites = append(rewrites, *r)
			}
		}
	}
	sort.Slice(rewrites, func(i, j int) bool { return rewrites[i].At < rewrites[j].At })
	return applyRewrites(body, rewrites), rewrites, sortedKeys(unresolved)
}

// retarget decides what one fragment becomes: nil when it already names the right
// anchor in this part, and ok false when it names no heading at all.
func retarget(frag string, at, self int, names []string, where map[string]target) (*rewrite, bool) {
	t, ok := where[frag[1:]]
	switch {
	case !ok:
		return nil, false
	case t.part == self && t.anchor == frag[1:]:
		return nil, true
	case t.part == self:
		return &rewrite{At: at, Old: frag, New: "#" + t.anchor}, true
	default:
		return &rewrite{At: at, Old: frag, New: names[t.part] + "#" + t.anchor}, true
	}
}

// applyRewrites returns body with each rewrite applied.
//
// Requires: rewrites are sorted by At, do not overlap, and each Old sits at its At.
func applyRewrites(body []byte, rewrites []rewrite) []byte {
	if len(rewrites) == 0 {
		return body
	}
	var b bytes.Buffer
	prev := 0
	for _, r := range rewrites {
		b.Write(body[prev:r.At])
		b.WriteString(r.New)
		prev = r.At + len(r.Old)
	}
	b.Write(body[prev:])
	return b.Bytes()
}

// linkFacts parses body and returns the in-document link destinations goldmark found,
// and the byte ranges of code where a look-alike must not be rewritten.
func linkFacts(body []byte) (map[string]bool, [][2]int) {
	root := newMarkdown().Parser().Parse(text.NewReader(body))
	dests := map[string]bool{}
	var code [][2]int
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Link:
			addFragment(dests, v.Destination)
		case *ast.Image:
			addFragment(dests, v.Destination)
		case *ast.LinkReferenceDefinition:
			addFragment(dests, v.Destination)
		case *ast.CodeSpan:
			if r, ok := childSpan(v); ok {
				code = append(code, r)
			}
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock, *ast.CodeBlock, *ast.HTMLBlock:
			if lines := n.Lines(); lines.Len() > 0 {
				code = append(code, [2]int{lines.At(0).Start, lines.At(lines.Len() - 1).Stop})
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return dests, code
}

func addFragment(dests map[string]bool, dest []byte) {
	if len(dest) > 1 && dest[0] == '#' {
		dests[string(dest)] = true
	}
}

// childSpan returns the byte range a code span's text covers.
func childSpan(n ast.Node) ([2]int, bool) {
	first, last := -1, -1
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			if first < 0 {
				first = t.Segment.Start
			}
			last = t.Segment.Stop
		}
	}
	return [2]int{first, last}, first >= 0
}

func inside(ranges [][2]int, at int) bool {
	for _, r := range ranges {
		if at >= r[0] && at < r[1] {
			return true
		}
	}
	return false
}

func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// next is github-slugger's `slug` method transcribed: the occurrence count is kept
// on the unsuffixed anchor, and a suffixed anchor is itself recorded as taken, so a
// heading literally titled "Summary 1" cannot collide with the second "Summary".
func (s slugger) next(title string) string {
	base := githubSlug(title)
	anchor := base
	for {
		if _, taken := s[anchor]; !taken {
			break
		}
		s[base]++
		anchor = base + "-" + strconv.Itoa(s[base])
	}
	s[anchor] = 0
	return anchor
}

func githubSlug(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-', r == '_', unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r),
			unicode.Is(unicode.Pc, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
