// Package subagent records SubagentStart/Stop and TaskCreated/Completed
// hook events into a per-repo events table and bumps the parent agent's
// heartbeat so long subagent work doesn't mark the parent stale.
package subagent

import (
	"context"
	"database/sql"
	"errors"
	"time"
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var present int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM agents WHERE id = ?`, e.AgentID).Scan(&present)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}

	var duration sql.NullInt64
	if pair := pairEvent(e.EventName); pair != "" {
		var startTs int64
		err := tx.QueryRowContext(ctx, `
            SELECT ts FROM subagent_events
            WHERE subagent_id = ? AND event = ?
            ORDER BY id DESC LIMIT 1`,
			e.SubagentID, pair).Scan(&startTs)
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

	if _, err := tx.ExecContext(ctx,
		`UPDATE agents SET last_tick_at = ? WHERE id = ?`, nowTs, e.AgentID); err != nil {
		return err
	}
	return tx.Commit()
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
