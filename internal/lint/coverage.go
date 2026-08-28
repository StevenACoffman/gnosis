package lint

import (
	"strconv"

	"github.com/StevenACoffman/skillet/finding"
)

// universalMinQuotes is how many quotations a universal claim must offer before this
// check stays quiet.
//
// **One, and the number is doing less work than it looks.** §6.2 forbids a threshold
// with no basis, and this is not really a threshold: §17.3.1's example is a claim
// resting on a *single* piece of evidence, and the mismatch it describes is between an
// unqualified assertion and one supporting passage. Two is where a reader would have to
// start arguing about sufficiency, and this check deliberately stops before that — it
// reports the case §17.3.1 states and no case it invented.
const universalMinQuotes = 1

// coverageCheck reports a claim asserting more strongly than its evidence supports.
//
// §17.3.1: relevance is the *quality* of a claim's evidence and sufficiency is its
// *quantity*. A quotation that validates is on-topic by construction, so the corpus
// checks the first and had no account of the second — nothing asked whether a claim's
// evidence supports the **scope it claims**.
//
// The rule is that sufficiency scales with the strength of the assertion. One
// photograph of somebody at an art shop the day a painting sold may be sufficient for
// *John may have bought the painting* and is plainly insufficient for *John definitely
// bought the painting*. The evidence did not change; the claim did.
//
// **A warning and never a rejection**, because the remedy is usually to weaken the claim
// rather than to find more evidence, and that is an author's decision. A gate that
// rejected the claim would push the author toward deleting the qualifier rather than
// adding the caveat — the corpus would get quieter and less true at once.
//
// **A hedged claim is silent even on one quotation.** A claim saying "typically" has
// already done what this finding would ask of it, and reporting it anyway would teach
// authors that hedging buys nothing.
func coverageCheck() Check {
	return Check{
		Name:       "coverage",
		Categories: []string{"coverage"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimOffersEvidence,
		Run:        overAssertedClaims,
	}
}

// someClaimOffersEvidence reports whether there is anything to weigh, and says which of
// the three ways there is not.
//
// Requires: nothing.
// Ensures: a reason whenever it declines, naming the missing thing rather than the
// check. Pure.
//
// **The third case is the one that matters.** This check compares an assertion's
// strength against the evidence behind it, so a corpus whose claims declare no evidence
// at all has nothing to compare — and reporting every claim in it as under-evidenced
// would be reporting the absence of the measurement as a fault in the thing measured.
func someClaimOffersEvidence(snap *Snapshot) (bool, string) {
	if !snap.Strength.Declared() {
		return false, "standards/strength.toml declares no claim-strength markers"
	}
	for i := range snap.Documents {
		for _, claim := range snap.Documents[i].Claims {
			if len(claim.Quotes) > 0 {
				return true, ""
			}
		}
	}
	return false, "no claim declares evidence yet, so nothing states a scope to weigh"
}

// overAssertedClaims reports each claim whose wording outruns its evidence.
//
// Requires: the strength markers are loaded.
// Ensures: one diagnostic per claim. Claims with no evidence at all are skipped — an
// unevidenced claim is `conformance`'s business, and reporting it here would say its
// evidence is thin when it has none. Pure.
func overAssertedClaims(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		normative := isNormative(snap, doc)
		for j := range doc.Claims {
			if d := overAsserted(snap, doc, &doc.Claims[j], normative); d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// isNormative reports whether this document's type prescribes.
//
// §14.4.1's stakes rule: a universal assertion the corpus leans on is where being wrong
// costs most. The standard differs, not the truth — "a person who knows a door is locked
// when a colleague's jacket is inside will say they do not know it when an armed
// intruder is being hunted, and both answers are correct."
func isNormative(snap *Snapshot, doc *Document) bool {
	declared, ok := snap.Vocabulary.TypeNamed(doc.Type)
	return ok && declared.Normative
}

// overAsserted reports one claim's mismatch, or nil when there is none.
func overAsserted(
	snap *Snapshot, doc *Document, claim *Claim, normative bool,
) *finding.Diagnostic {
	if len(claim.Quotes) == 0 {
		return nil
	}
	marker, asserts := snap.Strength.Asserts(claim.Anchor)
	if !asserts {
		return nil
	}
	if snap.Strength.Hedges(claim.Anchor) {
		// Both a universal and a hedge: the claim qualified itself, and which one
		// governs is a reading no lexical check can settle. Saying nothing is the
		// honest answer, and it is the same restraint §17 asks for elsewhere.
		return nil
	}
	if len(claim.Quotes) > universalMinQuotes {
		return nil
	}
	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "coverage",
		Path:     doc.Path,
		Message: "claim " + claim.ID + " asserts " + strconv.Quote(marker) + " on " +
			noun(len(claim.Quotes), "quotation") + stakes(normative) +
			"; weaken the wording, or add evidence for the scope it claims —" +
			" a claim that is genuinely universal and hard to source is worth saying" +
			" more carefully rather than less",
		Action: finding.ActionHuman,
	}
}

// stakes marks a claim of a prescribing type, for triage.
//
// **§14.4.1's rule is expressed here and not as a second threshold, and the first
// version got that wrong.** It raised the bar by one for a normative type, which
// silently required *three* quotations — a number §17.3.1 never states and §6.2
// forbids inventing. There is one comparison in that section and this check implements
// exactly it; the stakes appear where they can be acted on, in a message that tells a
// reader triaging a queue which findings to open first.
func stakes(normative bool) string {
	if !normative {
		return ""
	}
	return " — this type prescribes, so a wrong universal costs more (§14.4.1)"
}

// rungCheck reports a claim asserting causation on evidence that only observes.
//
// §17.3.1.1: Pearl's ladder names three rungs — association (seeing), intervention
// (doing), counterfactual (imagining) — and a claim whose wording is causal while its
// evidence is observational is asserted at one strength and supported at another. That is
// the same silent upgrade §9.4 refuses for quotations, one axis over from `coverage`'s
// quantifier rule.
//
// **This is a second axis of coverage, not a subsystem**, and it is registered separately
// only because §12's applicability rule is per check: a check that silently declines half
// of itself is indistinguishable from one that found nothing. Two names means two
// reasons, and a corpus that declares strength markers but no registers is told which
// half is unavailable.
//
// **The rung is stored nowhere and both sides are read here.** §17.3.1.1 refuses a
// declared `rung` field as self-certification, refuses the constraint because a constraint
// is a quantity and "restarting the pod clears the leak" carries no operator, and refuses
// `links.rel` because causality is carried as a claim rather than as a relation between
// documents. A single stored rung could not express a *gap*, and the gap is what the
// interesting case always is.
//
// **The evidence's rung comes from the quotation, never from the archived file.** A whole
// source almost certainly contains a causal word somewhere, so reading the file would
// clear nearly every claim while appearing to check it — the loudest possible way to be
// useless. The quotation is also what the corpus already validates verbatim, so it is the
// only text a finding can honestly attribute to the source.
//
// Warning tier and never a rejection, for the reason the quantifier axis gives: the
// remedy is usually to weaken the wording, and that is an author's decision.
func rungCheck() Check {
	return Check{
		Name:       "rung",
		Categories: []string{"rung"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimQuotesUnderDeclaredRegisters,
		Run:        overClaimedCausation,
	}
}

// someClaimQuotesUnderDeclaredRegisters reports whether there is anything to weigh.
func someClaimQuotesUnderDeclaredRegisters(snap *Snapshot) (bool, string) {
	if !snap.Registers.Declared() {
		return false, "standards/registers.toml declares no causal registers"
	}
	for i := range snap.Documents {
		for _, claim := range snap.Documents[i].Claims {
			if len(claim.Quotes) > 0 {
				return true, ""
			}
		}
	}
	return false, "no claim declares evidence yet, so no claim's support states a rung"
}

// overClaimedCausation reports each claim whose wording outruns its evidence's rung.
//
// Requires: the registers are loaded.
// Ensures: one diagnostic per claim, and none for a claim that said nothing causal or
// whose quotations said nothing either way. Pure.
func overClaimedCausation(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			if d := rungGap(snap, doc, &doc.Claims[j]); d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// rungGap reports one claim's mismatch, or nil when there is none.
//
// **Silence is the answer in three of the four states, and each for its own reason.** A
// claim with no causal wording is not making a causal claim. A claim already on the
// association rung matches whatever observed it. And a quotation with no register word
// has not been *shown* to be observational — reporting it would assert from absence,
// which is the move §9.4's `Unchecked` outcome exists to refuse one layer down.
func rungGap(snap *Snapshot, doc *Document, claim *Claim) *finding.Diagnostic {
	if len(claim.Quotes) == 0 {
		return nil
	}
	rung, marker, stated := snap.Registers.Rung(claim.Anchor)
	if !stated || rung != "intervention" {
		return nil
	}

	// Every quotation that said anything must have observed. One quotation asserting
	// intervention supports the claim, and the gap is gone.
	observed := ""
	for _, q := range claim.Quotes {
		quoteRung, quoteMarker, quoteStated := snap.Registers.Rung(q)
		if !quoteStated {
			continue
		}
		if quoteRung != "association" {
			return nil
		}
		if observed == "" {
			observed = quoteMarker
		}
	}
	if observed == "" {
		return nil
	}

	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "rung",
		Path:     doc.Path,
		Message: "claim " + claim.ID + " asserts " + strconv.Quote(marker) +
			" on evidence that says " + strconv.Quote(observed) +
			"; the claim is causal and its support is observational — weaken the" +
			" wording to what was seen, or cite evidence of the intervention",
		Action: finding.ActionHuman,
	}
}
