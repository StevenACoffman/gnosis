package bundle

import (
	"strconv"

	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/gnosis/internal/okf"
)

// claimsKey is the frontmatter list of a document's claims and their addresses
// (§5.5.1). Each entry carries an id, an anchor, and the evidence offered for it.
const claimsKey = "gnosis_claims"

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
			ArchivePaths: stringsOf(m, "archive_paths"),
		})
	}
	return out
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
