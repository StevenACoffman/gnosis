// Package schema maintains the corpus's agent-facing schema document (SPEC §5.7):
// the generated parts of `AGENTS.md`, and the contract that keeps a person's prose
// out of the generator's way.
//
// # Why a marker contract rather than a generated file
//
// §5.7 records a caution rather than ignoring it: an ETH Zurich study found
// *auto-generated* context files reduced agent success in five of eight settings. So
// gnosis generates the mechanical parts only — the type vocabulary, the command list —
// and never the prose that tells an agent how to think. That division has to survive
// contact with a real file somebody has edited, which is what the markers are for.
//
// The same contract answers a second question, which is why two backlog entries closed
// together. §6.3 splits accretion from synthesis at the level of the *operation*; the
// finer split is at the level of the *region*, so a refresh rewrites what the machine
// wrote and leaves what a person wrote alone. The blocker recorded against that was
// "needs a way to mark regions", and this is the way.
//
// Everything here is pure. The caller reads and writes the file.
package schema

import (
	"strings"
)

// The marker forms. A region opens with begin and closes with end, both carrying the
// region's name.
//
// HTML comments, because they render as nothing in every Markdown viewer — a reader of
// the rendered file should not see the machinery — and because the surveyed tool
// already uses this form. A format in the wild beats one invented here: somebody who
// has seen `agents-md` recognises it, and nothing about a bespoke syntax would be
// better.
const (
	beginPrefix  = "<!-- gnosis:begin "
	endPrefix    = "<!-- gnosis:end "
	markerSuffix = " -->"
)

// The outcomes of a merge, with the one that asserts least as the zero value.
const (
	// OutcomeUnwritten is the zero value: no merge was performed. A default of
	// Merged would report an unattempted write as a completed one, which is the
	// collapse every other zero value in this codebase is placed to refuse.
	OutcomeUnwritten Outcome = iota

	// OutcomeMerged means every generated region was found and replaced, and
	// everything else was preserved.
	OutcomeMerged

	// OutcomeUnmarked means the existing text carries no markers at all, so nothing
	// may be written to it. The generated text belongs in a sibling file.
	OutcomeUnmarked

	// OutcomeMalformed means a marker opens and never closes. Nothing may be
	// written, because the extent of the region is unknown.
	OutcomeMalformed
)

// Outcome is what a merge did, or why it refused.
type Outcome int

// marker is one marker found in the text: its region name, and whether the marker was
// terminated at all.
//
// The flag exists because a half-written marker has no name, and "no name" had to stop
// meaning "no marker" — see Unclosed.
type marker struct {
	name       string
	terminated bool
}

// Region is one generated block: a name and the lines that belong between its markers.
type Region struct {
	Name string
	Body string
}

// Merged reports whether the result may be written to the existing file.
//
// True only for OutcomeMerged. Both refusals return a result that is *not* the file's
// new content — Unmarked returns the generated text for a sibling file and Malformed
// returns nothing — so a caller that wrote on anything but this would either clobber a
// document or write a fragment.
func (o Outcome) Merged() bool { return o == OutcomeMerged }

// String renders the outcome.
func (o Outcome) String() string {
	switch o {
	case OutcomeMerged:
		return "merged"
	case OutcomeUnmarked:
		return "unmarked"
	case OutcomeMalformed:
		return "malformed"
	case OutcomeUnwritten:
		return "unwritten"
	default:
		return "invalid"
	}
}

// MarshalText renders the outcome as a word in the machine envelope, so an agent
// branches on "unmarked" rather than on the integer 0.
func (o Outcome) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

// Render is the whole document as gnosis would write it from scratch, markers
// included.
//
// Requires: regions are the generated blocks, in the order they should appear.
// Ensures: text that Merge over it is a no-op — the output is its own fixed point, so
// `schema --check` on a freshly written file reports no drift. Pure.
func Render(preamble string, regions []Region) string {
	var b strings.Builder
	if preamble != "" {
		b.WriteString(strings.TrimRight(preamble, "\n"))
		b.WriteString("\n\n")
	}
	for i := range regions {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(marked(&regions[i]))
	}
	b.WriteString("\n")
	return b.String()
}

// Merge replaces each generated region in existing and preserves everything else.
//
// Requires: existing is the committed file's text, which may be empty; regions are the
// generated blocks.
// Ensures: pure. On OutcomeMerged the result is the file's new content. On
// OutcomeUnmarked it is the text for a *sibling* file and must not be written to
// `existing`'s path. On OutcomeMalformed it is empty.
//
// # The three rules, and the fourth the entry does not state
//
// **A generated region replaces the text between its markers.** The markers stay, so
// the next run finds the region again.
//
// **Everything outside every marker is preserved byte for byte.** Not re-rendered, not
// re-wrapped, not normalised. A person's prose is not the generator's to tidy, and a
// formatter that "improved" it would be the silent rewrite this contract exists to
// prevent — the same argument §5.8.1 makes for announcing before enforcing, applied to
// somebody's paragraph.
//
// **A file with no markers is never overwritten.** This is the fail-closed rule and the
// one the backlog entry singles out. A file predating the tool was not written under
// its contract, so treating its existence as consent is fail-open — and §5.7's caution
// gives the cost: auto-generated context files measurably *reduced* agent success in
// five of eight settings, so silently converting somebody's hand-written AGENTS.md into
// a generated one is a change with evidence against it.
//
// **A marker that opens and never closes is a refusal.** This one is not in the entry
// and the code needs it: reading an unterminated marker as "everything to the end of
// the file" would let a single typo hand a whole document to the generator. The extent
// of the region is unknown, so nothing is written and the caller is told which marker.
func Merge(existing string, regions []Region) (string, Outcome) {
	if !hasAnyMarker(existing) {
		// Nothing to merge into. The generated text is returned for a sibling file,
		// which is a different question from whether it may be written here.
		return Render("", regions), OutcomeUnmarked
	}
	if _, bad := unclosed(existing); bad {
		return "", OutcomeMalformed
	}

	out := existing
	for i := range regions {
		r := &regions[i]
		open, shut := markers(r.Name)
		start := strings.Index(out, open)
		if start < 0 {
			// A region gnosis generates and this file does not carry. Left absent
			// rather than appended: where a region belongs in somebody's document is
			// their decision, and a generator that inserted one would be choosing the
			// shape of a file it was told to leave alone.
			continue
		}
		end := strings.Index(out[start:], shut)
		if end < 0 {
			// Unreachable: unclosed() already refused. Guarded so a future edit that
			// removes that check cannot silently truncate.
			return "", OutcomeMalformed
		}
		out = out[:start] + marked(r) + out[start+end+len(shut):]
	}
	return out, OutcomeMerged
}

// Unclosed names the first region whose marker opens and never closes.
//
// Requires: nothing.
// Ensures: pure. `found` is the answer and `name` is for the diagnostic — **the two are
// separate because an empty name is a real answer**, not the absence of one. A
// half-written marker (`<!-- gnosis:begin vocabular`, no terminator) has no name to
// report, and the first version of this returned "" for both "nothing wrong" and "a
// nameless malformed marker", so the refusal was skipped exactly where the file was
// most damaged. Its own test caught it.
//
// Exported because the refusal is only actionable if the caller can say *which* marker;
// a caller told "malformed" with no name has to search the file.
func Unclosed(text string) (name string, found bool) { return unclosed(text) }

// marked renders one region with its markers, and **without a trailing newline**.
//
// That absence is load-bearing. Merge replaces exactly the span from the opening marker
// through the closing one, so a rendered region carrying its own trailing newline would
// leave the file's newline in place and add another — one blank line per run, growing
// forever, and `--check` reporting drift against a file it had just written. Its own
// test caught it.
func marked(r *Region) string {
	open, shut := markers(r.Name)
	body := strings.Trim(r.Body, "\n")
	if body == "" {
		return open + "\n" + shut
	}
	return open + "\n" + body + "\n" + shut
}

// markers are the opening and closing lines for a region.
func markers(name string) (open, shut string) {
	return beginPrefix + name + markerSuffix, endPrefix + name + markerSuffix
}

// hasAnyMarker reports whether the text carries a gnosis region marker of any name.
//
// Any *begin* marker counts. A file carrying only an end marker is malformed rather
// than unmarked, and unclosed() is what reports it — treating it as unmarked would
// route it to the sibling file and leave the damage in place unremarked.
func hasAnyMarker(text string) bool {
	return strings.Contains(text, beginPrefix) || strings.Contains(text, endPrefix)
}

// unclosed finds a begin marker with no matching end, or an end with no begin.
//
// Both directions, because both mean the same thing to a writer: the extent of the
// region cannot be determined from the file, so the file may not be written.
func unclosed(text string) (string, bool) {
	for _, n := range names(text, beginPrefix) {
		if !n.terminated {
			return "", true
		}
		if _, shut := markers(n.name); !strings.Contains(text, shut) {
			return n.name, true
		}
	}
	for _, n := range names(text, endPrefix) {
		if !n.terminated {
			return "", true
		}
		if open, _ := markers(n.name); !strings.Contains(text, open) {
			return n.name, true
		}
	}
	return "", false
}

// names lists the markers appearing after the given prefix, in order.
func names(text, prefix string) []marker {
	var out []marker
	rest := text
	for {
		at := strings.Index(rest, prefix)
		if at < 0 {
			return out
		}
		rest = rest[at+len(prefix):]
		end := strings.Index(rest, markerSuffix)
		if end < 0 {
			// A marker prefix with no terminator at all. It is not a region and it
			// is not nothing: a file containing a half-written marker is one nobody
			// should write over, because whoever was editing it was interrupted.
			return append(out, marker{terminated: false})
		}
		out = append(out, marker{name: strings.TrimSpace(rest[:end]), terminated: true})
		rest = rest[end+len(markerSuffix):]
	}
}

// RegionBody returns the generated text between a region's markers.
//
// Requires: text is a document that may or may not carry markers.
// Ensures: false when the region is absent or its opening marker is never closed — an
// unterminated marker has no knowable extent, which is the same refusal `Merge` makes
// for the same reason. Pure.
//
// **The operation this package has been missing.** It could render regions and merge
// them and refuse a broken one, and could not read one back — so every consumer that
// wanted to know what a region *says* had to re-implement the marker scan.
func RegionBody(text, name string) (string, bool) {
	open, shut := markers(name)
	start := strings.Index(text, open)
	if start < 0 {
		return "", false
	}
	rest := text[start+len(open):]
	end := strings.Index(rest, shut)
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(rest[:end]), true
}
