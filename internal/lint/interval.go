package lint

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
)

// boundedClaim is one claim that stated a quantity.
type boundedClaim struct {
	Path    string
	ClaimID string
	Anchor  string

	// Bound points into the snapshot rather than copying it. The value grew past the
	// size where copying it per claim is free, and nothing here mutates it.
	Bound *Bound
}

// Bound is a claim's parsed reading, as the predicates compare them.
//
// A value the shell gathers, like Vocabulary and Strength: a check is handed the
// readings, never the parser. That keeps this package's single internal import and keeps
// the operator patterns in data where a corpus can extend them.
type Bound struct {
	// SubjectKey is what the claim is about, resolved. Two claims are comparable only
	// when these match.
	SubjectKey string

	// Dimension is the subject's declared dimension. Two values of different
	// dimensions are a category error, not a conflict.
	Dimension string

	// Op and Value are the reading. Op is empty when the prose stated no quantity,
	// which is a fact §10.2.3 counts and not something to compare.
	Op    string
	Value float64

	// Written is the dimension the claim's own unit belongs to — "400ms" is a
	// duration whatever its subject declares — and is empty when the value carried no
	// unit. That is the commonest case and it must stay silent: "3" is what every
	// dimension looks like when an author omitted the unit.
	//
	// **On the bound rather than read from the vocabulary, because it is a fact about
	// the value.** The declared dimension above comes from the subject; this comes
	// from the prose, and the whole signal is the two disagreeing.
	Written string

	// Pinned reports that this reading came from `gnosis_constraint` rather than from
	// the prose (§10.2.1).
	//
	// **Only the flag travels, not a second parse.** §10.2.1's drift check renders the
	// pinned value and looks for it in the claim's *text* under fold — it does not
	// compare two parses — so the claim's anchor, which the snapshot already carries,
	// is the other half. Carrying a prose reading here as well would be a second
	// representation held for a comparison nobody makes.
	Pinned bool

	// ProseText is the claim's anchor with spelled-out numbers normalised (§7.3),
	// filled **only for a pinned claim** because it is the only one anything compares
	// against text.
	//
	// Normalised by the shell rather than here: §10.2.1's mechanism is "normalize
	// spelled-out numbers in the claim text, render the constraint through the same
	// library, and look for it under fold", and the library is a parser this package
	// does not import (PLAN §0.1). Normalising the whole corpus to check a handful of
	// pins would also put every claim's rewritten text in the snapshot beside the
	// author's own.
	ProseText string

	// Raw and PatternID are what the finding shows, so an adjudicator sees the parse
	// beside the claim rather than only a verdict (§10.2.2).
	Raw       string
	PatternID string
}

// episodic reports whether a document's type makes its claims ineligible for this
// predicate (§5.8.3.1).
//
// **Two reports of different moments cannot contradict.** *"We set the retry budget to 3
// in March"* and *"we set it to 5 in June"* present here as one subject with disjoint
// values, and adjudicating that would be the corpus adjudicating its own history. §10.4's
// supersession then never fires either — not by prohibition, but because it deprecates
// the loser of an *adjudicated* conflict and there is never one to adjudicate.
//
// **Excluded from either side of a pair, not merely from episode-versus-episode.** An
// episode records what happened and a rule states what holds; neither contradicts the
// other, and a finding pairing them would ask a reader to adjudicate a fact against a
// policy.
//
// **Scoped to this predicate and not to evidence divergence**, which is the distinction
// §5.8.3.1 actually argues. Ineligibility is about adjudication between claims. An
// episode resting on two versions of one source still has evidence that moved, and
// suppressing that would use "this is history" to excuse an unsupported claim.
//
// **The flag is read from the document's type rather than carried on the bound.** It is a
// property of the type, and a copy on every claim value is a second place for it to be
// wrong.
func episodic(snap *Snapshot, doc *Document) bool {
	declared, ok := snap.Vocabulary.TypeNamed(doc.Type)
	return ok && declared.Episodic
}

// Parsed reports whether this claim stated a quantity the patterns could read.
func (b *Bound) Parsed() bool { return b.Op != "" }

// interval returns the admissible range this bound describes.
//
// Requires: b.Parsed().
// Ensures: low <= high. An unrecognised operator yields the whole line, which admits
// everything and therefore conflicts with nothing — the safe direction for a reading
// nobody anticipated.
func (b *Bound) interval() (low, high float64) {
	switch b.Op {
	case "<=":
		return math.Inf(-1), b.Value
	case ">=":
		return b.Value, math.Inf(1)
	case "==":
		return b.Value, b.Value
	default:
		return math.Inf(-1), math.Inf(1)
	}
}

// intervalConflicts reports pairs of claims on one subject whose admissible ranges
// cannot both hold.
//
// Requires: snap.Bounds is keyed by claim id.
// Ensures: one diagnostic per conflicting pair, sorted, each showing both parses. Pure.
//
// **Disjoint, not merely different.** `<= 3` and `<= 5` differ and are perfectly
// compatible; `<= 3` and `>= 5` are not. A predicate that fired on difference would
// report every corpus that states a bound twice, which is every well-specified corpus.
//
// **Only where both sides parsed** (§10.2.2), and only within one dimension: comparing a
// count to a duration is a category error the vocabulary already prevents, and reporting
// it as a conflict would blame the claims for the comparison.
//
// **The enumeration predicate of §10.2 is subsumed here and is not built separately.**
// With the operator set as it stands, two claims asserting `==` on one subject with
// different values are two disjoint intervals — so this reports them already. A second
// predicate firing on the same pair would report one problem twice. It separates from
// this one only if a pattern ever yields a set-valued operator, and none does.
func intervalConflicts(snap *Snapshot) []finding.Diagnostic {
	bySubject := map[string][]boundedClaim{}
	for i := range snap.Documents {
		doc := &snap.Documents[i]
		if episodic(snap, doc) {
			continue
		}
		for j := range doc.Claims {
			claim := &doc.Claims[j]
			b, ok := snap.Bounds[claim.ID]
			if !ok || !b.Parsed() {
				continue
			}
			key := b.SubjectKey + "\x00" + b.Dimension
			bySubject[key] = append(bySubject[key], boundedClaim{
				Path: doc.Path, ClaimID: claim.ID, Anchor: claim.Anchor, Bound: b,
			})
		}
	}

	out := make([]finding.Diagnostic, 0)
	for _, claims := range bySubject {
		out = append(out, disjointPairs(claims)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// disjointPairs reports every pair among claims whose intervals cannot both hold.
//
// Requires: every claim parsed and shares one subject and dimension.
// Ensures: each pair reported once, in claim-id order within the pair. Pure.
func disjointPairs(claims []boundedClaim) []finding.Diagnostic {
	sort.Slice(claims, func(i, j int) bool { return claims[i].ClaimID < claims[j].ClaimID })

	out := make([]finding.Diagnostic, 0)
	for i := range claims {
		for j := i + 1; j < len(claims); j++ {
			if overlaps(claims[i].Bound, claims[j].Bound) {
				continue
			}
			out = append(out, conflictFinding(&claims[i], &claims[j]))
		}
	}
	return out
}

// overlaps reports whether two bounds admit any common value.
func overlaps(a, b *Bound) bool {
	aLow, aHigh := a.interval()
	bLow, bHigh := b.interval()
	return math.Max(aLow, bLow) <= math.Min(aHigh, bHigh)
}

// conflictFinding renders one interval conflict, showing both parses.
//
// §10.2.2: "a finding derived from a parse says so, and shows the parse." A false
// conflict that shows its reasoning is dismissible in seconds; one that shows only a
// verdict erodes trust in the whole queue — and every conflict here is a *candidate* for
// a person to adjudicate, never a verdict.
func conflictFinding(a, b *boundedClaim) finding.Diagnostic {
	var s strings.Builder
	s.WriteString("two claims about ")
	s.WriteString(a.Bound.SubjectKey)
	s.WriteString(" cannot both hold: ")
	s.WriteString(describeBound(a))
	s.WriteString("; ")
	s.WriteString(describeBound(b))
	s.WriteString(" — derived from the prose by pattern ")
	s.WriteString(a.Bound.PatternID)
	s.WriteString(" and ")
	s.WriteString(b.Bound.PatternID)
	s.WriteString("; nothing blocks on this, adjudicate it or correct one claim")

	return finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Category: "conflict",
		Path:     a.Path,
		Message:  s.String(),
		Action:   finding.ActionHuman,
	}
}

// describeBound renders one side of a conflict: where it is, what it says, and how that
// was read.
func describeBound(c *boundedClaim) string {
	return c.Path + " " + c.ClaimID + " " + excerpt(c.Anchor) + " {op: " + c.Bound.Op +
		", value: " + strconv.FormatFloat(c.Bound.Value, 'g', -1, 64) + "}"
}
