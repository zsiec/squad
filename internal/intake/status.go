package intake

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Turn is one row from intake_turns hydrated for caller display.
type Turn struct {
	Seq          int       `json:"seq"`
	Role         string    `json:"role"`
	Content      string    `json:"content"`
	FieldsFilled []string  `json:"fields_filled,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// StatusResult bundles everything a caller needs to resume an intake
// session: the session metadata, every turn ordered by seq, and the
// still-required checklist gaps for the locked shape (or for item_only
// as the default when shape is not yet locked).
type StatusResult struct {
	Session       Session  `json:"session"`
	Transcript    []Turn   `json:"transcript"`
	StillRequired []string `json:"still_required"`
}

// Status returns the full transcript and current still_required for the
// session. Read-only — no side effects. Errors:
//   - ErrIntakeNotFound — session id unknown
//   - ErrIntakeNotYours — session owned by a different agent
//
// A cancelled or committed session still returns a populated transcript
// so callers can render audit history.
func Status(ctx context.Context, db *sql.DB, checklist Checklist, sessionID, agentID string) (StatusResult, error) {
	s, err := loadSession(ctx, db, sessionID)
	if err != nil {
		return StatusResult{}, err
	}
	if s.AgentID != agentID {
		return StatusResult{}, ErrIntakeNotYours
	}

	turns, allFilled, err := loadTurns(ctx, db, sessionID)
	if err != nil {
		return StatusResult{}, err
	}

	shape := s.Shape
	if shape == "" {
		shape = ShapeItemOnly
	}
	return StatusResult{
		Session:       s,
		Transcript:    turns,
		StillRequired: checklist.StillRequired(shape, allFilled),
	}, nil
}

func loadTurns(ctx context.Context, db *sql.DB, sessionID string) ([]Turn, []string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT seq, role, content, fields_filled, created_at
		FROM intake_turns
		WHERE session_id=?
		ORDER BY seq
	`, sessionID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var turns []Turn
	seen := map[string]struct{}{}
	var allFilled []string
	for rows.Next() {
		var (
			t         Turn
			rawFields sql.NullString
			createdAt int64
		)
		if err := rows.Scan(&t.Seq, &t.Role, &t.Content, &rawFields, &createdAt); err != nil {
			return nil, nil, err
		}
		t.CreatedAt = time.Unix(createdAt, 0).UTC()
		if rawFields.Valid && rawFields.String != "" {
			if err := json.Unmarshal([]byte(rawFields.String), &t.FieldsFilled); err != nil {
				return nil, nil, fmt.Errorf("intake: decode fields_filled at seq %d: %w", t.Seq, err)
			}
			for _, f := range t.FieldsFilled {
				if _, dup := seen[f]; dup {
					continue
				}
				seen[f] = struct{}{}
				allFilled = append(allFilled, f)
			}
		}
		turns = append(turns, t)
	}
	return turns, allFilled, rows.Err()
}
