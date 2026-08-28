package index

import (
	"context"
	"fmt"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
)

// ClaimRow is one claim's address in the index.
//
// It carries only what §5.5.1 makes recoverable from the document: an identity, the
// document holding it, the fold hash of its anchoring text, a byte offset when the
// anchor could be located, and the type of the document it belongs to.
//
// **The summary columns are absent from this type, not merely unset.** `title`,
// `description` and `lead` come from extraction (§10.2.1), and a writer that could
// supply them would be a writer that could get them wrong. Leaving them off the row
// means the address can be written today without anybody deciding what an unextracted
// title is; §5.5.3 makes the columns NULL, and NULL is what this writer leaves them.
type ClaimRow struct {
	ID         string
	DocumentID gnosis.ID

	// AnchorHash is the fold hash of the claim's anchoring text, computed the way
	// `claim-anchor` computes it so there is one answer to where a claim is.
	AnchorHash string

	// Pos is the byte offset of the anchor in the document body, or nil when it
	// could not be located.
	//
	// A pointer because the column is nullable and `0` is a real position — the
	// first byte of the body. §5.5.2 defines NULL as "this claim's text is not where
	// its anchor says it is", and a caller reading `0` as that state would send
	// readers to the top of the document.
	Pos *int

	// Type is the type of the document holding this claim, denormalised so a query
	// over claims need not join documents to filter by it.
	Type string

	// Lead is the claim's conclusion, stated first (§17.4), or empty when extraction
	// has not written one. Empty is stored as NULL, per §5.5.3: the empty string is a
	// real lead and would assert that the claim has no conclusion.
	Lead string
}

// VerificationRow is one OKF §5.2 verification event, as the index holds it.
//
// It has no key of its own: §5.5 makes `verified` a list of *events*, and two events
// with one actor at one time are indistinguishable and both real. A synthetic key would
// invite a caller to treat them as updatable, which an event is not.
type VerificationRow struct {
	ClaimID string
	By      string
	At      string
}

// insertVerification writes one verification event.
func insertVerification(ctx context.Context, tx execer, v *VerificationRow) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO verifications (claim_id, by, at) VALUES (?, ?, ?)`,
		v.ClaimID, v.By, v.At)
	if err != nil {
		return fmt.Errorf("insert verification for claim %s: %w", v.ClaimID, err)
	}
	return nil
}

// insertClaim writes one row.
//
// Unexported and taking a transaction handle so Replace can compose it; callers
// outside this package never see a transaction.
//
// **`claims_fts` is written only for a claim that has a lead**, which is §5.5.3's rule
// applied rather than repeated. Indexing a row whose three indexed columns are NULL puts
// a searchable entry in the table that matches nothing; indexing one that has a lead puts
// in something a query can find. So claim search covers the extracted part of the corpus
// and must say what it did not cover — a partial answer presented as a whole one is the
// defect `index rebuild` and the link report both had.
func insertClaim(ctx context.Context, tx execer, c *ClaimRow) error {
	var lead any
	if c.Lead != "" {
		lead = c.Lead
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO claims (id, document_id, anchor_hash, pos, type, lead)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.DocumentID.String(), c.AnchorHash, c.Pos, c.Type, lead)
	if err != nil {
		return fmt.Errorf("insert claim %s: %w", c.ID, err)
	}
	if c.Lead == "" {
		return nil
	}
	rowID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("row id for claim %s: %w", c.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO claims_fts (rowid, lead) VALUES (?, ?)`, rowID, c.Lead); err != nil {
		return fmt.Errorf("index claim %s for search: %w", c.ID, err)
	}
	return nil
}
