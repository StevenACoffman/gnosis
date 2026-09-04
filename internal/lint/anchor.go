package lint

import (
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/textnorm"
)

// claimAnchorCheck reports anchors that no longer address one passage.
//
// §5.5.1 makes a claim's address part of the document: an assigned id and a
// fold-normalised anchor in frontmatter, with `pos` demoted to a cached
// convenience. The point of putting it there rather than in the index is that a
// rebuild can recover it — and an address only recovers a claim if it still appears
// in the body, and appears **once**.
//
// Two findings, and they are different failures with different causes:
//
//   - `anchor-absent`: the anchor is not in the body. The prose was edited or the
//     anchor was never in it, and either way the claim now addresses nothing.
//   - `anchor-collision`: two claims in one document fold to the same anchor. Both
//     resolve to the same passage, so a reader adjudicating one cannot tell which
//     claim they are looking at.
//
// The collision case is the one that arrives without anybody making a mistake. Claim
// ids are UUIDv7 so they never collide, but two people adding claims to one document
// can independently anchor different ids to the same text, and **git merges that
// cleanly** — the same shape §4.6.1 gives for duplicate documents, one level down.
// Nothing else in the corpus would report it.
//
// It reports and never repairs, per §12: `pos` goes NULL and the anchor is left
// alone. A missing anchor has three quite different causes — the prose was rewritten,
// the anchor was mistyped, or the claim was fabricated — and they call for opposite
// responses. A tool that guessed would pick the wrong one silently.
//
// **The absent case cannot yet tell a fabricated anchor from a drifted source**, and
// that is a stated limitation rather than an oversight: distinguishing them needs the
// two-signal cross filed as `verify.Provenance`'s pairing of a whole-file hash with a
// semantic marker, which is Phase 3 work. Until then the finding says what it knows.
func claimAnchorCheck() Check {
	return Check{
		Name:       "claim-anchor",
		Categories: []string{"anchor-absent", "anchor-collision"},
		Actions:    []finding.Action{finding.ActionHuman},
		Applies: func(snap *Snapshot) (bool, string) {
			// Derived applicability, per §12. Most Phase 2 documents are written by
			// hand and declare no claims; reporting nothing found in a corpus with no
			// anchors is different from reporting there was nothing to look for.
			for i := range snap.Documents {
				claims := snap.Documents[i].Claims
				for j := range claims {
					if strings.TrimSpace(claims[j].Anchor) != "" {
						return true, ""
					}
				}
			}
			return false, "no document declares a claim with an anchor yet"
		},
		Run: func(snap *Snapshot) []finding.Diagnostic {
			out := make([]finding.Diagnostic, 0)
			for i := range snap.Documents {
				doc := &snap.Documents[i]
				out = append(out, absentAnchors(doc)...)
				out = append(out, collidingAnchors(doc)...)
			}
			return out
		},
	}
}

// absentAnchors reports each claim whose anchor is not in the body.
//
// Comparison is under textnorm.Fold rather than exact: §5.5.1 specifies a
// fold-normalised anchor, so a paragraph reflowed by a formatter must not read as a
// lost address. Case is **preserved** — this is textnorm.Fold and not
// gnosis.Surface.Fold — because an anchor locates a quotation and case carries
// meaning in one. The duplication signal makes the opposite choice for titles, and
// the gate's own self-test is what found that the two are different questions.
func absentAnchors(doc *Document) []finding.Diagnostic {
	if len(doc.Claims) == 0 {
		return nil
	}
	body := textnorm.Fold(doc.Body)

	var out []finding.Diagnostic
	for i := range doc.Claims {
		claim := &doc.Claims[i]
		anchor := strings.TrimSpace(claim.Anchor)
		if anchor == "" {
			// A claim with no anchor at all is a different finding from one whose
			// anchor moved, and it belongs to conformance rather than here: this
			// check is about addresses that stopped resolving.
			continue
		}
		if strings.Contains(body, textnorm.Fold(anchor)) {
			continue
		}
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "anchor-absent",
			Path:     doc.Path,
			Message: "claim " + claim.ID + " anchors to text that is not in the body" +
				"; the claim addresses nothing, and pos cannot be recovered" +
				" — it is not yet possible to say whether the prose was edited or" +
				" the anchor was never in it",
			Action: finding.ActionHuman,
		})
	}
	return out
}

// collidingAnchors reports anchors shared by more than one claim in one document.
//
// Within a document only. Two documents quoting the same sentence is ordinary and
// says nothing about either; two claims in *one* document resolving to one passage is
// the merge artefact §4.6.1 describes, and the only thing that can tell them apart is
// the scope of the comparison.
//
// One finding per colliding group rather than per claim: three claims on one passage
// is one problem, and three findings would make the report about the claims instead
// of about the passage they are fighting over.
func collidingAnchors(doc *Document) []finding.Diagnostic {
	if len(doc.Claims) < 2 {
		return nil
	}
	byAnchor := map[string][]string{}
	for i := range doc.Claims {
		claim := &doc.Claims[i]
		if anchor := strings.TrimSpace(claim.Anchor); anchor != "" {
			folded := textnorm.Fold(anchor)
			byAnchor[folded] = append(byAnchor[folded], claim.ID)
		}
	}

	var out []finding.Diagnostic
	for folded, ids := range byAnchor {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		out = append(out, finding.Diagnostic{
			Severity: finding.SeverityError,
			Category: "anchor-collision",
			Path:     doc.Path,
			Message: strconv.Itoa(len(ids)) + " claims anchor to one passage (" +
				strings.Join(ids, ", ") + "): " + excerpt(folded) +
				" — a reader adjudicating one cannot tell which claim it is;" +
				" merge them, or re-anchor so each addresses its own sentence",
			Action: finding.ActionHuman,
		})
	}
	// Sorted so a document with two collisions reports them in a stable order; the
	// grouping came out of a map.
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// excerpt renders an anchor in a message, bounded so one long anchor cannot make a
// report unreadable.
func excerpt(s string) string {
	const width = 48
	if len(s) > width {
		s = s[:width] + "…"
	}
	return strconv.Quote(s)
}
