package gnosis

import (
	"strings"

	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/textnorm"
)

// TypeKey names a kind of concept. OKF §4.1 requires one on every document and
// deliberately does not register them centrally, so the set is the corpus's own
// and lives in ontology.toml.
//
// A type is a behavioural distinction, not a semantic label: it drives whether
// limitations are required, whether a missing subject is flagged, and which
// template applies. Two candidate types with identical behaviour are one type
// with two aliases.
type TypeKey string

// SubjectKey names what a claim is about, so that two claims can be known to
// concern the same thing. Keys are dotted and lowercase by convention
// (`retry.max_attempts`); the convention is not enforced, because a corpus that
// wants a different shape should not have to fight the tool for it.
type SubjectKey string

// Surface is a phrase as an author wrote it, before resolution to a key. It is
// retained alongside the key it resolves to: the surface is what a reader sees,
// the key is what comparison uses.
type Surface string

// ParseTypeKey converts s into a TypeKey.
//
// Requires: nothing.
// Ensures: returns EINVALID for an empty or whitespace-only value. No other
// constraint is imposed, because OKF §4.1 leaves type values to the producer
// and a tool that narrowed them would reject conformant foreign documents.
func ParseTypeKey(s string) (TypeKey, error) {
	if strings.TrimSpace(s) == "" {
		return "", &errs.Error{
			Code:    errs.EINVALID,
			Message: "gnosis.ParseTypeKey: type must not be empty",
		}
	}
	return TypeKey(s), nil
}

// ParseSubjectKey converts s into a SubjectKey.
//
// Requires: nothing.
// Ensures: returns EINVALID for an empty or whitespace-only value, or one
// containing whitespace. A key with an interior space would be indistinguishable
// from a surface phrase at a glance, and the two must never be confused.
func ParseSubjectKey(s string) (SubjectKey, error) {
	const op = "gnosis.ParseSubjectKey"
	switch {
	case strings.TrimSpace(s) == "":
		return "", &errs.Error{Code: errs.EINVALID, Message: op + ": subject must not be empty"}
	case strings.ContainsFunc(s, isSpace):
		return "", &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": subject key must not contain whitespace; aliases carry phrasing",
		}
	}
	return SubjectKey(s), nil
}

// Fold normalises a surface phrase for comparison.
//
// Requires: nothing.
// Ensures: two phrases differing only in whitespace runs, typographic
// characters, or case fold to the same value. Folding uses skillet/textnorm so
// that gnosis and every other family tool agree on when two strings are the
// same — a second normaliser is the drift the shared kernel exists to prevent.
func (s Surface) Fold() string {
	return strings.ToLower(textnorm.Fold(string(s)))
}

// String renders the type key.
func (t TypeKey) String() string { return string(t) }

// String renders the subject key.
func (s SubjectKey) String() string { return string(s) }

// String renders the surface phrase as written.
func (s Surface) String() string { return string(s) }

// isSpace reports whether r is whitespace, for the subject-key check.
func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}
