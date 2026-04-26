// Package subagent records SubagentStart/Stop and TaskCreated/Completed
// hook events into a per-repo events table and bumps the parent agent's
// heartbeat so long subagent work doesn't mark the parent stale.
package subagent

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zsiec/squad/internal/store"
)

type Event struct {
	AgentID    string
	SubagentID string
	Type       string
	EventName  string
}

type Recorder struct {
	db     *sql.DB
	repoID string
	now    func() time.Time
}

func New(db *sql.DB, repoID string, now func() time.Time) *Recorder {
	if now == nil {
		now = time.Now
	}
	return &Recorder{db: db, repoID: repoID, now: now}
}

func (r *Recorder) Record(ctx context.Context, e Event) error {
	nowTs := r.now().Unix()
	return store.WithTxRetry(ctx, r.db, func(tx *sql.Tx) error {
		var present int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM agents WHERE id = ?`, e.AgentID).Scan(&present)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		var duration sql.NullInt64
		if pair := pairEvent(e.EventName); pair != "" {
			var startTs int64
			err := tx.QueryRowContext(ctx, `
                SELECT ts FROM subagent_events s
                WHERE s.subagent_id = ? AND s.event = ?
                  AND NOT EXISTS (
                    SELECT 1 FROM subagent_events e
                    WHERE e.subagent_id = s.subagent_id AND e.event = ? AND e.id > s.id
                  )
                ORDER BY s.id DESC LIMIT 1`,
				e.SubagentID, pair, e.EventName).Scan(&startTs)
			if err == nil {
				duration = sql.NullInt64{Int64: (nowTs - startTs) * 1000, Valid: true}
			}
		}

		var subType sql.NullString
		if e.Type != "" {
			subType = sql.NullString{String: e.Type, Valid: true}
		}
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO subagent_events (repo_id, agent_id, subagent_id, subagent_type, event, ts, duration_ms)
            VALUES (?, ?, ?, ?, ?, ?, ?)`,
			r.repoID, e.AgentID, e.SubagentID, subType, e.EventName, nowTs, duration); err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx,
			`UPDATE agents SET last_tick_at = ? WHERE id = ?`, nowTs, e.AgentID)
		return err
	})
}

func pairEvent(eventName string) string {
	switch eventName {
	case "subagent_stop":
		return "subagent_start"
	case "task_completed":
		return "task_created"
	default:
		return ""
	}
}
