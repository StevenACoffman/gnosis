package index

import (
	"context"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
)

// The rules §6.2 selects candidates by, named so a result can say which one found a
// claim.
//
// **A candidate that cannot say why it is one is a ranking**, and §6.2's whole claim is
// that selection is a query rather than a judgment. Naming the rule is what makes that
// checkable by a reader rather than asserted by this comment.
const (
	// RuleSharedSource is §6.2's first: claims resting on the incoming source.
	RuleSharedSource CandidateRule = "shared-source"

	// RuleOneHop is §6.2's second: claims in documents linked to or from those.
	RuleOneHop CandidateRule = "one-hop"

	// RuleConflictEdge is §6.2's fourth: claims whose deferred conflicts name the
	// incoming concept. Applied by the shell, because it reads frontmatter the index
	// does not hold — the constant lives here so one vocabulary names all four.
	RuleConflictEdge CandidateRule = "conflict-edge"

	// RuleTagAndRank is §6.2's third, which is not built. `tags` has no projection
	// (§5.5) and the rank cut is a `standards/` value nobody can write from
	// measurement yet (§6.5) — and §6.2 forbids a literal in Go for exactly this.
	RuleTagAndRank CandidateRule = "tag-and-rank"
)

// CandidateRule names which of §6.2's rules selected a claim.
type CandidateRule string

// Candidate is one claim an incoming source might update.
type Candidate struct {
	ClaimID    string        `json:"claim_id"`
	DocumentID string        `json:"document_id"`
	Path       string        `json:"path"`
	Title      string        `json:"title"`
	Lead       string        `json:"lead,omitempty"`
	Rule       CandidateRule `json:"rule"`
}

// CandidatesFor selects the claims an incoming source might update (§6.2, rules 1 and 2).
//
// Requires: db is open; archivePaths are the tier-0 files the incoming source produced;
// limit bounds the result and must be positive.
// Ensures: claims sharing a source first, then the one-hop neighbourhood, each row
// naming the rule that found it, ordered so two runs over one index agree. A claim found
// by both rules appears once, under the first.
//
// # Why the hop is one and stays one
//
// §6.2 specifies "one hop, `links`", and names hop limits beside rank cuts as the values
// that must be declared in `standards/` rather than written as literals in Go. One hop is
// the section's own number rather than a tuned one; a second would need the declared
// value that rule 3 is blocked on, so widening this is that item's work and not a
// parameter to add now.
//
// # Both directions
//
// "linked to or from those" — a concept citing the affected one is as likely to need
// updating as one it cites, and §6.1's memex argument is that following a link should not
// require re-entering the system on a new path.
func (db *DB) CandidatesFor(
	ctx context.Context, archivePaths []string, limit int,
) ([]Candidate, error) {
	const op = "index.DB.CandidatesFor"

	if limit < 1 {
		return nil, errcode.WithCode(gnosis.CodeInvalid, op+": limit must be positive", nil)
	}
	if len(archivePaths) == 0 {
		// No archived text means no claim can rest on it. An empty result rather than
		// an error: a source rejected by tier 0 is an ordinary state, and §6.2 selects
		// nothing for it.
		return []Candidate{}, nil
	}

	shared, err := db.sharedSource(ctx, archivePaths, limit)
	if err != nil {
		return nil, err
	}
	out := shared
	if len(out) >= limit {
		return out[:limit], nil
	}
	neighbours, err := db.oneHop(ctx, archivePaths, limit-len(out))
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(out))
	for _, c := range out {
		seen[c.ClaimID] = true
	}
	for _, c := range neighbours {
		// A claim both rules find is reported once, under the first. Two rows for one
		// claim would make a bounded list shorter than it looks.
		if !seen[c.ClaimID] {
			out = append(out, c)
		}
	}
	return out, nil
}

// sharedSource is §6.2's first rule: claims whose evidence names one of these paths.
func (db *DB) sharedSource(
	ctx context.Context, archivePaths []string, limit int,
) ([]Candidate, error) {
	const op = "index.DB.sharedSource"

	args := make([]any, 0, len(archivePaths)+1)
	for _, p := range archivePaths {
		args = append(args, p)
	}
	args = append(args, limit)
	query := `
		SELECT c.id, d.id, d.path, d.title, COALESCE(c.lead, '')
		FROM claim_sources cs
		JOIN claims c ON c.id = cs.claim_id
		JOIN documents d ON d.id = c.document_id
		WHERE cs.archive_path IN (` + placeholders(len(archivePaths)) + `)
		GROUP BY c.id
		ORDER BY d.path, c.id
		LIMIT ?`
	return db.scanCandidates(ctx, op, query, RuleSharedSource, args)
}

// oneHop is §6.2's second rule: claims in documents one link from an affected one.
func (db *DB) oneHop(
	ctx context.Context, archivePaths []string, limit int,
) ([]Candidate, error) {
	const op = "index.DB.oneHop"

	args := make([]any, 0, len(archivePaths)*2+1)
	for range 2 {
		for _, p := range archivePaths {
			args = append(args, p)
		}
	}
	args = append(args, limit)
	marks := placeholders(len(archivePaths))
	// The affected documents are those holding a claim on one of these paths; the
	// neighbourhood is every document linked to or from one of them, minus the
	// affected set itself — which the first rule already reported.
	query := `
		WITH affected AS (
			SELECT DISTINCT c.document_id AS id
			FROM claim_sources cs JOIN claims c ON c.id = cs.claim_id
			WHERE cs.archive_path IN (` + marks + `)
		),
		neighbours AS (
			SELECT l.target_document_id AS id FROM links l
			WHERE l.source_document_id IN (SELECT id FROM affected)
			  AND l.target_document_id IS NOT NULL
			UNION
			SELECT l.source_document_id AS id FROM links l
			WHERE l.target_document_id IN (SELECT id FROM affected)
		)
		SELECT c.id, d.id, d.path, d.title, COALESCE(c.lead, '')
		FROM claims c
		JOIN documents d ON d.id = c.document_id
		WHERE c.document_id IN (SELECT id FROM neighbours)
		  AND c.document_id NOT IN (
			SELECT DISTINCT c2.document_id
			FROM claim_sources cs2 JOIN claims c2 ON c2.id = cs2.claim_id
			WHERE cs2.archive_path IN (` + marks + `)
		  )
		ORDER BY d.path, c.id
		LIMIT ?`
	return db.scanCandidates(ctx, op, query, RuleOneHop, args)
}

// scanCandidates runs one candidate query and labels every row with its rule.
func (db *DB) scanCandidates(
	ctx context.Context, op, query string, rule CandidateRule, args []any,
) ([]Candidate, error) {
	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, toerr.WrapWithMessage(err, op)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Candidate, 0)
	for rows.Next() {
		c := Candidate{Rule: rule}
		if sErr := rows.Scan(
			&c.ClaimID, &c.DocumentID, &c.Path, &c.Title, &c.Lead,
		); sErr != nil {
			return nil, toerr.WrapWithMessage(sErr, op)
		}
		out = append(out, c)
	}
	if rErr := rows.Err(); rErr != nil {
		return nil, toerr.WrapWithMessage(rErr, op)
	}
	return out, nil
}

// placeholders is an `IN` list of n binds.
//
// Built from a count rather than from the values, so no caller can interpolate a value
// into the statement: the arguments travel as binds and this contributes only question
// marks. That is the one string-built part of these queries and it carries no input.
func placeholders(n int) string {
	if n < 1 {
		return "NULL"
	}
	marks := make([]byte, 0, n*3)
	for i := range n {
		if i > 0 {
			marks = append(marks, ',', ' ')
		}
		marks = append(marks, '?')
	}
	return string(marks)
}
