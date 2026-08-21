// Package gnosis is the domain: plain types, service interfaces, and the
// constructors that make invalid states unrepresentable.
//
// It imports no application package, no database driver, and no HTTP package.
// Every other internal package depends on this one; none depend on each other.
// That layering is enforced by depguard rather than by review — see PLAN.md §0.4.
//
// Errors are skillet/errs values. This package does not define an Error type:
// a fifth copy of the family's error vocabulary is exactly the drift the shared
// kernel exists to prevent.
package gnosis

import "github.com/StevenACoffman/skillet/errs"

// idLen is the length of a canonical UUID in 8-4-4-4-12 hyphenated form.
const idLen = 36

// uuidVersion7 is the version nibble gnosis assigns. v7 is time-ordered, so
// identifiers sort chronologically and give the index natural locality, and it
// is collision-free where a second-resolution timestamp is not.
const uuidVersion7 = '7'

// ID is a concept or document identity: an immutable UUIDv7 in hyphenated form.
//
// Identity is assigned once, at admission, and never rewritten — not on move,
// not on retitle, not on supersession, not when a body is replaced. A superseded
// concept keeps the identifier it was born with, which is what makes "what did
// we believe in March, and why did it change" answerable.
type ID string

// ParseID converts s into an ID.
//
// Requires: nothing; any string may be offered.
// Ensures: returns EINVALID unless s is a lowercase hyphenated UUID whose
// version nibble is 7. The returned ID is never empty when err is nil.
func ParseID(s string) (ID, error) {
	const op = "gnosis.ParseID"
	if len(s) != idLen {
		return "", &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": identifier must be a 36-character hyphenated UUID",
		}
	}
	// Position 14 is the version nibble in 8-4-4-4-12 layout: the first
	// character of the third group.
	if s[14] != uuidVersion7 {
		return "", &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": identifier must be UUID version 7",
		}
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return "", &errs.Error{
					Code:    errs.EINVALID,
					Message: op + ": hyphen expected in UUID layout",
				}
			}
			continue
		}
		if !isLowerHex(r) {
			return "", &errs.Error{
				Code:    errs.EINVALID,
				Message: op + ": identifier must be lowercase hexadecimal",
			}
		}
	}
	return ID(s), nil
}

// String renders the identifier for display and storage.
func (i ID) String() string { return string(i) }

// isLowerHex reports whether r is a lowercase hexadecimal digit. Uppercase is
// rejected rather than folded so that one identifier has exactly one spelling —
// two spellings would make the redundant-record comparison in PLAN.md §0.1
// report a false discrepancy.
func isLowerHex(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'a' && r <= 'f':
		return true
	default:
		return false
	}
}
