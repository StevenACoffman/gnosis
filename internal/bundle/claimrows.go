package bundle

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/skillet/identity"
	"github.com/StevenACoffman/skillet/textnorm"
)

// ClaimRows projects a corpus's declared claims into their index addresses.
//
// Requires: docs came from Load.
// Ensures: one row per claim carrying an anchor, in document order. Claims with no
// anchor are skipped — an address that was never written is not an address that broke,
// and `conformance` is what reports the absence. Pure.
//
// **`lead` is carried and `title` and `description` are not.** §5.5.3 splits the row in
// two — an address recoverable from the document, and a summary extraction supplies — and
// `lead` moved across that line when the relay began asking for one. It is read from
// frontmatter like every other declared field; a fold that *derived* it would be
// inventing the thing §17.4's check exists to examine.
func ClaimRows(docs []Document) []index.ClaimRow {
	out := make([]index.ClaimRow, 0)
	for i := range docs {
		d := &docs[i]
		if d.ID == "" {
			// A document with no identity cannot own a claim: the foreign key has
			// nothing to point at. `identity` reports the document itself.
			continue
		}
		for j := range d.Claims {
			claim := &d.Claims[j]
			anchor := strings.TrimSpace(claim.Anchor)
			if anchor == "" {
				continue
			}
			out = append(out, index.ClaimRow{
				ID:         claim.ID,
				DocumentID: d.ID,
				AnchorHash: identity.Hash(textnorm.Fold(anchor)),
				Pos:        anchorPos(d.Body, anchor),
				Type:       d.Type.String(),
				Lead:       claim.Lead,
			})
		}
	}
	return out
}

// anchorPos locates an anchor in a body, or reports that it could not.
//
// Requires: nothing.
// Ensures: nil unless the anchor appears in the body **exactly**, and a byte offset
// into the raw body when it does. Pure.
//
// **Exact rather than folded, and the difference is the whole contract.** §5.5.2 makes
// `pos` an offset into the document body, and `textnorm.Fold` collapses whitespace runs
// and typographic characters — so a fold-space offset is a position in a string that
// does not exist on disk. Writing one into a raw-space column would be silently wrong,
// which is worse than absent: a reader sent to the wrong byte has no way to tell.
//
// So an anchor that resolves under `claim-anchor`'s fold but differs from the body by a
// reflowed line still has no honest offset, and NULL is exactly what §5.5.2 defines for
// that state. Mapping fold-space back to raw-space is possible and would be a second
// answer to "where is this claim" — one that could disagree with the check's.
func anchorPos(body, anchor string) *int {
	at := strings.Index(body, anchor)
	if at < 0 {
		return nil
	}
	return &at
}

// VerificationRows projects a corpus's declared verification events into index rows.
//
// Requires: docs came from Load.
// Ensures: one row per event on a claim that has an address, in document order. Events
// on a claim with no anchor are skipped, because that claim has no row to point at —
// the same rule ClaimRows applies and for the same foreign key. Pure.
//
// An event with an actor and no timestamp is kept. OKF §11 says tolerate it, and the
// actor is the half §14.1's trust fold reads: dropping the event would lower a concept's
// tier because somebody omitted a date.
func VerificationRows(docs []Document) []index.VerificationRow {
	out := make([]index.VerificationRow, 0)
	for i := range docs {
		d := &docs[i]
		if d.ID == "" {
			continue
		}
		for j := range d.Claims {
			claim := &d.Claims[j]
			if strings.TrimSpace(claim.Anchor) == "" {
				continue
			}
			for _, v := range claim.Verified {
				out = append(out, index.VerificationRow{
					ClaimID: claim.ID, By: v.By, At: v.At,
				})
			}
		}
	}
	return out
}
