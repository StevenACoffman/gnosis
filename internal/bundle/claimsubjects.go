package bundle

import (
	"strings"

	"github.com/StevenACoffman/gnosis/internal/constraint"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/ontology"
)

// ClaimSubject is one claim's resolved subject and the reading of its prose.
//
// The two travel together because §10.2.1 makes them one row: a value with no subject
// cannot be compared to anything, and a subject with no value is the state §10.2.3 has
// to be able to see.
type ClaimSubject struct {
	SubjectKey string
	Dimension  string

	// Prose is the reading the parser found in the claim's text, and InText reports
	// whether there was one. Comma-ok rather than a zero Constraint, which would
	// assert a bound of zero.
	Prose  constraint.Constraint
	InText bool

	// Pin is a reading stated in `gnosis_constraint`, and Pinned reports whether there
	// was one (§10.2.1). It takes precedence over the prose, and is the one case where
	// two representations of one claim can disagree — which is what `constraint-drift`
	// reports and why `claim_subjects.derived` has two values rather than one.
	Pin    constraint.Constraint
	Pinned bool
}

// Effective is the reading this claim is compared under, and whether there is one.
//
// **A method rather than a third pair of fields**, which the first version stored. A
// stored `Constraint` alongside `Prose` holds the same value twice whenever a claim is
// unpinned, and two homes for one fact is two places for it to be wrong — the argument
// this codebase applied to the episodic flag and to `entity_aliases`. It also pushed the
// struct past the size where copying it per map entry is worth a linter's attention.
//
// §10.2.1: "when a pin is present it takes precedence over the derived value."
func (c *ClaimSubject) Effective() (constraint.Constraint, bool) {
	if c.Pinned {
		return c.Pin, true
	}
	return c.Prose, c.InText
}

// ClaimConstraints reads every claim's subject and constraint, once.
//
// Requires: docs came from Load; o is the loaded vocabulary or nil; patterns are the
// operator patterns or nil.
// Ensures: keyed by claim id; one entry per claim whose subject resolves to a declared
// key. Pure.
//
// **One fold, two callers.** The index writer projects these into `claim_subjects` and
// the lint snapshot carries them to the conflict predicates. Parsing in both places would
// be two answers to "what does this claim bound", able to drift apart in exactly the way
// §9.4 refuses for the diff a gate approved.
func ClaimConstraints(
	docs []Document, o *ontology.Ontology, patterns []constraint.Pattern,
) map[string]*ClaimSubject {
	out := map[string]*ClaimSubject{}
	if o == nil {
		return out
	}
	for i := range docs {
		d := &docs[i]
		if d.ID == "" {
			continue
		}
		for j := range d.Claims {
			claim := &d.Claims[j]
			cs, ok := claimSubjectOf(claim, o, patterns)
			if ok {
				out[claim.ID] = &cs
			}
		}
	}
	return out
}

// claimSubjectOf resolves one claim's subject and parses its prose.
func claimSubjectOf(
	claim *DocClaim, o *ontology.Ontology, patterns []constraint.Pattern,
) (ClaimSubject, bool) {
	surface := strings.TrimSpace(claim.Subject)
	if surface == "" || strings.TrimSpace(claim.Anchor) == "" {
		return ClaimSubject{}, false
	}
	key, ok := o.ResolveSubject(gnosis.Surface(surface))
	if !ok {
		// An unresolvable subject is `subject-unknown`'s finding. A row keyed on it
		// would be a foreign key into a vocabulary entry that does not exist.
		return ClaimSubject{}, false
	}
	out := ClaimSubject{
		SubjectKey: key.String(),
		Dimension:  string(dimensionOf(o, key)),
	}
	out.Prose, out.InText = constraint.Parse(claim.Anchor, patterns)

	// The prose reading is kept beside the pin rather than replaced by it: the drift
	// check compares the two, and a claim that dropped one has nothing to check.
	out.Pin, out.Pinned = claim.Pin, claim.Pinned
	return out, true
}

// ClaimSubjectRows projects each claim's subject and the value its prose parses to.
//
// Requires: docs came from Load; o is the loaded vocabulary or nil; patterns are the
// operator patterns or nil.
// Ensures: one row per claim whose subject resolves to a declared key, in document
// order. A claim whose subject does not resolve gets no row — `subject-unknown` reports
// that, and a row keyed on an undeclared subject would be a foreign key into a
// vocabulary entry that does not exist. Pure.
//
// **A row is written even when the prose parses to nothing**, with a NULL value. That is
// what makes §10.2.3's coverage loop possible: it has to distinguish *this claim states
// no quantity* from *this claim states one the patterns miss*, and a missing row says
// neither. The first is a fact about the corpus and the second is a backlog item.
//
// **Both halves together, per §10.2.1.** The declared key and the derived value are
// written by this one fold, so `derived` and `pattern_id` never describe a row that is
// half-parsed.
func ClaimSubjectRows(
	docs []Document, o *ontology.Ontology, patterns []constraint.Pattern,
) []index.ClaimSubjectRow {
	parsed := ClaimConstraints(docs, o, patterns)
	out := make([]index.ClaimSubjectRow, 0, len(parsed))
	for i := range docs {
		d := &docs[i]
		for j := range d.Claims {
			cs, ok := parsed[d.Claims[j].ID]
			if !ok {
				continue
			}
			out = append(out, subjectRow(d.Claims[j].ID, cs))
		}
	}
	return out
}

// subjectRow renders one fold entry as the row the index holds.
func subjectRow(claimID string, cs *ClaimSubject) index.ClaimSubjectRow {
	row := index.ClaimSubjectRow{
		ClaimID:    claimID,
		SubjectKey: cs.SubjectKey,
		Dimension:  cs.Dimension,
		Derived:    !cs.Pinned,
	}
	if effective, ok := cs.Effective(); ok {
		value := effective.Value
		row.Op, row.Value = string(effective.Op), &value
		row.ValueRaw, row.PatternID = effective.Raw, effective.PatternID
	}
	return row
}

// dimensionOf reports a declared subject's dimension, which tells a comparison whether
// two values are commensurable at all.
func dimensionOf(o *ontology.Ontology, key gnosis.SubjectKey) ontology.Dimension {
	for i := range o.Subjects {
		if o.Subjects[i].Key == key {
			return o.Subjects[i].Dimension
		}
	}
	return ""
}
