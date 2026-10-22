package index

import (
	"context"
	"fmt"

	toerr "github.com/StevenACoffman/toerr/errors"
)

// ClaimSourceRow is one claim's dependence on one archived source.
//
// **The join §6.2's first rule needs, and it did not exist.** That rule selects "claims
// sharing a `source_id` with the incoming source", and until this table there was no way
// to ask: `sources_fetched` projects tier 0, `claims` projects the corpus, and a claim's
// archive paths lived only in its frontmatter. A selector that re-read every document to
// answer would keep the determinism §6.2 requires and give up the reason it names the
// index — that selection is a query rather than a walk.
type ClaimSourceRow struct {
	ClaimID string

	// ArchivePath is the tier-0 file the claim's evidence names, bundle-relative. It
	// is the join key rather than the URI because it is what the claim actually
	// carries, and because two versions of one source are two archive paths — which is
	// the distinction §10.2's evidence-divergence predicate exists to see.
	ArchivePath string
}

// claimSourceSchema projects each claim's evidence onto tier 0.
//
// Appended as its own migration rather than added to an existing one, which this
// repository learned the hard way in §5.5.3: `schema-shape` compares object *names* and
// is blind to a column definition, so an edited migration leaves every existing index
// with the old shape and fails at the first insert — as a runtime error nothing warned
// about.
//
// No foreign key onto `sources_fetched`. A claim may cite an archive path this user has
// not fetched: tier 0 is per-user and the corpus is shared (§4.5), so a constraint would
// refuse to index a conformant document because of what somebody else had downloaded.
// The claim key does cascade, because a claim's evidence cannot outlive the claim.
func claimSourceSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS claim_sources (
			claim_id     TEXT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
			archive_path TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS claim_sources_claim ON claim_sources (claim_id)`,
		`CREATE INDEX IF NOT EXISTS claim_sources_path ON claim_sources (archive_path)`,
	}
}

// insertClaimSource writes one row.
func insertClaimSource(ctx context.Context, tx execer, r *ClaimSourceRow) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO claim_sources (claim_id, archive_path) VALUES (?, ?)`,
		r.ClaimID, r.ArchivePath)
	if err != nil {
		return fmt.Errorf("insert claim source %s/%s: %w", r.ClaimID, r.ArchivePath, err)
	}
	return nil
}

// ClaimSources lists every claim-source row, ordered so two reads are comparable.
//
// Requires: db is open.
// Ensures: an empty slice, never nil, for a corpus whose claims cite nothing archived.
func (db *DB) ClaimSources(ctx context.Context) ([]ClaimSourceRow, error) {
	const op = "index.DB.ClaimSources"

	rows, err := db.sql.QueryContext(ctx,
		`SELECT claim_id, archive_path FROM claim_sources
		 ORDER BY claim_id, archive_path`)
	if err != nil {
		return nil, toerr.WrapWithMessage(err, op)
	}
	defer func() { _ = rows.Close() }()

	out := make([]ClaimSourceRow, 0)
	for rows.Next() {
		var r ClaimSourceRow
		if sErr := rows.Scan(&r.ClaimID, &r.ArchivePath); sErr != nil {
			return nil, toerr.WrapWithMessage(sErr, op)
		}
		out = append(out, r)
	}
	if rErr := rows.Err(); rErr != nil {
		return nil, toerr.WrapWithMessage(rErr, op)
	}
	return out, nil
}
