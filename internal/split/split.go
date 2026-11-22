// Package split cuts one Markdown document into parts small enough to archive, at
// heading boundaries, without changing what any part renders to.
//
// SPEC §4.3 caps an archived file at a per-file size, and a source over the cap falls
// to `referenced`: its hash is kept and its text is not, so no quotation from it can
// ever validate. A book is the ordinary victim — the cap was set against a seed corpus
// whose longest source was under 90 KiB, and most book-length sources are several
// times that. Raising the cap is a §6.2 loosening; splitting the source keeps the cap
// and the evidence both.
//
// Three properties make a split safe to archive instead of the original, and each is
// checked rather than assumed:
//
//   - **Every part is a verbatim slice.** A part's body is src[Start:End] with only
//     trailing whitespace trimmed, so a quotation found in a part is a quotation
//     found in the source, and the recorded byte and line ranges say exactly where.
//   - **The slices tile the source.** Consecutive, non-overlapping, covering every
//     byte. Nothing is dropped between parts and nothing is duplicated across them.
//   - **Rendering is unchanged.** The concatenated HTML of the parts equals the HTML
//     of the source. Cutting only at top-level heading lines makes this hold by
//     construction for everything but link reference definitions, which resolve
//     document-wide; those are copied into each part that needs one, and the
//     comparison is what proves the copying was enough.
//
// The package is pure: bytes in, planned files out. Writing them is the shell's job.
package split

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/frontmatter"
	"github.com/StevenACoffman/skillet/markdown"
)

// Version names the splitting rules a part was cut under. It is recorded in every
// part, so a change to the rules that would cut a source differently is visible as a
// different value rather than as files that silently disagree with a re-run.
const Version = 2

// frontmatterReserve is the room left under the limit for a part's provenance
// frontmatter. A heading trail long enough to exceed it produces an Oversize part,
// which is reported rather than lost.
const frontmatterReserve = 2048

// maxSlug bounds a slug's length so a long chapter title does not produce a file
// name some filesystem refuses.
const maxSlug = 48

// ErrFrontmatter is returned for a source that begins with a frontmatter block.
//
// Each part carries its own provenance frontmatter, and the first part would then
// hold two. Merging them would mean deciding which of the source's keys survive,
// which is a judgement this package has no basis for; no source it was built for
// carries one.
var ErrFrontmatter = errors.New("source begins with a frontmatter block")

// ErrRenderChanged is returned when the parts do not render to what the source
// renders to. It is a refusal, not a warning: a part whose meaning differs from the
// passage it was cut from would be archived as evidence of something the source
// does not say.
var ErrRenderChanged = errors.New("the parts do not render to what the source renders to")

// Options are the inputs a plan depends on besides the source bytes.
type Options struct {
	// Source is the source file's base name, recorded in every part and used as the
	// prefix of every part's file name.
	Source string

	// Limit is the largest part, in bytes, frontmatter and copied definitions
	// included — normally the bundle's per-file archive cap.
	Limit int

	// Level, when non-zero, cuts at headings of this level first even where the
	// result would fit under Limit, so a book can be split into chapters rather than
	// into the fewest parts that fit. Parts still over Limit are cut further.
	Level int
}

// Part is one planned file.
type Part struct {
	// Name is the file name: `<stem>-<NN>-<slug>.md`.
	Name string

	// Start and End are the half-open byte range [Start, End) of the source this part
	// was cut from. StartLine and EndLine are the same range in 1-based lines,
	// inclusive.
	Start, End         int
	StartLine, EndLine int

	// Headings is the trail of headings enclosing the part, outermost first, ending
	// with the part's own first heading when it has one.
	Headings []string

	// Copied lists the link reference labels whose definitions live elsewhere in the
	// source and were appended to this part so its links still resolve.
	Copied []string

	// Relinked counts the in-document links this part points at another part, or at
	// its own renumbered anchor. Unresolved lists fragments that named no heading in
	// the source either; they are left as written.
	Relinked   int
	Unresolved []string

	// Content is the complete file: provenance frontmatter, the body, and any
	// copied definitions.
	Content []byte

	// unlinked is the body before relinking and rewrites what relinking changed;
	// verify checks the first against the source and the second against the file.
	unlinked []byte
	rewrites []rewrite
	number   int
}

// Plan is the outcome of splitting one source.
type Plan struct {
	// SHA256 is the hex digest of the source bytes.
	SHA256 string

	// Parts is every planned file, in source order. A source already under Limit
	// yields no parts: it can be archived as it is.
	Parts []Part

	// Suspects are cut headings that read as something other than a heading — a
	// chart label, a line of code, terminal output — or chapters nested under front
	// matter. The parts are planned regardless; the shell decides what a suspect
	// costs.
	Suspects []Suspect

	// Oversize names parts still over Limit because the stretch they cover has no
	// heading left to cut at. They are planned anyway — a part over the cap is still
	// a smaller problem than the whole book over it — and the shell reports them.
	Oversize []string
}

// span is a candidate part before it is rendered: a byte range and the headings
// that enclose it.
type span struct {
	start, end int
	trail      []string
}

// cutter holds what every recursive cut reads.
type cutter struct {
	src      []byte
	sections []markdown.Section
	opt      Options
}

// provenance is the frontmatter every part carries.
//
// Field order is declaration order, which go-yaml preserves, so the same plan always
// serializes to the same bytes.
type provenance struct {
	Split splitRecord `yaml:"gnosis_split"`
}

type splitRecord struct {
	Source   string   `yaml:"source"`
	SHA256   string   `yaml:"source_sha256"`
	Part     int      `yaml:"part"`
	Parts    int      `yaml:"parts"`
	Bytes    string   `yaml:"bytes"`
	Lines    string   `yaml:"lines"`
	Headings []string `yaml:"headings,omitempty"`
	Copied   []string `yaml:"copied_definitions,omitempty"`

	// Relinked counts in-document links pointed at the part now holding their
	// heading; Unresolved lists fragments that named no heading in the source.
	Relinked   int      `yaml:"relinked_links,omitempty"`
	Unresolved []string `yaml:"unresolved_fragments,omitempty"`
	Version    int      `yaml:"splitter_version"`
}

// Split plans the parts of src.
//
// Requires: opt.Source is non-empty and opt.Limit is positive.
// Ensures: on success, the parts tile src, every part's body is a verbatim slice of
// it, and the parts render to what src renders to. The result is a pure function of
// (src, opt) — re-running a split over unchanged bytes plans byte-identical files.
// Returns ErrFrontmatter or ErrRenderChanged (wrapped) for the two refusals.
func Split(src []byte, opt Options) (*Plan, error) {
	if opt.Source == "" || opt.Limit <= 0 {
		return nil, errors.New("split: a source name and a positive limit are required")
	}
	if block, _ := frontmatter.Split(string(src)); block != "" {
		return nil, fmt.Errorf("split %s: %w", opt.Source, ErrFrontmatter)
	}

	sum := sha256.Sum256(src)
	plan := &Plan{SHA256: hex.EncodeToString(sum[:])}
	if len(src) <= opt.Limit && opt.Level == 0 {
		return plan, nil
	}

	doc := markdown.Parse(string(src))
	c := cutter{src: src, sections: doc.Sections, opt: opt}
	spans := c.cut(span{start: 0, end: len(src)}, opt.Level)

	if err := renderAll(src, doc.Sections, spans, opt, plan); err != nil {
		return nil, fmt.Errorf("split %s: %w", opt.Source, err)
	}
	if err := verify(src, plan.Parts); err != nil {
		return nil, fmt.Errorf("split %s: %w", opt.Source, err)
	}
	plan.Suspects = suspects(src, plan.Parts)
	return plan, nil
}

// renderAll renders every span into plan.Parts, noting any still over the limit.
//
// Three passes, because the middle one needs every part's name: cut each body and
// copy the definitions it needs, point in-document links at the part now holding
// their heading, then put the provenance frontmatter on top — last, so it can record
// what the relinking did.
func renderAll(
	src []byte,
	sections []markdown.Section,
	spans []span,
	opt Options,
	plan *Plan,
) error {
	defs, err := definitions(src)
	if err != nil {
		return err
	}
	stem := strings.TrimSuffix(opt.Source, ".md")
	width := max(2, len(strconv.Itoa(len(spans))))
	parts := make([]Part, len(spans))
	names := make([]string, len(spans))
	for i, s := range spans {
		parts[i] = cutBody(src, s, i+1, width, stem, defs)
		names[i] = parts[i].Name
	}

	where := anchors(sections, parts)
	for i := range parts {
		p := &parts[i]
		p.unlinked = p.Content
		p.Content, p.rewrites, p.Unresolved = relink(p.unlinked, i, names, where)
		p.Relinked = len(p.rewrites)
	}

	for i := range parts {
		if err := addFrontmatter(&parts[i], len(parts), plan.SHA256, opt); err != nil {
			return err
		}
		if len(parts[i].Content) > opt.Limit {
			plan.Oversize = append(plan.Oversize, parts[i].Name)
		}
	}
	plan.Parts = parts
	return nil
}

// cut divides s at the shallowest heading level found inside it, then cuts again any
// piece still too large.
//
// A heading starting exactly at s.start is s's own and is not a cut point; it joins
// the trail instead. Cutting at the shallowest level first is what makes the parts
// follow the book's structure — parts, then chapters, then sections — rather than an
// arbitrary byte count.
//
// The lead — whatever sits between s.start and the first cut, typically a part title
// and its short introduction — is joined to the first piece when the two fit
// together, so a part's opening paragraph is not archived as a file of its own.
//
// Requires: 0 <= s.start <= s.end <= len(c.src).
// Ensures: the returned spans tile [s.start, s.end) in order.
func (c *cutter) cut(s span, level int) []span {
	own, inner := c.headingsIn(s)
	trail := s.trail
	if own != nil {
		trail = appendTrail(trail, own.Title)
	}
	if !c.tooBig(s) && level == 0 {
		return []span{{start: s.start, end: s.end, trail: trail}}
	}

	cutLevel := shallowest(inner, level)
	if cutLevel == 0 {
		return []span{{start: s.start, end: s.end, trail: trail}}
	}

	pieces := atLevel(s, inner, cutLevel, trail)
	if level == 0 {
		// Under a forced level the lead stays separate: joining it could fold a
		// two-chapter book back into one file and defeat the option.
		pieces = c.joinLead(pieces, s.start)
	}

	var out []span
	for _, p := range pieces {
		// The level option applies once, at the top: below it only size decides.
		out = append(out, c.cut(p, 0)...)
	}
	return out
}

// atLevel cuts s before every inner heading of the given level.
//
// A piece headed by a cut heading sits under s's own heading, so it inherits trail,
// which includes it. The lead starts at s's own heading and adds that heading itself
// when it is cut, so it inherits s.trail, which does not.
//
// Requires: some heading in inner has the given level, so at least one cut falls
// strictly inside s.
// Ensures: the pieces tile s in order, the first starting at s.start.
func atLevel(s span, inner []markdown.Section, level int, trail []string) []span {
	var pieces []span
	prev := s.start
	for i := range inner {
		h := &inner[i]
		if h.Level != level {
			continue
		}
		if h.Start > prev {
			pieces = append(pieces, span{start: prev, end: h.Start, trail: trail})
		}
		prev = h.Start
	}
	pieces = append(pieces, span{start: prev, end: s.end, trail: trail})
	if pieces[0].start == s.start {
		pieces[0].trail = s.trail
	}
	return pieces
}

// joinLead merges a lead piece into the piece after it when the two fit together.
//
// Only a lead that is not itself headed by a cut-level heading qualifies — that is
// the first piece when it starts at the span's own start and the span had content
// before its first cut.
func (c *cutter) joinLead(pieces []span, start int) []span {
	if len(pieces) < 2 || pieces[0].start != start {
		return pieces
	}
	// The lead's trail, because the joined span starts where the lead does and its
	// own heading is re-derived from that start when it is cut.
	joined := span{start: pieces[0].start, end: pieces[1].end, trail: pieces[0].trail}
	if c.tooBig(joined) {
		return pieces
	}
	return append([]span{joined}, pieces[2:]...)
}

// headingsIn returns the heading at s.start, if any, and every heading strictly
// inside s.
func (c *cutter) headingsIn(s span) (*markdown.Section, []markdown.Section) {
	var own *markdown.Section
	var inner []markdown.Section
	for i := range c.sections {
		h := &c.sections[i]
		switch {
		case h.Start == s.start && own == nil:
			own = h
		case h.Start > s.start && h.Start < s.end:
			inner = append(inner, *h)
		}
	}
	return own, inner
}

// tooBig reports whether s would exceed the limit as a body alone. Frontmatter and
// copied definitions are measured when the part is rendered; this is the cutting
// decision, and a margin for them is reserved here so the common case needs no
// second pass.
func (c *cutter) tooBig(s span) bool {
	return s.end-s.start > c.opt.Limit-frontmatterReserve
}

// shallowest returns the cut level for a span's inner headings: want when some
// heading has that level, otherwise the shallowest level present, or 0 when there
// are no headings.
func shallowest(inner []markdown.Section, want int) int {
	best := 0
	for i := range inner {
		if want != 0 && inner[i].Level == want {
			return want
		}
		if best == 0 || inner[i].Level < best {
			best = inner[i].Level
		}
	}
	return best
}

// appendTrail returns trail with title added, never aliasing trail's backing array:
// sibling spans share a parent trail, and an append that wrote into it would rename
// a sibling's heading.
func appendTrail(trail []string, title string) []string {
	out := make([]string, 0, len(trail)+1)
	out = append(out, trail...)
	return append(out, strings.TrimSpace(title))
}

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
