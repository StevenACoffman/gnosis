// Package scan implements SPEC §9.3's admission security checks over text.
//
// **Ingested text is text an agent will obey.** A poisoned upstream page filed
// into the corpus is a durable prompt injection carrying the team's own authority —
// it arrives in a document that passed review, sits beside claims somebody
// verified, and is retrieved by the same search that returns everything else. That
// is the whole reason this stage exists and the reason it runs before any model
// sees the content rather than after.
//
// # This package implements stage 1 of four, and says so
//
// §9.3 lists hidden characters, injection and exfiltration patterns, secrets, and
// oversize. Only the first is here. The other three each need a decision this
// package cannot make on its own — a pattern corpus with its own test set, a
// `betterleaks` dependency, a bound in `standards/` — and shipping them together
// would produce one change nobody could review. A caller must not read a clean
// scan as "§9.3 passed"; it means "no hidden characters", and `Stages` says which
// stages ran so that a report can be honest about it.
//
// # Why these constants are allowed to block
//
// §9.3 makes the point and it is worth repeating where the constants live: **these
// are codepoint ranges from the Unicode standard, not tuned thresholds.** A gate
// that blocks on a number somebody chose invites arguing the number down until the
// gate is quiet. There is no version of this check that is 30% less strict. A
// zero-width space either is or is not U+200B.
//
// Everything here is pure.
package scan

import (
	"fmt"
	"sort"
)

// The hidden-character classes of SPEC §9.3.
//
// ClassUnset is the zero value and names no class, so a Finding nobody populated
// cannot be mistaken for a report about zero-width characters.
const (
	ClassUnset Class = ""

	// ClassZeroWidth is text that occupies no space and survives copy-paste.
	// Instructions written in it are invisible in every reviewer's editor.
	ClassZeroWidth Class = "zero-width"

	// ClassBidi is the Trojan Source class: bidirectional overrides that make
	// rendered order differ from stored order, so what a reviewer reads is not
	// what a parser or a model consumes.
	ClassBidi Class = "bidi-override"

	// ClassTag is the Unicode tag block, deprecated for language tagging and
	// repurposed as a channel for text no renderer displays at all.
	ClassTag Class = "unicode-tag"
)

// Class names a kind of hidden character.
type Class string

// Finding is one class of hidden character found in one text.
//
// One per class rather than one per occurrence: a document with four hundred
// zero-width joiners has one problem, and four hundred findings would bury it.
// Count and Offset locate it well enough to act.
type Finding struct {
	Class Class `json:"class"`

	// Rune is the first codepoint of this class encountered, rendered as U+XXXX.
	Rune string `json:"rune"`

	// Offset is the byte offset of that first occurrence, in the same space as
	// claims.pos and links.snippet_start (§5.5.2).
	Offset int `json:"offset"`

	// Count is how many characters of this class the text holds.
	Count int `json:"count"`
}

// Hidden reports the hidden-character classes present in text.
//
// Requires: nothing. Empty text and invalid UTF-8 are both valid inputs.
// Ensures: one Finding per class present, ordered by class name so two runs over
// one text produce comparable output. Empty rather than nil when the text is
// clean, so a caller need not distinguish "no findings" from "did not run" — that
// distinction belongs to Stages. Pure.
//
// Invalid UTF-8 is not a hidden character and is not reported here. A source that
// is not valid UTF-8 fails the archive's text test (§4.3) and never reaches this,
// so treating a decode error as a finding would report the same problem twice
// under a name that does not fit it.
func Hidden(text string) []Finding {
	byClass := map[Class]*Finding{}

	for offset, r := range text {
		class := classify(r)
		if class == ClassUnset {
			continue
		}
		if f, seen := byClass[class]; seen {
			f.Count++
			continue
		}
		byClass[class] = &Finding{
			Class: class, Rune: codepoint(r), Offset: offset, Count: 1,
		}
	}

	out := make([]Finding, 0, len(byClass))
	for _, f := range byClass {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Class < out[j].Class })
	return out
}

// classify reports which hidden class a rune belongs to, or ClassUnset.
//
// The ranges are from the Unicode standard. They are listed as literals rather
// than derived from a table so that a reader can check them against the standard
// without running anything, which is the property that makes them safe to block on.
func classify(r rune) Class {
	switch {
	case r == 0x200B, r == 0x200C, r == 0x200D, r == 0x2060, r == 0xFEFF:
		return ClassZeroWidth
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return ClassBidi
	case r >= 0xE0001 && r <= 0xE007F:
		return ClassTag
	default:
		return ClassUnset
	}
}

// codepoint renders a rune the way the Unicode standard writes one, so a finding
// can be looked up rather than only recognised. Four digits minimum, more when the
// codepoint needs them: U+200B and U+E0001 are both correct.
func codepoint(r rune) string { return fmt.Sprintf("U+%04X", r) }

// Stages names the §9.3 stages this build actually runs.
//
// Requires: nothing.
// Ensures: the stages implemented, not the stages specified. A caller reporting a
// clean scan MUST say which stages produced it — "no hidden characters" and "§9.3
// passed" are different claims, and only one of them is currently available.
func Stages() []string { return []string{"hidden-characters"} }
