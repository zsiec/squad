package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func TestRegister_PureWritesAgentRow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-pure-1")
	t.Setenv("SQUAD_AGENT", "")

	res, _, err := Register(context.Background(), RegisterArgs{
		As:          "agent-pure",
		Name:        "Agent Pure",
		NoRepoCheck: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res == nil || res.AgentID != "agent-pure" || res.Name != "Agent Pure" {
		t.Fatalf("unexpected result: %+v", res)
	}

	db, err := store.Open(filepath.Join(dir, "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var gotID, gotName, gotRepo string
	if err := db.QueryRowContext(context.Background(),
		`SELECT id, display_name, repo_id FROM agents WHERE id=?`, "agent-pure",
	).Scan(&gotID, &gotName, &gotRepo); err != nil {
		t.Fatal(err)
	}
	if gotID != "agent-pure" || gotName != "Agent Pure" || gotRepo != "_unscoped" {
		t.Fatalf("got id=%q name=%q repo=%q", gotID, gotName, gotRepo)
	}
}

func seedAgent(t *testing.T, dbPath, id, worktree string, pid int, lastTick int64) {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active')
		ON CONFLICT(id) DO UPDATE SET
			worktree = excluded.worktree,
			pid = excluded.pid,
			last_tick_at = excluded.last_tick_at
	`, id, "_unscoped", id, worktree, pid, lastTick, lastTick); err != nil {
		t.Fatalf("seed agent %s: %v", id, err)
	}
}

func TestRegister_WarnsOnAutoDerivedIdentityCollision(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "collision-session")
	t.Setenv("SQUAD_AGENT", "agent-collision")

	dbPath := filepath.Join(dir, "global.db")
	now := time.Now().Unix()
	seedAgent(t, dbPath, "agent-collision", "/some/other/place", 99999, now)

	res, warnings, err := Register(context.Background(), RegisterArgs{
		NoRepoCheck: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res == nil || res.AgentID != "agent-collision" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected at least one warning, got none")
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "identity collision") {
		t.Fatalf("warning should mention 'identity collision', got: %q", joined)
	}
	if !strings.Contains(joined, "SQUAD_SESSION_ID") {
		t.Fatalf("warning should mention 'SQUAD_SESSION_ID', got: %q", joined)
	}
}

func TestRegister_NoWarnOnSelfReregister(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "self-session")
	t.Setenv("SQUAD_AGENT", "agent-self")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dbPath := filepath.Join(dir, "global.db")
	now := time.Now().Unix()
	seedAgent(t, dbPath, "agent-self", wd, os.Getpid(), now)

	_, warnings, err := Register(context.Background(), RegisterArgs{
		NoRepoCheck: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings on self re-register, got: %v", warnings)
	}
}

func TestRegister_NoWarnOnStaleAgent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "stale-session")
	t.Setenv("SQUAD_AGENT", "agent-stale")

	dbPath := filepath.Join(dir, "global.db")
	stale := time.Now().Add(-2 * time.Hour).Unix()
	seedAgent(t, dbPath, "agent-stale", "/long/dead/wt", 12345, stale)

	_, warnings, err := Register(context.Background(), RegisterArgs{
		NoRepoCheck: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings on stale agent takeover, got: %v", warnings)
	}
}

func TestRegister_NoWarnOnSamePid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "samepid-session")
	t.Setenv("SQUAD_AGENT", "agent-samepid")

	dbPath := filepath.Join(dir, "global.db")
	now := time.Now().Unix()
	seedAgent(t, dbPath, "agent-samepid", "/different/wt", os.Getpid(), now)

	_, warnings, err := Register(context.Background(), RegisterArgs{
		NoRepoCheck: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings when pid matches, got: %v", warnings)
	}
}
