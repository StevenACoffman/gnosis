package gnosis

import "strings"

// minTemplateWords is how long a phrase must be before finding it inside a
// rationale is evidence of anything.
//
// A short phrase recurs by accident. "as --rationale" appearing in somebody's
// reasoning says nothing, and refusing it would teach them to reword rather than to
// think — the opposite of what §10.6.4 is for. Six words is `quotecheck`'s own
// threshold for a passage that can serve as evidence, reused rather than re-chosen,
// because the question is the same one: is this enough text to be a match rather
// than a coincidence.
const minTemplateWords = 6

// PriorRationale is a rationale already on the record, and how a diagnostic should
// refer to it.
//
// Label is supplied by the caller rather than assembled here, because what names an
// earlier decision is whatever the caller can actually look up — an actor and a date
// from the audit trail today, a warrant identifier when §10.6.5 lands. A domain
// function that formatted timestamps would be choosing for both.
type PriorRationale struct {
	Label string
	Text  string
}

// UnusableRationale reports why a rationale may not be accepted, or "" when it may.
//
// Requires: asked are the phrases the tool itself put in front of the author — its
// own instructions and templates; prior are the rationales already recorded for
// whatever this one is about. Either may be empty.
// Ensures: "" for an acceptable rationale and for an empty one; otherwise one
// sentence naming what matched. Pure — no clock, no disk, and the prior rationales
// are a parameter, so the caller does the reading (§4.6).
//
// # What this defends, and why non-empty is not enough
//
// §10.6.4 bets that a required rationale filters more bad adjudications than a
// permission check, because somebody who cannot articulate a reason usually stops
// before finishing the sentence. That bet has a known way of losing, and it is not
// the one a reviewer expects: not a bad reason, which is legible and arguable, but
// **the field being satisfied without being used.** A surveyed system requires a
// decision rationale, enforces non-empty by schema, and then had to warn its own
// agents in prose that they were emitting the template text verbatim. Non-empty is a
// check on length, and the thing being defended against has length.
//
// # The two refusals, and why they match differently
//
// **Template text is matched by containment**, because the workaround for equality
// is adding a word. §10.6.4 names that workaround in its argument for quoting the
// match, so a check that equality could defeat would be answering the wrong
// objection. Only phrases of at least minTemplateWords count, so a flag name or a
// fragment cannot condemn a real sentence.
//
// **A prior rationale is matched by equality**, folded. Two decisions may
// legitimately rest on the same reason, and the honest way to say so is a reference
// to the first one rather than a second copy of its prose — so the refusal names the
// earlier one, which makes writing that reference the easy path. Containment would be
// wrong here: a rationale that quotes an earlier one and then says why this case
// differs is exactly what should be encouraged.
//
// Folding is `Surface.Fold` for both — `textnorm.Fold` plus case — and the case half
// is a deliberate departure from the quotation guards. `textnorm` preserves case on
// the argument that "a quotation differing only in case is a different quotation",
// which is right for evidence and wrong here: a rationale is not evidence, and
// "State why you are promoting…" is the same boilerplate as "state why you are
// promoting…". Capitalising one letter is the cheapest evasion there is, and a
// case-sensitive check would be defeated by it. Reusing `Surface.Fold` rather than
// writing a second normaliser is the point — a second definition of "the same words"
// is the drift the shared kernel exists to prevent. It is what carries the rest of
// the folding too, so re-wrapping a line, straightening a quotation mark, or the
// non-breaking space a copy-paste leaves behind defeats neither refusal.
//
// # Two limits, stated because the check must not be over-trusted
//
// It cannot detect original prose that says nothing. No mechanical check can, and
// §17's refusal to score means gnosis will not pretend otherwise.
//
// It is for the field that carries *reasoning*. §10.6.4 excludes `override.reason`
// by name, where "marcus on leave until 09-02" is a complete and correct answer that
// will legitimately recur, and a caller must not pass one here.
func UnusableRationale(rationale string, asked []string, prior []PriorRationale) string {
	folded := Surface(rationale).Fold()
	if folded == "" {
		// Presence is a different check with a different message, and it already
		// exists where the other authorisation requirements are named. Two checks
		// reporting the same emptiness would report it twice.
		return ""
	}

	if phrase := templateIn(folded, asked); phrase != "" {
		return "this rationale repeats what the tool asked for (" + quoted(phrase) +
			"); state the reason in your own words"
	}
	for _, p := range prior {
		if Surface(p.Text).Fold() == folded {
			return "this rationale is identical to the one " + p.Label +
				" already recorded; reference that decision instead of restating it"
		}
	}
	return ""
}

// templateIn is the first of the tool's own phrases that appears in a folded
// rationale, or "".
//
// Requires: folded is already normalised; asked are raw.
// Ensures: pure. Phrases shorter than minTemplateWords are skipped, and a caller
// therefore cannot make the check stricter by passing a fragment.
func templateIn(folded string, asked []string) string {
	for _, phrase := range asked {
		if len(strings.Fields(phrase)) < minTemplateWords {
			continue
		}
		if f := Surface(phrase).Fold(); f != "" && strings.Contains(folded, f) {
			return phrase
		}
	}
	return ""
}

// quoted renders a matched phrase for a diagnostic, shortened if it is long.
//
// §10.6.4 requires the match to be shown: "a diagnostic that says 'rationale
// rejected' without showing what it matched is a diagnostic somebody works around by
// adding a word." Shortening keeps that legible without turning one refusal into a
// paragraph.
func quoted(phrase string) string {
	const width = 60
	if len(phrase) <= width {
		return `"` + phrase + `"`
	}
	// Cut at a word boundary. Running the command showed the naive cut landing
	// mid-word — "could not f…" — which reads as a bug in the tool rather than as
	// an elision, and this text's whole job is to be recognised by the person who
	// typed it.
	cut := phrase[:width]
	if at := strings.LastIndexByte(cut, ' '); at > 0 {
		cut = cut[:at]
	}
	return `"` + cut + `…"`
}
