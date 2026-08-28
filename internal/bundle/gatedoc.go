package bundle

import (
	"strconv"

	"github.com/StevenACoffman/gnosis/internal/gate"
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
		out = append(out, DocClaim{
			ID:           firstString(m, "id", "anchor", strconv.Itoa(i)),
			Anchor:       stringOr(m, "anchor"),
			Subject:      stringOr(m, subjectKey),
			Lead:         stringOr(m, leadKey),
			Quotes:       stringsOf(m, evidenceKey),
			Verified:     verifiedOf(m),
			ArchivePaths: stringsOf(m, "archive_paths"),
		})
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
