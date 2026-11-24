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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

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

	// MinBytes, when positive, joins a piece smaller than this to its neighbour
	// whenever the two fit under Limit together, so a book whose conversion
	// flattened its sections does not become dozens of near-empty parts. Zero keeps
	// every section its own part. Recorded in each part when set, so a re-run can
	// be reproduced from the file.
	MinBytes int

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

	// joined marks small sections joinSmall put together on purpose: the first
	// heading owns the span although its siblings follow it, so the part is named
	// for the first section it holds.
	joined bool
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
	MinBytes int      `yaml:"min_part_bytes,omitempty"`

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
// A heading starting exactly at s.start that encloses s is s's own and is not a cut
// point; it joins the trail instead. Cutting at the shallowest level first is what
// makes the parts follow the book's structure — parts, then chapters, then sections —
// rather than an arbitrary byte count.
//
// The lead — whatever sits between s.start and the first cut, typically a part title
// and its short introduction — is joined to the first piece when the two fit
// together, so a part's opening paragraph is not archived as a file of its own. A
// source opening on front matter that is a sibling of the headings after it, such as
// "# Preface" before "# Chapter 1", has no lead: its first piece is a section like
// any other.
//
// Requires: 0 <= s.start <= s.end <= len(c.src).
// Ensures: the returned spans tile [s.start, s.end) in order.
func (c *cutter) cut(s span, level int) []span {
	own, inner := c.headingsIn(s)
	trail := s.trail
	if own != nil {
		trail = appendTrail(trail, own.Title)
	}
	if !c.tooBig(s) && level == 0 && fitsWhole(own, inner) {
		return []span{{start: s.start, end: s.end, trail: trail}}
	}

	cutLevel := shallowest(inner, level)
	if cutLevel == 0 {
		return []span{{start: s.start, end: s.end, trail: trail}}
	}

	pieces := atLevel(s, inner, cutLevel, trail)
	// Under a forced level the lead stays separate: joining it could fold a
	// two-chapter book back into one file and defeat the option.
	if level == 0 && hasLead(s, own, inner, cutLevel) {
		pieces = c.joinLead(pieces)
	}
	pieces = c.joinSmall(pieces)

	var out []span
	for _, p := range pieces {
		// The level option applies once, at the top: below it only size decides.
		out = append(out, c.cut(p, 0)...)
	}
	return out
}

// fitsWhole reports whether a span holding own and inner may be one part when it
// fits: it is one heading's section, or, having no heading of its own, holds no
// heading shallower than its first.
//
// Sections with no common parent — a book's front matter before its first chapter —
// may share a part whose trail names none of them. A shallower heading among them,
// "### Foreword" then "## Part I", may not: it opens a section that trail would hide,
// so such a span is cut whatever its size.
func fitsWhole(own *markdown.Section, inner []markdown.Section) bool {
	return own != nil || len(inner) == 0 ||
		!slices.ContainsFunc(inner[1:], func(h markdown.Section) bool {
			return h.Level < inner[0].Level
		})
}

// hasLead reports whether s's first piece, cutting at cutLevel, is a lead: the
// introduction to s's own heading, or text before any heading.
//
// Requires: inner is non-empty.
func hasLead(s span, own *markdown.Section, inner []markdown.Section, cutLevel int) bool {
	return own != nil || inner[0].Start > s.start && inner[0].Level == cutLevel
}

// atLevel cuts s before every inner heading of the given level.
//
// A piece headed by a cut heading sits under s's own heading, so it inherits trail,
// which includes it. The first piece starts at s.start and adds its own heading itself
// when it is cut, so it inherits s.trail, which does not.
//
// Requires: some heading in inner has the given level and starts after s.start, so at
// least one cut falls strictly inside s.
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
	pieces[0].trail = s.trail
	return pieces
}

// joinLead merges a lead piece into the piece after it when the two fit together.
//
// Requires: pieces[0] is a lead, as hasLead reports, so len(pieces) >= 2.
func (c *cutter) joinLead(pieces []span) []span {
	// The lead's trail, because the joined span starts where the lead does and its
	// own heading is re-derived from that start when it is cut.
	joined := span{start: pieces[0].start, end: pieces[1].end, trail: pieces[0].trail}
	if c.tooBig(joined) {
		return pieces
	}
	return append([]span{joined}, pieces[2:]...)
}

// joinSmall joins each piece smaller than MinBytes to the piece before it, and a
// small piece to the one after it, whenever the two fit under the limit together.
//
// Requires: pieces tile a span in order.
// Ensures: the result tiles the same span; no join produces a piece over the limit;
// with MinBytes zero, pieces is returned unchanged. Pure.
//
// The joined piece keeps the first piece's trail, so it is named for the first
// section it holds. Terraform: Up & Running is the case it was written for: its
// conversion put every section at one level, and it split into 81 parts, 13 of them
// too small to hold a single claim.
func (c *cutter) joinSmall(pieces []span) []span {
	if c.opt.MinBytes <= 0 || len(pieces) < 2 {
		return pieces
	}
	small := func(s span) bool { return s.end-s.start < c.opt.MinBytes }
	out := []span{pieces[0]}
	for _, p := range pieces[1:] {
		last := out[len(out)-1]
		joined := span{start: last.start, end: p.end, trail: last.trail, joined: true}
		if (small(last) || small(p)) && !c.tooBig(joined) {
			out[len(out)-1] = joined
			continue
		}
		out = append(out, p)
	}
	return out
}

// headingsIn returns s's own heading, if it has one, and every other heading in s.
// A heading at s.start that is not s's own is returned in inner, as a cut point like
// the rest.
func (c *cutter) headingsIn(s span) (*markdown.Section, []markdown.Section) {
	var first *markdown.Section
	var inner []markdown.Section
	for i := range c.sections {
		h := &c.sections[i]
		switch {
		case h.Start == s.start && first == nil:
			first = h
		case h.Start > s.start && h.Start < s.end:
			inner = append(inner, *h)
		}
	}
	if first == nil || owns(s, first, inner) {
		return first, inner
	}
	return nil, append([]markdown.Section{*first}, inner...)
}

// owns reports whether first, the heading at s.start, is s's own.
//
// It is when every later heading in s is deeper: then s is that heading's section.
// When a later heading is at its level or above, the two are siblings — "# Preface"
// then "# Chapter 1" — and first does not enclose the book.
//
// Two spans are owned by their first heading regardless. One joinSmall joined holds
// siblings on purpose. And a span opening the source on anything but front matter
// opens on the book's title: conversions often set a title at the level of the
// chapters, and it still encloses them.
func owns(s span, first *markdown.Section, inner []markdown.Section) bool {
	return s.joined || s.start == 0 && !frontMatter(first.Title) ||
		allDeeper(inner, first.Level)
}

// allDeeper reports whether every heading in hs is deeper than level, so that hs
// all fall within the section of a heading at that level.
func allDeeper(hs []markdown.Section, level int) bool {
	return !slices.ContainsFunc(hs, func(h markdown.Section) bool { return h.Level <= level })
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
