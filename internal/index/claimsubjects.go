package index

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/StevenACoffman/skillet/errs"
)

// ClaimSubjectRow is one claim's subject, and the value its prose parses to.
//
// **Both halves in one row, written by one writer** (§10.2.1). The declared subject and
// the derived value arrive together because a half-filled row would leave `derived` and
// `pattern_id` meaning neither parsed nor pinned — a state no reader could interpret and
// no check could distinguish from a parse that failed.
type ClaimSubjectRow struct {
	ClaimID    string
	SubjectKey string

	// Op and Value are the parsed constraint. Value is nil when the prose states no
	// quantity.
	//
	// **A row is written even then, and that is the point of the coverage loop.**
	// §10.2.3 has to tell *no quantity present* from *a phrasing the patterns miss*,
	// and a missing row cannot say either. A NULL value says "this claim is about this
	// subject and states no bound the patterns could read".
	Op    string
	Value *float64

	// ValueRaw is the span the value was read from, so a finding can show its parse
	// (§10.2.2) without re-running the parser.
	ValueRaw string

	// Dimension is the subject's declared dimension, denormalised so a comparison need
	// not join the vocabulary to know whether two values are commensurable.
	Dimension string

	// Derived is false only for a pinned `gnosis_constraint` (§10.2.1). A pin takes
	// precedence over the prose and is then checked against it.
	Derived bool

	// PatternID names which operator pattern produced the reading, empty for a pin or
	// for a claim that parsed to nothing.
	PatternID string
}

// insertClaimSubject writes one row.
func insertClaimSubject(ctx context.Context, tx execer, r *ClaimSubjectRow) error {
	var value any
	if r.Value != nil {
		value = *r.Value
	}
	derived := 0
	if r.Derived {
		derived = 1
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO claim_subjects
			(claim_id, subject_key, op, value_norm, value_raw, dimension, derived, pattern_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ClaimID, r.SubjectKey, r.Op, value, r.ValueRaw, r.Dimension, derived, r.PatternID)
	if err != nil {
		return fmt.Errorf("insert claim subject %s/%s: %w", r.ClaimID, r.SubjectKey, err)
	}
	return nil
}

// ClaimSubjects lists every claim-subject row, for the predicates that compare values.
//
// Requires: db is open.
// Ensures: ordered by claim then subject, so two reads agree. Never nil.
func (db *DB) ClaimSubjects(ctx context.Context) ([]ClaimSubjectRow, error) {
	const op = "index.DB.ClaimSubjects"

	rows, err := db.sql.QueryContext(ctx, `
		SELECT claim_id, subject_key, op, value_norm, value_raw, dimension,
		       derived, pattern_id
		FROM claim_subjects ORDER BY claim_id, subject_key`)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = rows.Close() }()

	out := make([]ClaimSubjectRow, 0)
	for rows.Next() {
		var r ClaimSubjectRow
		var value sql.NullFloat64
		var derived int
		if err := rows.Scan(&r.ClaimID, &r.SubjectKey, &r.Op, &value, &r.ValueRaw,
			&r.Dimension, &derived, &r.PatternID); err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		if value.Valid {
			v := value.Float64
			r.Value = &v
		}
		r.Derived = derived == 1
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	return out, nil
}
