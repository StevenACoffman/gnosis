package lint

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/finding"
)

// residueCategory names a pair no deterministic predicate could separate.
//
// **Its own category, and the distinction is the whole value.** `conflict` reporting
// "these two disagree" and "nobody can tell whether these two disagree" under one name
// would make a reader treat the second as the first — and only the second is judge work
// (§10.3). A category is what lets §17.1's four-state review report which act ran.
const residueCategory = "conflict:unseparated"

// UnseparatedPair is two claims on one subject that the predicates left undecided.
//
// Sorted on construction, so the pair (a, b) and the pair (b, a) are one pair with one
// identity. That is what makes the finding id stable enough for a deferral to match it
// again after a rebuild.
//
// **Exported because a diagnostic cannot carry it.** `finding.Diagnostic` is a severity,
// a category, a path and a message; §13's review queue needs *both claims side by side*
// with their own references, and the first version of the queue scraped the id back out
// of the message — which would break the first time somebody improved the wording. The
// pair is the structured form, and the message is one rendering of it.
type UnseparatedPair struct {
	Subject gnosis.SubjectKey
	First   UnseparatedClaim
	Second  UnseparatedClaim
}

// UnseparatedClaim is one side of a pair.
type UnseparatedClaim struct {
	Path    string
	DocID   gnosis.ID
	ClaimID string
	Anchor  string

	// Parsed says whether this claim's prose yielded a bound. It travels because it is
	// the reason the pair is residue rather than a conflict: the predicate ran and one
	// side gave it nothing to compare.
	Parsed bool
}

// ID is the pair's stable identity, as a deferral records it.
func (p *UnseparatedPair) ID() string { return ResidueID(p.First.Ref(), p.Second.Ref()) }

// Ref addresses the claim as §5.4 requires — by identifier, never by path.
func (c *UnseparatedClaim) Ref() string { return gnosis.ClaimRef(c.DocID, c.ClaimID) }

// ResidueID is the stable identity of one unseparated pair.
//
// Requires: refs are the two claim references, in any order.
// Ensures: the same id for the same pair on every machine and in either order, and a
// different id for any other pair. Pure.
//
// **Content-addressed rather than minted**, which is tier 0's rule applied to a finding.
// A UUID per run would make a recorded deferral unmatchable the next time the check ran —
// the corpus would forget what somebody had decided to live with, which is the one thing
// §10.7.4 says must survive a rebuild.
//
// Truncated to sixteen hex characters, as `identity.Hash` truncates: this identifies a
// pair within one corpus, and the full digest would put sixty-four characters of noise
// into frontmatter a person reads.
func ResidueID(refs ...string) string {
	sorted := make([]string, len(refs))
	copy(sorted, refs)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))
	return hex.EncodeToString(sum[:])[:16]
}

// unseparatedPairs reports the pairs §10.3 routes to a judge.
//
// Requires: snap.Bounds holds the readings the shell parsed; snap.Vocabulary resolves
// subject surfaces.
// Ensures: one diagnostic per pair, sorted, each naming its stable id and why the
// predicate could not decide. Pure.
//
// # What makes a pair residue, and what does not
//
// Two claims are candidates when the corpus itself says they are about one thing — a
// resolved subject key. §10.3 refuses a similarity threshold over an embedding, so
// "claims that look alike" is not available and a declared subject is the only
// non-invented population there is.
//
// A pair is residue when the predicate **ran and could not decide**: at least one side's
// prose did not parse to a bound, so there was nothing to compare. Two parsed bounds are
// the interval predicate's business — it reports them when they are disjoint and stays
// silent when they overlap, and a pair it deliberately passed is not a pair nobody could
// judge.
//
// **A claim with no subject is not residue either**, which is the case that would have
// made this noise: an unresolvable phrase is `subject-unknown`'s finding, and treating
// every unsubjected claim as potentially conflicting with every other would report the
// corpus as one large contradiction.
func unseparatedPairs(snap *Snapshot) []finding.Diagnostic {
	pairs := Unseparated(snap)
	deferred := deferredHere(snap)
	out := make([]finding.Diagnostic, 0, len(pairs))
	for i := range pairs {
		out = append(out, residueFinding(&pairs[i], deferred[pairs[i].ID()]))
	}
	return out
}

// deferredHere maps each deferred conflict's identity to who deferred it.
//
// Requires: snap carries the parsed conflict edges.
// Ensures: only valid deferrals count, which is the safe direction — a half-written
// entry leaves the finding reading as open rather than as decided about. Pure.
func deferredHere(snap *Snapshot) map[string]*gnosis.ConflictEdge {
	out := map[string]*gnosis.ConflictEdge{}
	for i := range snap.Documents {
		edges := snap.Documents[i].Conflicts
		for j := range edges {
			if edges[j].Valid() {
				out[edges[j].Finding] = &edges[j]
			}
		}
	}
	return out
}

// Unseparated enumerates the pairs no implemented predicate could decide.
//
// Requires: snap is the same snapshot the checks run over.
// Ensures: pairs in subject order then reference order, so two runs over one corpus
// produce the same list. Pure.
//
// The one place the population is computed, so the findings, the queue and the
// stale-edge check cannot disagree about what the residue is.
func Unseparated(snap *Snapshot) []UnseparatedPair {
	bySubject := map[gnosis.SubjectKey][]UnseparatedClaim{}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		if episodic(snap, doc) {
			continue
		}
		collectResidueClaims(snap, doc, bySubject)
	}

	keys := make([]gnosis.SubjectKey, 0, len(bySubject))
	for key := range bySubject {
		keys = append(keys, key)
	}
	// Sorted, because map iteration is randomized and a findings list whose order
	// moved between two runs over one corpus is one a reader cannot diff.
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	out := make([]UnseparatedPair, 0)
	for _, key := range keys {
		out = append(out, undecidedPairs(key, bySubject[key])...)
	}
	return out
}

// collectResidueClaims adds one document's subjected claims to the population.
func collectResidueClaims(
	snap *Snapshot, doc *Document, bySubject map[gnosis.SubjectKey][]UnseparatedClaim,
) {
	for j := range doc.Claims {
		claim := &doc.Claims[j]
		key, ok := snap.Vocabulary.ResolvesSubject(claim.Subject)
		if !ok {
			continue
		}
		bound, has := snap.Bounds[claim.ID]
		bySubject[key] = append(bySubject[key], UnseparatedClaim{
			Path: doc.Path, DocID: doc.ID, ClaimID: claim.ID, Anchor: claim.Anchor,
			Parsed: has && bound.Parsed(),
		})
	}
}

// undecidedPairs reports the pairs under one subject that no predicate could separate.
func undecidedPairs(key gnosis.SubjectKey, claims []UnseparatedClaim) []UnseparatedPair {
	sort.Slice(claims, func(i, j int) bool { return claims[i].Ref() < claims[j].Ref() })

	out := make([]UnseparatedPair, 0)
	for i := range claims {
		for j := i + 1; j < len(claims); j++ {
			if claims[i].Parsed && claims[j].Parsed {
				// Both parsed, so the interval predicate decided this pair one way
				// or the other. Reporting it here as well would make one examined
				// pair read as two findings.
				continue
			}
			out = append(out, UnseparatedPair{
				Subject: key, First: claims[i], Second: claims[j],
			})
		}
	}
	return out
}

// residueFinding renders one unseparated pair.
//
// **A warning and never an error**, which is §10.2.2's rule: a derived comparison
// generates findings and never verdicts, and nothing blocks on a conflict. The action is
// a human's, because that is precisely what the finding says — no predicate can settle
// this.
//
// The message names the id, because the id is what a deferral is recorded against: a
// reader told "these two are unseparated" and not told the id cannot act on it without
// re-deriving one.
func residueFinding(pair *UnseparatedPair, edge *gnosis.ConflictEdge) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: residueCategory,
		Path:     pair.First.Path,
		Action:   finding.ActionHuman,
		Message: "no predicate can separate these two claims about " +
			string(pair.Subject) + ": " + pair.First.Ref() + " (" +
			oneLineAnchor(pair.First.Anchor) + ") and " + pair.Second.Ref() + " (" +
			oneLineAnchor(pair.Second.Anchor) + ") — " + whyUndecided(pair) +
			". Conflict " + pair.ID() + remedyFor(edge),
	}
}

// remedyFor says what to do about a pair, which depends on whether anybody already has.
//
// **A deferred conflict is still reported, and saying which set it is in is the point.**
// §17.0: "reviewing the deferred set is a different activity from reviewing the open set,
// and it is the one that tells a team what it has decided to live with" — so `lint` keeps
// the finding and names the decision, while the review queue hides it, because the
// queue's job is what nobody has looked at.
//
// A hand run is what found this missing. The deferral vanished from the queue, `lint`
// went on reporting the conflict in words identical to an undecided one, and the output
// a team would audit its deferrals from could not tell the two apart.
func remedyFor(edge *gnosis.ConflictEdge) string {
	if edge == nil {
		return ": a judge decides this, or `gnosis defer` records living with it"
	}
	return ", deferred by " + edge.By + " on " + edge.At + ": " +
		oneLineAnchor(edge.Reason)
}

// whyUndecided says which side gave the predicate nothing.
//
// Named per side rather than as "one of them", because the remedy differs: a claim whose
// prose does not state a sharp bound is one somebody can rewrite, and knowing which one
// is the difference between an edit and a search.
func whyUndecided(pair *UnseparatedPair) string {
	switch {
	case !pair.First.Parsed && !pair.Second.Parsed:
		return "neither states a bound this corpus's operator patterns can read"
	case !pair.First.Parsed:
		return "the first states no bound the operator patterns can read"
	default:
		return "the second states no bound the operator patterns can read"
	}
}

// oneLineAnchor collapses an anchor so it cannot break the line it sits on.
func oneLineAnchor(s string) string {
	flat := strings.Join(strings.Fields(s), " ")
	const width = 60
	runes := []rune(flat)
	if len(runes) <= width {
		return flat
	}
	return string(runes[:width]) + "…"
}
