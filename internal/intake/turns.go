package intake

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// validRoles are the only role strings AppendTurn accepts. Anything else
// is a programmer error and gets rejected at the API surface so corrupt
// transcripts can't reach the database.
var validRoles = map[string]struct{}{
	"user":   {},
	"agent":  {},
	"system": {},
}

// AppendTurn records one Q/A turn against an open session and returns
// the assigned seq plus the still-required checklist fields after this
// turn's fieldsFilled is applied. The session's shape locks on the very
// first AppendTurn — to spec_epic_items if fieldsFilled contains any
// dotted name, otherwise to item_only — and that lock guarantees
// still_required is monotonically non-growing across subsequent turns.
// Once locked, fieldsFilled that conflict with the shape (flat names
// against spec_epic_items, dotted names against item_only) are
// rejected.
//
// Errors:
//   - ErrIntakeNotFound        — session id unknown
//   - ErrIntakeNotYours        — session owned by a different agent
//   - ErrIntakeAlreadyClosed   — session is committed or cancelled
//   - validation error (role, content, shape conflict) — fmt.Errorf
//
// fieldsFilled is the agent's honor-system claim about which checklist
// fields this turn satisfies. Squad does not natural-language-parse
// content; the structural commit-time validator (Validate) is the real
// gate.
func AppendTurn(
	ctx context.Context,
	db *sql.DB,
	checklist Checklist,
	sessionID, agentID, role, content string,
	fieldsFilled []string,
) (int, []string, error) {
	if _, ok := validRoles[role]; !ok {
		return 0, nil, fmt.Errorf("intake: invalid role %q (want user|agent|system)", role)
	}
	if strings.TrimSpace(content) == "" {
		return 0, nil, fmt.Errorf("intake: turn content must not be empty")
	}

	var owner, status string
	err := db.QueryRowContext(ctx,
		`SELECT agent_id, status FROM intake_sessions WHERE id=?`, sessionID,
	).Scan(&owner, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, ErrIntakeNotFound
	}
	if err != nil {
		return 0, nil, err
	}
	if owner != agentID {
		return 0, nil, ErrIntakeNotYours
	}
	if status != StatusOpen {
		return 0, nil, ErrIntakeAlreadyClosed
	}

	hasDotted, hasFlat := classifyFields(fieldsFilled)
	if hasDotted && hasFlat {
		return 0, nil, fmt.Errorf("intake: fieldsFilled mixes dotted and flat names — pick one shape per turn")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback()

	var statusInTx string
	var lockedShape sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT status, shape FROM intake_sessions WHERE id=?`, sessionID,
	).Scan(&statusInTx, &lockedShape); err != nil {
		return 0, nil, err
	}
	if statusInTx != StatusOpen {
		return 0, nil, ErrIntakeAlreadyClosed
	}

	shape := lockedShape.String
	if shape == "" {
		if hasDotted {
			shape = ShapeSpecEpicItems
		} else {
			shape = ShapeItemOnly
		}
	} else if (shape == ShapeItemOnly && hasDotted) || (shape == ShapeSpecEpicItems && hasFlat) {
		return 0, nil, fmt.Errorf("intake: session locked to %s; conflicting fieldsFilled rejected", shape)
	}

	var maxSeq int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM intake_turns WHERE session_id=?`, sessionID,
	).Scan(&maxSeq); err != nil {
		return 0, nil, err
	}
	seq := maxSeq + 1

	fieldsJSON, err := encodeFields(fieldsFilled)
	if err != nil {
		return 0, nil, err
	}
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO intake_turns (session_id, seq, role, content, fields_filled, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, sessionID, seq, role, content, fieldsJSON, now); err != nil {
		return 0, nil, err
	}

	if !lockedShape.Valid || lockedShape.String == "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE intake_sessions SET shape=?, updated_at=? WHERE id=?`,
			shape, now, sessionID,
		); err != nil {
			return 0, nil, err
		}
	} else if _, err := tx.ExecContext(ctx,
		`UPDATE intake_sessions SET updated_at=? WHERE id=?`, now, sessionID,
	); err != nil {
		return 0, nil, err
	}

	allFilled, err := collectFilled(ctx, tx, sessionID)
	if err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, err
	}

	still := checklist.StillRequired(shape, allFilled)
	return seq, still, nil
}

// classifyFields reports whether fieldsFilled contains any dotted
// ("spec.X" / "epic.X" / "item.X") names and/or any bare names. A turn
// with both is a caller-side error: each turn must commit to one shape.
func classifyFields(fields []string) (hasDotted, hasFlat bool) {
	for _, f := range fields {
		if strings.Contains(f, ".") {
			hasDotted = true
		} else {
			hasFlat = true
		}
	}
	return
}

func encodeFields(fields []string) (any, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("intake: encode fields_filled: %w", err)
	}
	return string(b), nil
}

// collectFilled returns the union of fields_filled across every turn in
// the session, ordered by seq. Decode errors propagate so a corrupt JSON
// row surfaces loudly rather than silently distorting still_required.
func collectFilled(ctx context.Context, tx *sql.Tx, sessionID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT fields_filled FROM intake_turns WHERE session_id=? ORDER BY seq`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	var out []string
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if !raw.Valid || raw.String == "" {
			continue
		}
		var fields []string
		if err := json.Unmarshal([]byte(raw.String), &fields); err != nil {
			return nil, fmt.Errorf("intake: decode fields_filled: %w", err)
		}
		for _, f := range fields {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out, rows.Err()
}
