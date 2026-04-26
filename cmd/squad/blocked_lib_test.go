package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/claims"
)

func TestBlocked_PureMarksClaimedItemBlocked(t *testing.T) {
	env := newTestEnv(t)
	itemPath := filepath.Join(env.ItemsDir, "BUG-100-thing.md")
	if err := os.WriteFile(itemPath,
		[]byte("---\nid: BUG-100\ntitle: thing\ntype: bug\npriority: P1\nstatus: open\nestimate: 1h\n---\n\n## Acceptance criteria\n- [ ] x\n"),
		0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, '', 0)
	`, env.RepoID, "BUG-100", env.AgentID, 1000, 1000); err != nil {
		t.Fatalf("seed claim: %v", err)
	}

	res, err := Blocked(context.Background(), BlockedArgs{
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
		ItemID:   "BUG-100",
		Reason:   "waiting for upstream fix",
		ItemsDir: env.ItemsDir,
	})
	if err != nil {
		t.Fatalf("Blocked: %v", err)
	}
	if res == nil || res.ItemID != "BUG-100" || res.Reason == "" || res.AtUnix == 0 {
		t.Fatalf("unexpected result: %+v", res)
	}

	body, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status: blocked") {
		t.Errorf("item file not rewritten: %s", body)
	}
	if !strings.Contains(string(body), "## Blocker") {
		t.Errorf("blocker section missing: %s", body)
	}
}

func TestBlocked_PureRejectsUnclaimed(t *testing.T) {
	env := newTestEnv(t)
	_, err := Blocked(context.Background(), BlockedArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		ItemID:  "GHOST-1",
		Reason:  "no",
	})
	if !errors.Is(err, claims.ErrNotClaimed) {
		t.Fatalf("err=%v want ErrNotClaimed", err)
	}
}
