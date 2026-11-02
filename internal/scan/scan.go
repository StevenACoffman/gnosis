// Package scan implements SPEC §9.3's admission security checks over text.
//
// **Ingested text is text an agent will obey.** A poisoned upstream page filed
// into the corpus is a durable prompt injection carrying the team's own authority —
// it arrives in a document that passed review, sits beside claims somebody
// verified, and is retrieved by the same search that returns everything else. That
// is the whole reason this stage exists and the reason it runs before any model
// sees the content rather than after.
//
// # This package implements stages 1 to 3 of four, and says so
//
// §9.3 lists hidden characters, injection and exfiltration patterns, secrets, and
// oversize. The first three are here; the fourth is not, for the reason
// Ruleset.Coverage gives — the bound exists for a fetched source and not for a
// candidate document, and one function cannot report it truthfully for both. A
// caller must not read a clean scan as "§9.3 passed", and `Coverage` says which
// stages ran so a report can be honest about it.
//
// The earlier form of this was a `Stages()` returning the one implemented name,
// and nothing ever called it — an honesty mechanism declared and read by nobody,
// which is the same failure `standards.Unread` was built to report one layer up.
// Coverage is consumed: the promote gate's `security` signal reports Unchecked
// when stages are missing, which is what routes a candidate to a human.
//
// # Why these constants are allowed to block
//
// §9.3 makes the point and it is worth repeating where the constants live: **these
// are codepoint ranges from the Unicode standard, not tuned thresholds.** A gate
// that blocks on a number somebody chose invites arguing the number down until the
// gate is quiet. There is no version of this check that is 30% less strict. A
// zero-width space either is or is not U+200B.
//
// A *pattern* does not get that standing for free, which is why stages 2 and 3
// arrive as a Ruleset that self-tests at load: every rule carries a case it must
// flag and a case it must not, and one that fails either takes the whole ruleset
// down rather than being loaded on trust. See LoadRules.
//
// # The classifier is skillet's
//
// The hidden-character classes and their context rules live in skillet's `hidden`
// package, shared with exegesis. gnosis carried its own copy until 2026-11-02, and it
// was wrong in the same ways as exegesis's: it refused every zero-width joiner, so a
// source holding a ZWJ emoji sequence was not archived, and the whole tag block, so
// the England flag was refused. The shared scan excuses a joiner between non-ASCII
// characters, the three flag tag sequences, and a byte-order mark at offset 0, and
// lists six codepoints this copy did not, among them the soft hyphen and the
// invisible math operators.
//
// Everything here is pure. The rules are data; the caller supplies the Ruleset.
package scan

import "github.com/StevenACoffman/skillet/hidden"

// The hidden-character classes of SPEC §9.3. The values are skillet's, which are the
// names gnosis has always stored.
//
// ClassUnset is the zero value and names no class, so a Finding nobody populated
// cannot be mistaken for a report about zero-width characters.
const (
	ClassUnset     Class = ""
	ClassZeroWidth       = hidden.ClassZeroWidth
	ClassBidi            = hidden.ClassBidi
	ClassTag             = hidden.ClassTag
)

// Class names a kind of hidden character.
type Class = hidden.Class

// Finding is one class of hidden character found in one text: the first codepoint as
// U+XXXX, its byte offset (in the same space as claims.pos and links.snippet_start,
// §5.5.2), and how many characters of the class the text holds.
//
// One per class rather than one per occurrence: a document with four hundred
// zero-width characters has one problem, and four hundred findings would bury it.
type Finding = hidden.Summary

// Hidden reports the hidden-character classes present in text.
//
// Requires: nothing. Empty text and invalid UTF-8 are both valid inputs.
// Ensures: one Finding per class present, ordered by class name so two runs over
// one text produce comparable output. Empty rather than nil when the text is
// clean, so a caller need not distinguish "no findings" from "did not run" — that
// distinction belongs to Coverage. Pure.
//
// Invalid UTF-8 is not a hidden character and is not reported here. A source that
// is not valid UTF-8 fails the archive's text test (§4.3) and never reaches this,
// so treating a decode error as a finding would report the same problem twice
// under a name that does not fit it.
func Hidden(text string) []Finding { return hidden.ByClass(hidden.Scan(text)) }
