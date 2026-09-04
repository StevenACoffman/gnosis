package lint

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// warrantCheck reports an adjudicated claim whose decision is not recorded (§12, §10.6.4).
//
// §10.4's second provenance class is the largest one a group of experienced
// practitioners produces: not the published research, but *which* of it this team
// treats as authoritative and how it is weighed against local constraints. Such a claim
// can carry no quote by construction, so it fails the evidence invariant by
// construction — the highest-value artifact the team produces, rejected by the check
// that exists to protect quality. The warrant is what admits it, and this check is what
// keeps the warrant from being a formality.
//
// **§12's row names two findings and they are found by different evidence**, which is
// worth stating because only one of them looks like a check:
//
//   - **A warrant with no rationale.** The field §10.6.4 calls the real gate. A
//     permission bit asks whether somebody may decide; the rationale asks them to write
//     down why, and someone who cannot articulate a reason usually stops before
//     finishing the sentence.
//   - **A supersession with no warrant.** §10.4 gives the winner of an adjudicated
//     conflict a `gnosis_supersedes` edge, so a claim carrying one *was* adjudicated —
//     and if it carries no warrant, the decision that displaced another claim was never
//     written down. That is the only structural evidence of adjudication the corpus
//     holds.
//
// **There is deliberately no `class: adjudicated` key**, which would be the obvious way
// to make the first half of §12's row fire more widely. A claim could then assert the
// class and carry no decision, so the assertion and the evidence for it would be the
// same field under two names — and the check would be reporting what an author typed
// rather than what the corpus did. What is not reported, and cannot be: an ordinary
// hand-written claim that *should* have been adjudicated and simply says nothing. No
// mechanical check decides that, and §17 keeps gnosis from pretending otherwise.
func warrantCheck() Check {
	return Check{
		Name:       "warrant",
		Categories: []string{"warrant"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimIsAdjudicated,
		Run:        unwarrantedDecisions,
	}
}

// coSignCheck reports an escalated claim missing a required co-signer (§10.6).
//
// Authority scales with the adjudicators the corpus actually has (§10.6.1), and at
// `paired` or `quorum` an escalated claim needs a second signature. "Escalated" is
// already computable and needs no new judgment: load-bearing by §14.4.1's centrality,
// or normative by the type registry (§5.8).
//
// **An override passes the check, and recording it is the whole mechanism.** A waived
// co-signature that leaves no trace is indistinguishable from an authority that was
// never in force; a gate with no override is a gate people route around, and a gate
// whose overrides are countable is still a gate. So this reports a missing signature
// with no recorded reason, and says nothing about one with a reason — `audit` is what
// enumerates those.
//
// **The authority is the warrant's, not today's.** §10.6.3 makes the authority govern
// admission at the time of the decision, so an adjudication made under `quorum` stays
// exactly as valid when the team shrinks to one. A warrant that records none falls back
// to the corpus's current authority, and the message says that it did — the alternative
// is silence about a claim whose requirement nobody can reconstruct.
func coSignCheck() Check {
	return Check{
		Name:       "co-sign",
		Categories: []string{"co-sign"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies:    someClaimIsAdjudicated,
		Run:        missingCoSignatures,
	}
}

// someClaimIsAdjudicated reports whether the corpus has adjudicated anything.
//
// Requires: nothing.
// Ensures: a reason whenever it declines, naming the missing thing rather than the
// check. Pure.
//
// Both warrant checks share it, because both are about a population that does not
// exist in a corpus nobody has adjudicated in — which is every corpus before its first
// conflict. Reporting either on such a corpus would be reporting the absence of a
// decision nobody had to make.
func someClaimIsAdjudicated(snap *Snapshot) (bool, string) {
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			if claim.Warrant.Adjudicated() || len(claim.Supersedes) > 0 {
				return true, ""
			}
		}
	}
	return false, "no claim carries a gnosis_warrant or supersedes another, so the" +
		" corpus has adjudicated nothing yet"
}

// unwarrantedDecisions reports each adjudication whose reasoning is missing.
//
// Requires: nothing.
// Ensures: one diagnostic per claim, in document order; a claim with a rationale and no
// supersession is silent. Pure.
func unwarrantedDecisions(snap *Snapshot) []finding.Diagnostic {
	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			if d := unwarranted(doc, claim); d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// unwarranted is one claim's warrant problem, or nil.
func unwarranted(doc *Document, claim *Claim) *finding.Diagnostic {
	var why string
	switch {
	case len(claim.Supersedes) > 0 && !claim.Warrant.Adjudicated():
		// "…displaced them" was the first wording, and it disagrees with a count of
		// one exactly as §17.5's verbs did — a pronoun agreeing with a plural noun
		// phrase that may be singular. The grammar detector cannot see it: it scans
		// for a verb next to the count, and a pronoun sits a clause away, which is a
		// parse rather than a scan. So the sentence is written with no pronoun at all.
		why = "it supersedes " + Noun(len(claim.Supersedes), "claim") +
			" and records no gnosis_warrant, so the decision behind the supersession" +
			" was never written down"
	case claim.Warrant.Adjudicated() &&
		strings.TrimSpace(claim.Warrant.Rationale) == "":
		why = "its gnosis_warrant carries no rationale, and §10.6.4 requires one at" +
			" every authority including sole — the reader you are writing for is" +
			" yourself in six months, and that reader has no other record of why"
	default:
		return nil
	}
	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "warrant",
		Path:     doc.Path,
		Message:  "claim " + claim.ID + ": " + why,
		Action:   finding.ActionHuman,
	}
}

// missingCoSignatures reports each escalated adjudication with no second signature and
// no recorded override.
//
// Requires: snap.Authority is the corpus's derived authority; snap.InDegreeCut and the
// vocabulary decide what is escalated.
// Ensures: one diagnostic per claim, in document order. Pure.
func missingCoSignatures(snap *Snapshot) []finding.Diagnostic {
	inDegree, _ := centrality(snap, durabilityByDocument(snap))

	out := make([]finding.Diagnostic, 0)
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		escalated, why := escalation(snap, doc, inDegree[doc.ID])
		if !escalated {
			continue
		}
		for j := range doc.Claims {
			if d := uncosigned(snap, doc, &doc.Claims[j], why); d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// escalation reports whether a document's claims are load-bearing or normative, and
// which of the two it is.
//
// Requires: inDegree is how many documents link here.
// Ensures: a reason whenever it reports true, because the finding has to say what made
// the claim escalated — "a co-signer was required" with no reason is a rule a reader
// cannot check. Pure.
func escalation(snap *Snapshot, doc *Document, inDegree int) (bool, string) {
	if declared, ok := snap.Vocabulary.TypeNamed(doc.Type); ok && declared.Normative {
		return true, "its type " + string(doc.Type) + " is normative"
	}
	if snap.InDegreeCut > 0 && inDegree >= snap.InDegreeCut {
		return true, "it is load-bearing: cited by " + Noun(inDegree, "document") +
			", at or above the declared in-degree cut"
	}
	return false, ""
}

// uncosigned is one escalated claim's co-signature problem, or nil.
//
// A claim carrying no warrant at all is silent here: that is `warrant`'s finding when
// the claim was adjudicated, and nothing at all when it was not. Reporting it would
// ask every normative claim in the corpus for a second signature on a decision nobody
// made.
func uncosigned(snap *Snapshot, doc *Document, claim *Claim, why string) *finding.Diagnostic {
	w := &claim.Warrant
	if !w.Adjudicated() || strings.TrimSpace(w.CoSignedBy) != "" ||
		strings.TrimSpace(w.OverrideReason) != "" {
		return nil
	}
	authority, declared := gnosis.AuthorityOf(w.Authority)
	if !declared {
		authority = snap.Authority
	}
	if !authority.RequiresCoSigner() {
		return nil
	}
	inForce := "the warrant records " + authority.String()
	if !declared {
		inForce = "the warrant records no authority, so the corpus's current " +
			authority.String() + " is assumed"
	}
	return &finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "co-sign",
		Path:     doc.Path,
		Message: "claim " + claim.ID + ": " + why + ", and " + inForce +
			" — it needs a co_signed_by, or an override whose reason is recorded" +
			" (§10.6.4). A waiver that leaves no trace cannot be told from an" +
			" authority that was never in force",
		Action: finding.ActionHuman,
	}
}
