package server

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// activityPump bridges the three append-only activity-stream tables
// (commits, attestations, agent_events) onto the in-process bus so the
// per-agent timeline drawer sees new rows without polling. Mirrors the
// shape of messagesPump and claimsPump: 500ms tick, repo-scoped queries,
// no replay of pre-existing rows on start.
//
// The published payloads mirror the `timelineRow` shape returned by
// /api/agents/{id}/timeline so the client renderer can append a new row
// without a separate parse path. Each event carries Kind="agent_activity"
// with payload.source distinguishing the source table.
type activityPump struct {
	db       *sql.DB
	repoID   string
	bus      *chat.Bus
	interval time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}

	initialAttestationCursor int64
	initialEventCursor       int64
	initialCommitTS          int64
	initialCommitSha         string
}

// commitCursor pairs ts with sha so two commits at the exact same unix
// second don't lose the second one to a strict `ts > ?` filter. The
// (repo_id, sha) primary key on commits means sha is unique per repo,
// breaking ties deterministically.
type commitCursor struct {
	ts  int64
	sha string
}

func newActivityPump(db *sql.DB, repoID string, bus *chat.Bus) *activityPump {
	return &activityPump{
		db:       db,
		repoID:   repoID,
		bus:      bus,
		interval: 500 * time.Millisecond,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (p *activityPump) start() {
	ctx := context.Background()
	_ = p.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), 0) FROM attestations`).Scan(&p.initialAttestationCursor)
	_ = p.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), 0) FROM agent_events`).Scan(&p.initialEventCursor)
	// Pin both ts and sha so the strict tuple comparison `(ts > ?) OR
	// (ts = ? AND sha > ?)` skips only this exact row on the next tick.
	_ = p.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(ts), 0), COALESCE((SELECT sha FROM commits WHERE ts = (SELECT MAX(ts) FROM commits) ORDER BY sha DESC LIMIT 1), '') FROM commits`).
		Scan(&p.initialCommitTS, &p.initialCommitSha)
	go p.loop()
}

func (p *activityPump) stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
	<-p.doneCh
}

func (p *activityPump) loop() {
	defer close(p.doneCh)
	attCursor := p.initialAttestationCursor
	evCursor := p.initialEventCursor
	commit := commitCursor{ts: p.initialCommitTS, sha: p.initialCommitSha}

	t := time.NewTicker(p.interval)
	defer t.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-t.C:
			if next, err := p.drainAttestations(attCursor); err == nil {
				attCursor = next
			}
			if next, err := p.drainAgentEvents(evCursor); err == nil {
				evCursor = next
			}
			if next, err := p.drainCommits(commit); err == nil {
				commit = next
			}
		}
	}
}

func (p *activityPump) drainAttestations(after int64) (int64, error) {
	q := `SELECT id, item_id, agent_id, kind, exit_code, created_at FROM attestations WHERE id > ?`
	args := []any{after}
	if p.repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, p.repoID)
	}
	q += ` ORDER BY id`
	rows, err := p.db.QueryContext(context.Background(), q, args...)
	if err != nil {
		return after, err
	}
	defer rows.Close()
	highest := after
	for rows.Next() {
		var id, exitCode, ts int64
		var itemID, agent, kind string
		if err := rows.Scan(&id, &itemID, &agent, &kind, &exitCode, &ts); err != nil {
			return highest, err
		}
		highest = id
		p.bus.Publish(chat.Event{
			Kind: "agent_activity",
			Payload: map[string]any{
				"id":               id,
				"agent_id":         agent,
				"source":           "attestation",
				"kind":             "attestation",
				"ts":               ts,
				"item_id":          itemID,
				"attestation_kind": kind,
				"exit_code":        exitCode,
			},
		})
	}
	return highest, rows.Err()
}

func (p *activityPump) drainAgentEvents(after int64) (int64, error) {
	q := `SELECT id, agent_id, ts, event_kind, tool, target, exit_code FROM agent_events WHERE id > ?`
	args := []any{after}
	if p.repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, p.repoID)
	}
	q += ` ORDER BY id`
	rows, err := p.db.QueryContext(context.Background(), q, args...)
	if err != nil {
		return after, err
	}
	defer rows.Close()
	highest := after
	for rows.Next() {
		var id, ts, exitCode int64
		var agent, eventKind, tool, target string
		if err := rows.Scan(&id, &agent, &ts, &eventKind, &tool, &target, &exitCode); err != nil {
			return highest, err
		}
		highest = id
		p.bus.Publish(chat.Event{
			Kind: "agent_activity",
			Payload: map[string]any{
				"id":         id,
				"agent_id":   agent,
				"source":     "event",
				"kind":       "event",
				"ts":         ts,
				"event_kind": eventKind,
				"tool":       tool,
				"target":     target,
				"exit_code":  exitCode,
			},
		})
	}
	return highest, rows.Err()
}

// drainCommits cursors on a (ts, sha) tuple because the commits table has
// no autoincrement id (PK is (repo_id, sha) for done-idempotency). Tied
// timestamps at second granularity are realistic on multi-file commits;
// sha is unique per repo so it breaks the tie deterministically without
// dropping rows.
func (p *activityPump) drainCommits(after commitCursor) (commitCursor, error) {
	q := `SELECT item_id, agent_id, sha, subject, ts FROM commits WHERE (ts > ? OR (ts = ? AND sha > ?))`
	args := []any{after.ts, after.ts, after.sha}
	if p.repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, p.repoID)
	}
	q += ` ORDER BY ts, sha`
	rows, err := p.db.QueryContext(context.Background(), q, args...)
	if err != nil {
		return after, err
	}
	defer rows.Close()
	cur := after
	for rows.Next() {
		var ts int64
		var itemID, agent, sha, subject string
		if err := rows.Scan(&itemID, &agent, &sha, &subject, &ts); err != nil {
			return cur, err
		}
		cur = commitCursor{ts: ts, sha: sha}
		p.bus.Publish(chat.Event{
			Kind: "agent_activity",
			Payload: map[string]any{
				"agent_id": agent,
				"source":   "commit",
				"kind":     "commit",
				"ts":       ts,
				"item_id":  itemID,
				"sha":      sha,
				"subject":  subject,
			},
		})
	}
	return cur, rows.Err()
}
