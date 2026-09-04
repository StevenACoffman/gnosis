package bundle

import (
	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// DocTrust is one document's trust tier and each of its claims' (§14.1).
//
// It sits beside DocFreshness rather than inside it, and the reason is §14.4's:
// trust and freshness are orthogonal axes and neither ordering dominates. A
// human-reviewed claim resting on a source nobody has re-checked is well attested
// and possibly out of date; a machine-confirmed claim verified this morning is the
// reverse. Composing them would produce a single word that answers neither
// question, which is the score §17 refuses.
type DocTrust struct {
	// State is the weakest tier among the document's claims, or unverified for a
	// document declaring none.
	State gnosis.Tier `json:"state"`

	// Why is one sentence naming what produced the tier, populated for every state
	// including the strongest. "Human-reviewed" alone does not say who reviewed it.
	Why string `json:"why"`

	// Claims is each declared claim's own tier, in declaration order, and empty for
	// a document declaring none.
	Claims []ClaimTrust `json:"claims,omitempty"`
}

// ClaimTrust is one claim's trust tier and the actors that produced it.
type ClaimTrust struct {
	ID string `json:"id"`

	// Anchor is the span of the body this claim addresses, carried so a reader can
	// find the sentence without opening the file. Empty for a claim declaring none.
	Anchor string `json:"anchor,omitempty"`

	State gnosis.Tier `json:"state"`

	// By are the actors in this claim's `verified` list, verbatim and in
	// declaration order.
	//
	// **The actors rather than a count**, for the reason ClaimFreshness.Sources is
	// a list: two verifications by one agent and two by two people are the same
	// number and not the same evidence. Verbatim because §14.1.1 makes these a
	// different population from `gnosis.Actor` — a string this parser cannot mint
	// is still a conformant OKF producer, and normalising it here would discard
	// exactly what a reader wanting to know *who* would open the file for.
	By []string `json:"by,omitempty"`
}

// TrustFor reports one document's trust tier, and each of its claims'.
//
// Requires: rel is a bundle-relative document path.
// Ensures: ENOTFOUND when the bundle holds no document at rel; otherwise the tier
// derived from what the document commits. Never an error for a document carrying no
// trust frontmatter, which is unverified and MUST stay consumable (OKF §11).
//
// It reads the bundle rather than the index, although the index carries a
// `verifications` table. The tier is derived and never stored (§14), so the answer
// has to come from the committed tier — and a reader asking how far a claim has been
// confirmed is exactly the reader who must not be told what a cache last thought.
func TrustFor(bundleDir, rel string) (DocTrust, error) {
	doc, err := documentAt(bundleDir, rel)
	if err != nil {
		return DocTrust{}, err
	}
	return describeTrust(doc), nil
}

// describeTrust is the pure half: everything above it is I/O.
//
// Requires: doc came from Load.
// Ensures: the document's tier is the fold over its claims' tiers, computed from the
// same values, so the two grains cannot disagree about what a verification is worth.
// Pure.
func describeTrust(doc *Document) DocTrust {
	claims := claimTrust(doc)
	tiers := make([]gnosis.Tier, 0, len(claims))
	for i := range claims {
		tiers = append(tiers, claims[i].State)
	}
	state := gnosis.FoldTrustDocument(tiers)
	return DocTrust{State: state, Why: whyTrust(state, len(claims)), Claims: claims}
}

// claimTrust is each claim's own tier.
//
// Requires: doc came from Load.
// Ensures: one entry per declared claim, in declaration order, or nil for a document
// declaring none. Pure.
func claimTrust(doc *Document) []ClaimTrust {
	if len(doc.Claims) == 0 {
		return nil
	}
	out := make([]ClaimTrust, 0, len(doc.Claims))
	for i := range doc.Claims {
		c := &doc.Claims[i]
		by := make([]string, 0, len(c.Verified))
		for _, v := range c.Verified {
			by = append(by, v.By)
		}
		out = append(out, ClaimTrust{
			ID:     c.ID,
			Anchor: c.Anchor,
			State:  gnosis.FoldTrust(by),
			By:     by,
		})
	}
	return out
}

// whyTrust is the sentence for a document's tier.
//
// The unverified case has two causes a reader has to tell apart — a page declaring
// no claims at all, and claims nobody has verified — because the remedies are
// different and only one of them is work anybody would do.
func whyTrust(state gnosis.Tier, claims int) string {
	switch state {
	case gnosis.TierHumanReviewed:
		return "every claim on it carries a verification by a person"
	case gnosis.TierMachineConfirmed:
		return "its claims are verified, and none of the verifications is a person's"
	case gnosis.TierUnverified:
		if claims == 0 {
			return "it declares no claims, so there is no verification to fold"
		}
		return "at least one of its claims carries no verification at all"
	default:
		// A tier added to the vocabulary and not handled here produces no sentence
		// rather than a wrong one.
		return ""
	}
}
