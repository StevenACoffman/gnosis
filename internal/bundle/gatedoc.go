package bundle

import (
	"strconv"
	"strings"

	"github.com/StevenACoffman/gnosis/internal/constraint"
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/okf"
)

// claimsKey is the frontmatter list of a document's claims and their addresses
// (§5.5.1). Each entry carries an id, an anchor, and the evidence offered for it.
const claimsKey = "gnosis_claims"

// subjectKey names what a claim is about, per claim rather than per document.
//
// §5.5.1 puts it here rather than at document level, and refused an inherited
// default: editing one would silently re-subject every claim that did not override,
// which is the failure the vocabulary layer exists to catch arriving through a
// convenience.
const subjectKey = "subject"

// verifiedKey is OKF §5.2's verification list, read per claim (§5.5).
const verifiedKey = "verified"

// leadKey is §17.4's conclusion-first summary, per claim.
const leadKey = "lead"

// constraintKey is §5.4's optional pin: a mapping stating the reading directly, for the
// case where a precise value exists but not in prose the parser can reach — a number in
// a table, a code fence, or a figure caption (§10.2.1).
const constraintKey = "gnosis_constraint"

// warrantKey is §10.6.4's record of a human adjudication, per claim.
//
// Per claim rather than per document, for the reason `subject` and `lead` are: a
// decision is about an assertion, and a page-level warrant would assert that somebody
// adjudicated every claim on it when they adjudicated one.
const warrantKey = "gnosis_warrant"

// supersedesKey is §10.4's edge from the winner of an adjudicated conflict to the claim
// it replaced. Supersession, never deletion: the loser keeps its links and its history.
const supersedesKey = "gnosis_supersedes"

// challengesKey is §10.7.4's list of reader-filed contests, per document.
//
// Per document rather than per claim, and §10.7.4 writes it that way: a challenge
// arrives as a diff on the document it contests, which is where a reviewer is already
// looking, and a reader contesting a page has not always identified which claim they
// mean — which is part of what the challenge is asking somebody to work out.
const challengesKey = "gnosis_challenges"

// limitationsKey is what a concept declares it does not cover (§17.2), per document.
//
// Per document rather than per claim, unlike `subject` and `lead`: §17.2's scope is the
// concept's, and a claim does not have limits of its own — the page does.
const limitationsKey = "gnosis_limitations"

// claimsOf reads a document's claims out of frontmatter.
//
// §5.5.1 requires a claim's identity and address to be recoverable from the
// document alone, which is why this reads the document rather than the index: the
// index is a derived cache, and a gate that consulted it would be gating on
// something rebuildable rather than on what is committed.
//
// A document with no gnosis_claims yields none, and the evidence signal then
// passes with "no enforced claims to check". That is the correct reading of a
// document that asserts nothing enforceable — Phase 2 documents are written by
// hand and most will be in that state — and the detail string is what keeps it
// from being mistaken for a checked pass.
func claimsOf(doc *okf.Document) []gate.Claim {
	raw, ok := doc.Fields[claimsKey].([]any)
	if !ok {
		return nil
	}
	out := make([]gate.Claim, 0, len(raw))
	for i, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, gate.Claim{
			ID: firstString(m, "id", "anchor", strconv.Itoa(i)),
			// Enforced defaults to true. A claim that says nothing about whether
			// its evidence is gated is gated: the opposite default would let an
			// omitted key exempt a claim from the corpus's central invariant,
			// which is the same fail-open mistake `DryRun bool` makes.
			Enforced:     boolOr(m, "enforced", true),
			Text:         stringOr(m, "anchor"),
			Lead:         stringOr(m, leadKey),
			Quotes:       stringsOf(m, evidenceKey),
			ArchivePaths: stringsOf(m, "archive_paths"),
		})
	}
	return out
}

// docClaims reads a document's claims down to what a check needs.
//
// Separate from claimsOf, which builds the gate's richer shape from the same
// frontmatter. They read one format and answer different questions: the gate needs
// quotations and enforcement to judge evidence, and a check needs only which claim
// named which file. Sharing a type would give a check the fields to start judging.
func docClaims(doc *okf.Document) []DocClaim {
	raw, ok := doc.Fields[claimsKey].([]any)
	if !ok {
		return nil
	}
	out := make([]DocClaim, 0, len(raw))
	for i, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		claim := DocClaim{
			ID:           firstString(m, "id", "anchor", strconv.Itoa(i)),
			Anchor:       stringOr(m, "anchor"),
			Subject:      stringOr(m, subjectKey),
			Lead:         stringOr(m, leadKey),
			Quotes:       stringsOf(m, evidenceKey),
			Verified:     verifiedOf(m),
			Status:       stringOr(m, statusKey),
			ArchivePaths: stringsOf(m, "archive_paths"),
		}
		claim.Pin, claim.Pinned = pinOf(m)
		claim.Warrant = warrantOf(m)
		claim.Supersedes = stringsOf(m, supersedesKey)
		out = append(out, claim)
	}
	return out
}

// verifiedOf reads a claim entry's OKF §5.2 verification list.
//
// Requires: m is a gnosis_claims entry.
// Ensures: one Verification per well-formed event, in declaration order; an entry
// missing either field is skipped rather than half-recorded. OKF §11 requires a bare
// mapping be treated as a one-element list, which is what the string case does.
func verifiedOf(m map[string]any) []Verification {
	switch v := m[verifiedKey].(type) {
	case map[string]any:
		return oneVerification(v)
	case []any:
		out := make([]Verification, 0, len(v))
		for _, entry := range v {
			switch e := entry.(type) {
			case map[string]any:
				out = append(out, oneVerification(e)...)
			case string:
				// A bare actor with no time. Recorded, because OKF §11 says tolerate
				// it and the actor is the half the trust fold reads (§14.1).
				out = append(out, Verification{By: e})
			}
		}
		return out
	default:
		return nil
	}
}

// oneVerification reads a single event mapping, or nothing.
func oneVerification(m map[string]any) []Verification {
	by := stringOr(m, "by")
	if by == "" {
		return nil
	}
	return []Verification{{By: by, At: stringOr(m, "at")}}
}

// sourcesOf reads a document's OKF sources list.
//
// A source is a scope descriptor when it says so. OKF §5.1 permits a resource a
// consumer cannot dereference, and declaring that is what distinguishes an honest
// unfollowable source from a URI that merely happens to be missing.
func sourcesOf(doc *okf.Document) []gate.Source {
	raw, ok := doc.Fields[sourcesKey].([]any)
	if !ok {
		return nil
	}
	out := make([]gate.Source, 0, len(raw))
	for _, entry := range raw {
		switch v := entry.(type) {
		case string:
			// The shorthand: a bare string is a resource.
			out = append(out, gate.Source{Resource: v})
		case map[string]any:
			out = append(out, gate.Source{
				Resource: stringOr(v, "resource"),
				Scope:    boolOr(v, "scope", false),
			})
		}
	}
	return out
}

// stringOr reads a string field, or "" when it is absent or another shape.
func stringOr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// firstString returns the first of keys present as a non-empty string, or fallback.
func firstString(m map[string]any, keys ...string) string {
	fallback := keys[len(keys)-1]
	for _, k := range keys[:len(keys)-1] {
		if s := stringOr(m, k); s != "" {
			return s
		}
	}
	return fallback
}

// boolOr reads a boolean field, or def when it is absent or another shape.
//
// A malformed value takes the default rather than reading as false, because the
// default here is the conservative direction and a mistyped `enforced: yes` must
// not silently exempt a claim.
func boolOr(m map[string]any, key string, def bool) bool {
	b, ok := m[key].(bool)
	if !ok {
		return def
	}
	return b
}

// stringsOf reads a list of strings, tolerating a single string as a list of one.
func stringsOf(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// pinOf reads a claim's `gnosis_constraint` mapping.
//
// Requires: m is a gnosis_claims entry.
// Ensures: comma-ok. A claim with no pin yields false and **never a zero Constraint**,
// which would assert a bound of zero — the rule ClaimSubject.Parsed already keeps one
// layer up, and the reason `op` is checked rather than the value.
//
// **A mapping present but unreadable yields false and is not an error here**, which is
// the one place this differs from the loaders in `standards/`. `Load` reads a whole
// corpus for `lint`, and OKF §11 requires unknown and malformed frontmatter to be
// tolerated rather than to fail the read — a corpus that would not open because one
// claim's pin is mistyped is a corpus nobody can lint back into shape. What must not
// happen is the pin *silently becoming a parsed value*: an unreadable pin leaves the
// claim derived, so `constraint-drift` has nothing to compare and says nothing, rather
// than comparing prose against a bound of zero.
func pinOf(m map[string]any) (constraint.Constraint, bool) {
	raw, ok := m[constraintKey].(map[string]any)
	if !ok {
		return constraint.Constraint{}, false
	}
	op := constraint.OpKind(strings.TrimSpace(stringOr(raw, "op")))
	if !op.Valid() {
		return constraint.Constraint{}, false
	}
	value, ok := floatOf(raw["value"])
	if !ok {
		return constraint.Constraint{}, false
	}
	return constraint.Constraint{Op: op, Value: value, Raw: stringOr(raw, "raw")}, true
}

// floatOf reads a YAML scalar as a number.
//
// **The decoder yields `uint64` for `value: 5`**, not `int`, and float64 only for a
// value written with a decimal point. The first version of this switch covered
// int/int64/float64 on the reasoning that "a pin stating an integer bound is the commoner
// case" — the reasoning was right and the type list was wrong, so every whole-numbered
// pin was silently dropped and the claim fell back to its prose. It reads as a pin that
// did not take effect, which is indistinguishable from a pin nobody wrote.
//
// A negative bound decodes as `int64`, so both signs are listed rather than assumed.
func floatOf(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case uint:
		return float64(n), true
	default:
		return 0, false
	}
}

// resourcesOf reads a document's declared OKF `sources` list down to the resources.
//
// Requires: doc is a parsed concept.
// Ensures: one entry per declared source in declaration order, dropping the empty
// ones; nil for a document declaring none. Pure.
//
// Separate from sourcesOf, which builds the gate's `gate.Source` — that shape carries
// `scope`, which the gate weighs and §14.4 has no use for. Sharing it would hand a
// derived signal a field the provenance gate is the only judge of.
func resourcesOf(doc *okf.Document) []string {
	raw, ok := doc.Fields[sourcesKey].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		var res string
		switch v := entry.(type) {
		case string:
			res = v
		case map[string]any:
			res = stringOr(v, "resource")
		}
		if res != "" {
			out = append(out, res)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// warrantOf reads a claim's `gnosis_warrant` mapping (§10.6.4).
//
// Requires: m is a gnosis_claims entry.
// Ensures: the zero Warrant for a claim carrying none, which Adjudicated reports as
// false. Every field is read as declared and none is normalised: what a reader has to
// be able to see is what the author wrote. Pure.
//
// **The override is a nested mapping and its reason is lifted out.** §10.6.4 writes it
// as `override: {reason: …}` because a waiver may later want more than a reason, and
// flattening it here would make adding one a frontmatter migration. The struct holds
// the reason because that is the whole of the mechanism today: a waived co-signature
// that leaves no trace is indistinguishable from an authority that was never in force.
func warrantOf(m map[string]any) gnosis.Warrant {
	raw, ok := m[warrantKey].(map[string]any)
	if !ok {
		return gnosis.Warrant{}
	}
	w := gnosis.Warrant{
		By:        stringOr(raw, "by"),
		At:        stringOr(raw, "at"),
		Authority: stringOr(raw, "tier"),
		Review:    stringOr(raw, "review"),
		Rationale: stringOr(raw, "rationale"),
		Reverses:  stringOr(raw, "reverses"),
	}
	w.CoSignedBy = stringOr(raw, "co_signed_by")
	if override, hasOverride := raw["override"].(map[string]any); hasOverride {
		w.OverrideReason = stringOr(override, "reason")
	}
	return w
}

// challengesOf reads a document's `gnosis_challenges` list (§10.7.4).
//
// Requires: doc is a parsed concept.
// Ensures: one Challenge per well-formed entry in declaration order; nil for a document
// declaring none. An entry whose class is not one of §10.7.1's six keeps the raw string,
// because dropping it would make a malformed challenge indistinguishable from no
// challenge — and the one thing this list must not do is lose a reader's objection.
// Pure.
func challengesOf(doc *okf.Document) []gnosis.Challenge {
	raw, ok := doc.Fields[challengesKey].([]any)
	if !ok {
		return nil
	}
	out := make([]gnosis.Challenge, 0, len(raw))
	for _, entry := range raw {
		m, isMap := entry.(map[string]any)
		if !isMap {
			continue
		}
		c := gnosis.Challenge{
			Class:     gnosis.ChallengeClass(stringOr(m, "class")),
			By:        stringOr(m, "by"),
			At:        stringOr(m, "at"),
			Rationale: stringOr(m, "rationale"),
			State:     gnosis.ChallengeState(stringOr(m, "state")),
		}
		if id, err := gnosis.ParseID(stringOr(m, "id")); err == nil {
			c.ID = id
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
