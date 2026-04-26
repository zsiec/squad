package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

// testEnv is a thin wrapper used by R6 round-trip tests of the pure-function
// verb entry points. setupSquadRepo wires a tmp git repo + `squad init`; this
// helper additionally opens the squad store and resolves repo/agent ids so
// tests can call e.g. NextItem(ctx, NextArgs{DB: env.DB, ...}) without
// re-deriving them by hand.
type testEnv struct {
	Root     string
	DB       *sql.DB
	RepoID   string
	AgentID  string
	ItemsDir string
	DoneDir  string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	root := setupSquadRepo(t)
	t.Chdir(root)
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("store.OpenDefault: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repoID, err := repo.IDFor(root)
	if err != nil {
		t.Fatalf("repo.IDFor: %v", err)
	}
	agentID, err := identity.AgentID()
	if err != nil {
		t.Fatalf("identity.AgentID: %v", err)
	}
	return &testEnv{
		Root:     root,
		DB:       db,
		RepoID:   repoID,
		AgentID:  agentID,
		ItemsDir: filepath.Join(root, ".squad", "items"),
		DoneDir:  filepath.Join(root, ".squad", "done"),
	}
}
