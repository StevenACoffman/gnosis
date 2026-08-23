package gnosis

import (
	"github.com/google/uuid"

	"github.com/StevenACoffman/skillet/errs"
)

// NewID assigns a fresh identity.
//
// Requires: nothing.
// Ensures: a valid UUIDv7, or an error — never a zero ID paired with a nil error,
// because a document that silently received the empty identity would collide with
// every other document that did.
//
// **This is the one function in this package that is not pure**, and it is worth
// naming rather than leaving to be discovered. Everything else here is a value
// operation over inputs a caller supplies; this reads a clock and a random source,
// because that is what an assigned identity is. SPEC §5.1.3 states the trade
// directly: identity is assigned rather than derived from content, so a typo
// correction does not change what a document *is* — and the price is that two
// people documenting one subject get two identifiers, which §4.6.1 makes the
// duplication signal's job rather than a defect.
//
// v7 rather than v4 because it is time-ordered, so identifiers sort into creation
// order and an index over them clusters by recency instead of scattering.
func NewID() (ID, error) {
	const op = "gnosis.NewID"

	u, err := uuid.NewV7()
	if err != nil {
		return "", &errs.Error{Op: op, Err: err}
	}
	id, err := ParseID(u.String())
	if err != nil {
		// Unreachable unless uuid changes its rendering, and worth the check
		// anyway: an identifier this package's own parser rejects would be written
		// into a document and fail every later read.
		return "", &errs.Error{Op: op, Err: err}
	}
	return id, nil
}
