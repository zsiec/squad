package chat

import (
	"database/sql"
	"time"
)

// Chat is the package's top-level service. It writes against the global
// SQLite DB and publishes events on a non-blocking bus.
//
// Construction takes *sql.DB + repoID directly: Phase 1 did not introduce
// a *store.Store wrapper. Repo scoping happens at the per-row level
// (every messages/claims/touches/agents row carries repo_id).
type Chat struct {
	db     *sql.DB
	repoID string
	bus    *Bus
	now    func() time.Time
}

func New(db *sql.DB, repoID string) *Chat {
	return &Chat{db: db, repoID: repoID, bus: NewBus(), now: time.Now}
}

func NewWithClock(db *sql.DB, repoID string, clock func() time.Time) *Chat {
	return &Chat{db: db, repoID: repoID, bus: NewBus(), now: clock}
}

func (c *Chat) Bus() *Bus      { return c.bus }
func (c *Chat) DB() *sql.DB    { return c.db }
func (c *Chat) RepoID() string { return c.repoID }
func (c *Chat) nowUnix() int64 { return c.now().Unix() }
