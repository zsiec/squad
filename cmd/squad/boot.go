package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/notify"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

type claimContext struct {
	store    *claims.Store
	chat     *chat.Chat
	db       *sql.DB
	repoID   string
	itemsDir string
	doneDir  string
	agentID  string
}

func (c *claimContext) Close() { _ = c.db.Close() }

func bootClaimContext(_ context.Context) (*claimContext, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := repo.Discover(wd)
	if err != nil {
		return nil, err
	}
	db, err := store.OpenDefault()
	if err != nil {
		return nil, err
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	agentID, err := identity.AgentID()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	upgradeUnscopedAgent(db, agentID, repoID)

	chatSvc := chat.New(db, repoID)
	registry := notify.NewRegistry(db)
	chatSvc.SetNotifier(func(ctx context.Context, repoID string) {
		_ = notify.Wake(ctx, registry, repoID, 100*time.Millisecond)
	})
	return &claimContext{
		store:    claims.New(db, repoID, nil),
		chat:     chatSvc,
		db:       db,
		repoID:   repoID,
		itemsDir: filepath.Join(root, ".squad", "items"),
		doneDir:  filepath.Join(root, ".squad", "done"),
		agentID:  agentID,
	}, nil
}

// upgradeUnscopedAgent flips the agent's row from "_unscoped" to repoID when
// the row exists and is unscoped. Best-effort: any DB error is swallowed
// because this runs in the prelude of every CLI command and must not block
// the actual operation. The dashboard / `squad who` filter by repo_id, so
// without this upgrade the agent is invisible despite holding live claims.
func upgradeUnscopedAgent(db *sql.DB, agentID, repoID string) {
	if agentID == "" || repoID == "" || repoID == unscopedRepoID {
		return
	}
	_, _ = db.ExecContext(context.Background(),
		`UPDATE agents SET repo_id = ? WHERE id = ? AND repo_id = ?`,
		repoID, agentID, unscopedRepoID,
	)
}

func findItemPath(itemsDir, itemID string) string {
	entries, err := os.ReadDir(itemsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(itemsDir, e.Name())
		it, err := items.Parse(path)
		if err != nil {
			continue
		}
		if it.ID == itemID {
			return path
		}
	}
	return ""
}
