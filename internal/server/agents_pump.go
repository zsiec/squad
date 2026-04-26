package server

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// agentsPump bridges the agents table to the in-process bus. Snapshot-diff
// only — no cursor table — because the agents row is mutated in place on
// each tick. Detects three transitions:
//   - appeared: agent_id not present in prior snapshot
//   - updated:  last_tick_at or status changed
//   - vanished: agent_id present in prior snapshot, absent now (squad does
//     not currently delete agent rows, but the path is here for completeness)
type agentsPump struct {
	db       *sql.DB
	repoID   string
	bus      *chat.Bus
	interval time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type agentSnap struct {
	lastTickAt int64
	status     string
}

func newAgentsPump(db *sql.DB, repoID string, bus *chat.Bus) *agentsPump {
	return &agentsPump{
		db:       db,
		repoID:   repoID,
		bus:      bus,
		interval: 500 * time.Millisecond,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (p *agentsPump) start() { go p.loop() }

func (p *agentsPump) stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
	<-p.doneCh
}

func (p *agentsPump) loop() {
	defer close(p.doneCh)
	prev, _ := p.snapshot()

	t := time.NewTicker(p.interval)
	defer t.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-t.C:
			cur, err := p.snapshot()
			if err != nil {
				continue
			}
			p.diffAndEmit(prev, cur)
			prev = cur
		}
	}
}

func (p *agentsPump) snapshot() (map[string]agentSnap, error) {
	q := `SELECT id, last_tick_at, status FROM agents`
	args := []any{}
	if p.repoID != "" {
		q += ` WHERE repo_id = ?`
		args = append(args, p.repoID)
	}
	rows, err := p.db.QueryContext(context.Background(), q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]agentSnap)
	for rows.Next() {
		var id, status string
		var ts int64
		if err := rows.Scan(&id, &ts, &status); err != nil {
			return nil, err
		}
		out[id] = agentSnap{lastTickAt: ts, status: status}
	}
	return out, rows.Err()
}

func (p *agentsPump) diffAndEmit(prev, cur map[string]agentSnap) {
	for id, snap := range cur {
		old, existed := prev[id]
		switch {
		case !existed:
			p.bus.Publish(chat.Event{
				Kind: "agent_status",
				Payload: map[string]any{
					"agent_id":     id,
					"last_tick_at": snap.lastTickAt,
					"status":       snap.status,
					"kind":         "appeared",
				},
			})
		case old.lastTickAt != snap.lastTickAt || old.status != snap.status:
			p.bus.Publish(chat.Event{
				Kind: "agent_status",
				Payload: map[string]any{
					"agent_id":     id,
					"last_tick_at": snap.lastTickAt,
					"status":       snap.status,
					"kind":         "updated",
				},
			})
		}
	}
	for id, old := range prev {
		if _, stillThere := cur[id]; stillThere {
			continue
		}
		p.bus.Publish(chat.Event{
			Kind: "agent_status",
			Payload: map[string]any{
				"agent_id":     id,
				"last_tick_at": old.lastTickAt,
				"status":       old.status,
				"kind":         "vanished",
			},
		})
	}
}
