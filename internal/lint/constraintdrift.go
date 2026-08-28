package lint

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/textnorm"
)

// constraintDriftCheck reports a pinned constraint the prose no longer states.
//
// §10.2.1 makes prose authoritative and the constraint a derived reading of it, so drift
// is impossible by construction — with **one exception**, which is what this check
// covers. A `gnosis_constraint` pin exists for the case where a precise value is real but
// unreachable by the prose parser: a number in a table, a code fence, or a figure caption.
// A pin outranks the prose, and from that moment the two are separate representations
// that can disagree.
//
// **The mechanism is §10.2.1's, exactly**: render the pinned value and look for it in the
// claim's text under `textnorm.Fold`, having normalised spelled-out numbers first (§7.3)
// so "three" and "3" are one thing.
//
// **It cannot catch a paraphrase, and the finding says so.** §10.2.1 states that limit
// outright, and a reader who takes silence here as proof that a pin agrees with its prose
// will trust a number this check never really compared. A check that overstates what it
// verified is worse than one that does not run.
//
// **A warning and never a gate** (§10.2.1). Which side is wrong is a judgement: the pin
// may be right and the prose stale, or the prose may have been rewritten deliberately.
//
// **Day one it skips, and that is the argument for building it now.** No corpus has a
// pin, because §10.2.1 keeps them opt-in precisely so most claims never carry one. The
// path has to exist before the first pin, or the first pin is the one nobody checked —
// the same reasoning §5.8.3.1's episodic guard turned on.
func constraintDriftCheck() Check {
	return Check{
		Name:       "constraint-drift",
		Categories: []string{"constraint-drift"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimPinsItsConstraint,
		Run:        driftedPins,
	}
}

// someClaimPinsItsConstraint reports whether any claim states a pin to check.
func someClaimPinsItsConstraint(snap *Snapshot) (bool, string) {
	for id := range snap.Bounds {
		if snap.Bounds[id].Pinned {
			return true, ""
		}
	}
	return false, "no claim pins a gnosis_constraint, and a derived reading cannot drift" +
		" from the prose it was read out of"
}

// driftedPins reports each pinned claim whose text does not state the pinned value.
//
// Requires: bounds carry Pinned; documents carry their claims' anchors.
// Ensures: one diagnostic per claim, in document then claim order so two runs agree.
// Claims with no pin are skipped, and so are claims with no anchor — there is no text to
// look in, which is not the same as text that disagrees. Pure.
func driftedPins(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			b, ok := snap.Bounds[claim.ID]
			if !ok || !b.Pinned || strings.TrimSpace(claim.Anchor) == "" {
				continue
			}
			if statesValue(b.ProseText, b.Value) {
				continue
			}
			out = append(out, driftFinding(doc, claim, b))
		}
	}
	return out
}

// statesValue reports whether text states value.
//
// Requires: text is the claim's anchor with spelled-out numbers already normalised
// (§7.3), which the shell does — this package takes values and never the parser
// (PLAN §0.1), so "three" has become "3" before it arrives here.
// Ensures: a match only where the numeral is not part of a longer number, so a pin of 3
// is not satisfied by "30" or by "1.3". Pure.
//
// **The boundary is "not a digit and not a decimal point", not whitespace.** A unit is
// written against its number in the two dimensions that always carry one — "400ms",
// "5MB" — and requiring a space would report every duration and size pin in a corpus.
// That is the same defect the constraint parser shipped with and had to be taught out of;
// a check disagreeing with the parser about what a number looks like would be worse than
// no check, because the two would contradict each other on the same claim.
func statesValue(text string, value float64) bool {
	folded := strings.ToLower(textnorm.Fold(text))
	rendered := strconv.FormatFloat(value, 'f', -1, 64)

	for at := 0; ; {
		i := strings.Index(folded[at:], rendered)
		if i < 0 {
			return false
		}
		i += at
		if !partOfANumber(folded, i-1) && !partOfANumber(folded, i+len(rendered)) {
			return true
		}
		at = i + 1
	}
}

// partOfANumber reports whether the byte at i would extend a numeral.
func partOfANumber(text string, i int) bool {
	if i < 0 || i >= len(text) {
		return false
	}
	return text[i] >= '0' && text[i] <= '9' || text[i] == '.'
}

// driftFinding renders one mismatch, showing both representations.
func driftFinding(doc *Document, claim *Claim, b *Bound) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "constraint-drift",
		Path:     doc.Path,
		Message: "claim " + claim.ID + " pins " + strconv.Quote(b.Op+" "+
			strconv.FormatFloat(b.Value, 'f', -1, 64)) +
			" and its text does not state that value: " + strconv.Quote(claim.Anchor) +
			" — the pin outranks the prose (§10.2.1), so a reader gets the pinned" +
			" number and the text says something else; correct whichever is stale." +
			" This compares the value only and cannot see a paraphrase, so agreement" +
			" here is not proof the two say the same thing",
		Action: finding.ActionHuman,
	}
}
