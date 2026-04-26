package server

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// claimsPump bridges the claims and claim_history tables to the in-process
// bus. Mirrors messagesPump in shape (500ms tick, repo_id filter, no replay
// of pre-existing rows on start) so SSE subscribers see the same per-row
// granularity for claim lifecycle that they get for chat messages.
//
// Two cursors run side-by-side:
//   - claim_history.id (monotonic) for "released"-style transitions. The
//     outcome column carries done/blocked/released/etc.
//   - claims snapshot map (item_id -> agent + claimed_at) for adds and
//     reassignments. Disappearances are intentionally ignored here — every
//     claim end goes through releaseInTx, which writes to claim_history.
type claimsPump struct {
	db       *sql.DB
	repoID   string
	bus      *chat.Bus
	interval time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
}

type claimSnap struct {
	agentID   string
	claimedAt int64
}

func newClaimsPump(db *sql.DB, repoID string, bus *chat.Bus) *claimsPump {
	return &claimsPump{
		db:       db,
		repoID:   repoID,
		bus:      bus,
		interval: 500 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}
}

func (p *claimsPump) start() { go p.loop() }

func (p *claimsPump) stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
}

func (p *claimsPump) loop() {
	var historyCursor int64
	_ = p.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(MAX(id), 0) FROM claim_history`).Scan(&historyCursor)
	prev, _ := p.snapshotClaims()

	t := time.NewTicker(p.interval)
	defer t.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-t.C:
			if next, err := p.drainHistory(historyCursor); err == nil {
				historyCursor = next
			}
			cur, err := p.snapshotClaims()
			if err != nil {
				continue
			}
			p.diffAndEmit(prev, cur)
			prev = cur
		}
	}
}

func (p *claimsPump) snapshotClaims() (map[string]claimSnap, error) {
	q := `SELECT item_id, agent_id, claimed_at FROM claims`
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
	out := make(map[string]claimSnap)
	for rows.Next() {
		var id, agent string
		var ts int64
		if err := rows.Scan(&id, &agent, &ts); err != nil {
			return nil, err
		}
		out[id] = claimSnap{agentID: agent, claimedAt: ts}
	}
	return out, rows.Err()
}

func (p *claimsPump) drainHistory(after int64) (int64, error) {
	q := `SELECT id, item_id, agent_id, released_at, COALESCE(outcome,'') FROM claim_history WHERE id > ?`
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
		var id, releasedAt int64
		var item, agent, outcome string
		if err := rows.Scan(&id, &item, &agent, &releasedAt, &outcome); err != nil {
			return highest, err
		}
		highest = id
		kind := outcome
		if kind == "" {
			kind = "released"
		}
		p.bus.Publish(chat.Event{
			Kind: "item_changed",
			Payload: map[string]any{
				"item_id":     item,
				"kind":        kind,
				"agent_id":    agent,
				"released_at": releasedAt,
				"outcome":     outcome,
			},
		})
	}
	return highest, rows.Err()
}

func (p *claimsPump) diffAndEmit(prev, cur map[string]claimSnap) {
	for id, snap := range cur {
		old, existed := prev[id]
		switch {
		case !existed:
			p.bus.Publish(chat.Event{
				Kind: "item_changed",
				Payload: map[string]any{
					"item_id":    id,
					"kind":       "claimed",
					"agent_id":   snap.agentID,
					"claimed_at": snap.claimedAt,
				},
			})
		case old.agentID != snap.agentID:
			p.bus.Publish(chat.Event{
				Kind: "item_changed",
				Payload: map[string]any{
					"item_id":    id,
					"kind":       "reassigned",
					"agent_id":   snap.agentID,
					"claimed_at": snap.claimedAt,
				},
			})
		}
	}
}
