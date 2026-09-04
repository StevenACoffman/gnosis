package gnosis

import "strings"

// claimRefSep separates a document identifier from a claim identifier in a reference.
//
// `#` rather than a character either half could contain. A UUIDv7 is hex and dashes and
// a claim identifier comes from frontmatter, so the two cannot be told apart by
// position — and a separator that could appear inside either half would make the parse
// ambiguous exactly where a reference is hardest to check by eye.
const claimRefSep = "#"

// ClaimRef names one claim across the whole corpus, by identifier.
//
// Requires: id is the document's `gnosis_id`; claimID is the identifier the document
// declares for the claim.
// Ensures: a reference ParseClaimRef reads back. Pure.
//
// # Why an identifier and not a path
//
// §5.4 states the rule for the frontmatter families that carry these — "`gnosis_supersedes`
// and `gnosis_conflicts` name **identifiers, never paths**… an edge that survives
// reorganization is the point" — and §5.1.1 is what makes it work: a document's slug
// changes when its title does, and its identifier never does.
//
// **This shipped as a path for one day and was wrong.** `gnosis supersede` wrote
// `c/<uuid>-<slug>.md#<claim>`, which reads well and breaks the moment anybody retitles
// the losing concept: the edge would then name a file that does not exist, in the one
// direction the corpus cannot repair, because the claim it pointed at is exactly the one
// nobody is looking at any more.
//
// # Why a path is still what a reader sees
//
// §5.6 already settles that: the canonical form is authoritative and presented forms
// resolve *to* it, never the reverse. So an identifier is what is stored and compared,
// and a command that shows a reference to a person renders the path beside it — which
// is a view computed from the index rather than a second address.
//
// It is in the domain because it has three consumers — the supersession edge, the
// critic's sample, and the coverage ledger — and a corpus's own address format spelled
// three times is one for the three to disagree about.
func ClaimRef(id ID, claimID string) string {
	return id.String() + claimRefSep + claimID
}

// ParseClaimRef splits a reference back into a document identifier and a claim
// identifier.
//
// Requires: nothing.
// Ensures: (id, claim, true) for a well-formed reference whose left half is a valid
// identifier and whose right half is non-empty; ("", "", false) otherwise. Pure.
//
// **A path-form reference is refused rather than repaired.** The form written before
// this corrected itself — `c/<uuid>-<slug>.md#<claim>` — has an identifier inside it,
// and extracting one would be guessing that the substring means what it looks like. A
// reference this cannot read is reported, which is what `warrant`'s check and
// `audit --reversed` both want: an edge nobody can follow is worth seeing, and an edge
// silently reinterpreted is not.
//
// It cuts at the **first** separator, and the switch from the last one came with the
// switch from paths. A document identifier is a UUID and cannot contain `#`, so the
// first one always ends it and everything after is the claim — which may then contain
// one. Cutting at the last separator was right while the left half was a path that
// might, and wrong the moment the left half became a fixed shape: it would refuse
// `<id>#claim#two` by handing `ParseID` a string with a `#` in it. The test that
// articulated the round-trip property is what found the stale rule.
func ParseClaimRef(ref string) (id ID, claimID string, ok bool) {
	at := strings.Index(ref, claimRefSep)
	if at <= 0 || at == len(ref)-1 {
		return "", "", false
	}
	parsed, err := ParseID(ref[:at])
	if err != nil {
		return "", "", false
	}
	return parsed, ref[at+1:], true
}
