package workspace

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

type fixture struct {
	t   *testing.T
	db  *sql.DB
	Now int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &fixture{t: t, db: db, Now: time.Now().Unix()}
}

func (f *fixture) Store() *sql.DB { return f.db }

func (f *fixture) addRepo(t *testing.T, id, remote, root string, lastActiveAgo time.Duration) {
	t.Helper()
	createdAt := f.Now - int64(lastActiveAgo.Seconds())
	if _, err := f.db.Exec(`
		INSERT INTO repos (id, root_path, remote_url, name, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, root, remote, "", createdAt); err != nil {
		t.Fatalf("addRepo: %v", err)
	}
	// Seed last activity via a claim or message ts so lastActiveAt resolves.
	if _, err := f.db.Exec(`
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, priority)
		VALUES (?, ?, 'system', 'global', 'system', '', 'normal')
	`, id, createdAt); err != nil {
		t.Fatalf("seed messages: %v", err)
	}
}

func (f *fixture) addAgent(t *testing.T, repoID, agentID, name string, tickAgo time.Duration) {
	t.Helper()
	tick := f.Now - int64(tickAgo.Seconds())
	if _, err := f.db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, '/tmp/wt', 1, ?, ?, 'active')
	`, agentID, repoID, name, tick, tick); err != nil {
		t.Fatalf("addAgent: %v", err)
	}
}

func (f *fixture) addClaim(t *testing.T, repoID, agentID, itemID, intent string) {
	t.Helper()
	if _, err := f.db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`, repoID, itemID, agentID, f.Now, f.Now, intent); err != nil {
		t.Fatalf("addClaim: %v", err)
	}
}

func (f *fixture) addItem(t *testing.T, repoID, id, priority, status string) {
	t.Helper()
	if _, err := f.db.Exec(`
		INSERT INTO items (repo_id, item_id, title, priority, status, path, updated_at)
		VALUES (?, ?, '', ?, ?, '', ?)
	`, repoID, id, priority, status, f.Now); err != nil {
		t.Fatalf("addItem: %v", err)
	}
}
