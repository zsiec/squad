// Package workspace runs read-only aggregation queries across every repo
// registered in the global SQLite DB. Defaults to "all repos"; filters via
// Filter.Mode (current/other/explicit).
package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type Scope int

const (
	ScopeAll Scope = iota
	ScopeCurrent
	ScopeOther
	ScopeExplicit
)

type Filter struct {
	Mode           Scope
	CurrentRepoID  string
	ExplicitIDs    []string
	StaleThreshold time.Duration
}

type StatusRow struct {
	RepoID, RemoteURL, RootPath      string
	InProgress, Ready, Blocked, Done int
	LastTickAge                      time.Duration
	LastActiveAt                     int64
}

type ListRow struct {
	RepoID, RemoteURL, RootPath string
	LastActiveAt                int64
	ItemCount                   int
}

type WhoRow struct {
	RepoID, RemoteURL, AgentID, DisplayName string
	LastTickAge                             time.Duration
	ClaimItem, Intent                       string
}

type NextOptions struct{ Limit int }

type NextRow struct {
	RepoID, RemoteURL, ID, Priority, Title, Estimate, Claimed string
}

type Workspace struct {
	db    *sql.DB
	items ItemSource
	now   func() time.Time
}

func New(db *sql.DB) *Workspace {
	return &Workspace{db: db, items: dbItemSource{db: db}, now: time.Now}
}

// newWithItems is a test seam: inject a fixed ItemSource (for in-memory test
// fixtures that haven't populated the items mirror table).
func newWithItems(db *sql.DB, items []ItemRef) *Workspace {
	return &Workspace{db: db, items: staticItemSource(items), now: time.Now}
}

func (w *Workspace) scopeIDs(ctx context.Context, f Filter) ([]string, error) {
	switch f.Mode {
	case ScopeCurrent:
		if f.CurrentRepoID == "" {
			return nil, fmt.Errorf("workspace: current scope requires CurrentRepoID")
		}
		return []string{f.CurrentRepoID}, nil
	case ScopeExplicit:
		return f.ExplicitIDs, nil
	}
	rows, err := w.db.QueryContext(ctx, `SELECT id FROM repos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if f.Mode == ScopeOther && id == f.CurrentRepoID {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// lastActiveAt derives the most recent activity timestamp for a repo from
// claims.claimed_at, claims.last_touch, and messages.ts (whichever is highest).
// Falls back to repos.created_at.
func (w *Workspace) lastActiveAt(ctx context.Context, repoID string) int64 {
	var ts int64
	_ = w.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(t), 0) FROM (
			SELECT MAX(claimed_at) AS t FROM claims WHERE repo_id = ?
			UNION ALL SELECT MAX(last_touch) FROM claims WHERE repo_id = ?
			UNION ALL SELECT MAX(ts) FROM messages WHERE repo_id = ?
			UNION ALL SELECT created_at FROM repos WHERE id = ?
		)
	`, repoID, repoID, repoID, repoID).Scan(&ts)
	return ts
}

func (w *Workspace) repoMeta(ctx context.Context, repoID string) (remote, root string, err error) {
	err = w.db.QueryRowContext(ctx,
		`SELECT COALESCE(remote_url,''), COALESCE(root_path,'') FROM repos WHERE id = ?`,
		repoID).Scan(&remote, &root)
	return
}

func (w *Workspace) Status(ctx context.Context, f Filter) ([]StatusRow, error) {
	ids, err := w.scopeIDs(ctx, f)
	if err != nil {
		return nil, err
	}
	now := w.now().Unix()
	var out []StatusRow
	for _, id := range ids {
		remote, root, err := w.repoMeta(ctx, id)
		if err != nil {
			return nil, err
		}
		la := w.lastActiveAt(ctx, id)
		if f.StaleThreshold > 0 && la > 0 && time.Duration(now-la)*time.Second > f.StaleThreshold {
			continue
		}
		var inProgress int
		if err := w.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM claims WHERE repo_id = ?`, id).Scan(&inProgress); err != nil {
			return nil, err
		}
		items, err := w.items.For(ctx, id)
		if err != nil {
			return nil, err
		}
		var ready, blocked, done int
		for _, it := range items {
			switch it.Status {
			case "blocked":
				blocked++
			case "done":
				done++
			case "", "ready", "open":
				ready++
			}
		}
		var age time.Duration
		if la > 0 {
			age = time.Duration(now-la) * time.Second
		}
		out = append(out, StatusRow{
			RepoID:       id,
			RemoteURL:    remote,
			RootPath:     root,
			InProgress:   inProgress,
			Ready:        ready,
			Blocked:      blocked,
			Done:         done,
			LastTickAge:  age,
			LastActiveAt: la,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActiveAt > out[j].LastActiveAt })
	return out, nil
}

func (w *Workspace) Next(ctx context.Context, f Filter, opts NextOptions) ([]NextRow, error) {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	ids, err := w.scopeIDs(ctx, f)
	if err != nil {
		return nil, err
	}
	priScore := map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}
	var out []NextRow
	for _, id := range ids {
		remote, _, _ := w.repoMeta(ctx, id)
		items, err := w.items.For(ctx, id)
		if err != nil {
			return nil, err
		}
		claimed := map[string]string{}
		rows, err := w.db.QueryContext(ctx,
			`SELECT item_id, agent_id FROM claims WHERE repo_id = ?`, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var itemID, agentID string
			if err := rows.Scan(&itemID, &agentID); err != nil {
				rows.Close()
				return nil, err
			}
			claimed[itemID] = agentID
		}
		rows.Close()
		for _, it := range items {
			if it.Priority != "P0" && it.Priority != "P1" {
				continue
			}
			if it.Status == "done" || it.Status == "blocked" {
				continue
			}
			out = append(out, NextRow{
				RepoID:    id,
				RemoteURL: remote,
				ID:        it.ID,
				Priority:  it.Priority,
				Title:     it.Title,
				Estimate:  it.Estimate,
				Claimed:   claimed[it.ID],
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := priScore[out[i].Priority], priScore[out[j].Priority]
		if pi != pj {
			return pi < pj
		}
		if out[i].RepoID != out[j].RepoID {
			return out[i].RepoID < out[j].RepoID
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

func (w *Workspace) Who(ctx context.Context, f Filter) ([]WhoRow, error) {
	ids, err := w.scopeIDs(ctx, f)
	if err != nil {
		return nil, err
	}
	now := w.now().Unix()
	var out []WhoRow
	for _, id := range ids {
		remote, _, _ := w.repoMeta(ctx, id)
		rows, err := w.db.QueryContext(ctx, `
			SELECT a.id, COALESCE(a.display_name,''), a.last_tick_at,
			       COALESCE(c.item_id,''), COALESCE(c.intent,'')
			FROM agents a
			LEFT JOIN claims c ON c.agent_id = a.id AND c.repo_id = a.repo_id
			WHERE a.repo_id = ?
		`, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r WhoRow
			var tick int64
			r.RepoID = id
			r.RemoteURL = remote
			if err := rows.Scan(&r.AgentID, &r.DisplayName, &tick, &r.ClaimItem, &r.Intent); err != nil {
				rows.Close()
				return nil, err
			}
			if tick > 0 {
				r.LastTickAge = time.Duration(now-tick) * time.Second
			}
			out = append(out, r)
		}
		rows.Close()
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastTickAge < out[j].LastTickAge })
	return out, nil
}

func (w *Workspace) List(ctx context.Context) ([]ListRow, error) {
	rows, err := w.db.QueryContext(ctx,
		`SELECT id, COALESCE(remote_url,''), COALESCE(root_path,'') FROM repos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListRow
	for rows.Next() {
		var r ListRow
		if err := rows.Scan(&r.RepoID, &r.RemoteURL, &r.RootPath); err != nil {
			return nil, err
		}
		r.LastActiveAt = w.lastActiveAt(ctx, r.RepoID)
		var n int
		_ = w.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM claims WHERE repo_id = ?`, r.RepoID).Scan(&n)
		r.ItemCount = n
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastActiveAt > out[j].LastActiveAt })
	return out, nil
}

func (w *Workspace) Forget(ctx context.Context, repoID string, force bool) error {
	if !force {
		var n int
		if err := w.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM claims WHERE repo_id = ?`, repoID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("repo %s has %d active claims; pass --force to forget anyway", repoID, n)
		}
	}
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, q := range []string{
		`DELETE FROM claims WHERE repo_id = ?`,
		`DELETE FROM messages WHERE repo_id = ?`,
		`DELETE FROM agents WHERE repo_id = ?`,
		`DELETE FROM repos WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, repoID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
