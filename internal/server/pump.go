package server

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// messagesPump bridges the DB and the in-process bus so SSE subscribers see
// events written by *other* processes (the CLI). The CLI's chat.Chat has its
// own bus; without the pump, server-mediated POSTs were the only events the
// dashboard's SSE stream ever saw.
//
// The pump is a SELECT polling loop. It tracks the highest message id seen
// and emits everything newer on each tick. Latency is the polling interval
// (default 500ms); CLI writes show up "live enough" without forcing every
// command to talk through the HTTP API.
type messagesPump struct {
	db       *sql.DB
	repoID   string
	bus      *chat.Bus
	interval time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func newMessagesPump(db *sql.DB, repoID string, bus *chat.Bus) *messagesPump {
	return &messagesPump{
		db:       db,
		repoID:   repoID,
		bus:      bus,
		interval: 500 * time.Millisecond,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (p *messagesPump) start() {
	go p.loop()
}

// stop signals the loop and blocks until the goroutine has actually exited.
// Synchronous teardown matches claimsPump/agentsPump and prevents the
// "TempDir RemoveAll: directory not empty" race where SQLite WAL handles
// outlive Server.Close.
func (p *messagesPump) stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
	<-p.doneCh
}

func (p *messagesPump) loop() {
	defer close(p.doneCh)
	// Initialize cursor at current high-water so we don't replay history
	// at server start. Subscribers that want history can fetch it via
	// /api/messages.
	var lastID int64
	row := p.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(MAX(id), 0) FROM messages`)
	_ = row.Scan(&lastID)

	t := time.NewTicker(p.interval)
	defer t.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-t.C:
			next, err := p.drain(lastID)
			if err == nil {
				lastID = next
			}
		}
	}
}

func (p *messagesPump) drain(after int64) (int64, error) {
	q := `SELECT id, agent_id, thread, kind, COALESCE(body,'') FROM messages WHERE id > ?`
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
		var id int64
		var agent, thread, kind, body string
		if err := rows.Scan(&id, &agent, &thread, &kind, &body); err != nil {
			return highest, err
		}
		highest = id
		p.bus.Publish(chat.Event{
			Kind: "message",
			Payload: map[string]any{
				"id":       id,
				"agent_id": agent,
				"thread":   thread,
				"kind":     kind,
				"body":     body,
			},
		})
	}
	return highest, rows.Err()
}
