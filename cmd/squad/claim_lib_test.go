package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeMinimalItem(t *testing.T, dir, id string) {
	t.Helper()
	body := "---\nid: " + id + "\ntitle: t\ntype: bug\npriority: P1\nstatus: open\nestimate: 1h\n---\n\n## Acceptance criteria\n- [ ] x\n"
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaim_PureClaimsItem(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-200")

	res, err := Claim(context.Background(), ClaimArgs{
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
		ItemID:   "BUG-200",
		Intent:   "fix it",
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
	})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if res == nil || res.ItemID != "BUG-200" || res.AgentID != env.AgentID {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestClaim_PureRejectsAlreadyClaimed(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-201")
	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, 'agent-other', 1000, 1000, '', 0)
	`, env.RepoID, "BUG-201"); err != nil {
		t.Fatal(err)
	}

	_, err := Claim(context.Background(), ClaimArgs{
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
		ItemID:   "BUG-201",
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
	})
	var held *ClaimHeldError
	if !errors.As(err, &held) {
		t.Fatalf("err=%v want *ClaimHeldError", err)
	}
	if held.Holder != "agent-other" || held.ItemID != "BUG-201" {
		t.Fatalf("unexpected fields: %+v", held)
	}
}

func TestClaim_PureRejectsConcurrencyCap(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-202")
	writeMinimalItem(t, env.ItemsDir, "BUG-203")

	if _, err := Claim(context.Background(), ClaimArgs{
		DB:             env.DB,
		RepoID:         env.RepoID,
		AgentID:        env.AgentID,
		ItemID:         "BUG-202",
		ItemsDir:       env.ItemsDir,
		DoneDir:        env.DoneDir,
		ConcurrencyCap: 1,
	}); err != nil {
		t.Fatalf("first Claim: %v", err)
	}

	_, err := Claim(context.Background(), ClaimArgs{
		DB:             env.DB,
		RepoID:         env.RepoID,
		AgentID:        env.AgentID,
		ItemID:         "BUG-203",
		ItemsDir:       env.ItemsDir,
		DoneDir:        env.DoneDir,
		ConcurrencyCap: 1,
	})
	var cap *ConcurrencyExceededError
	if !errors.As(err, &cap) {
		t.Fatalf("err=%v want *ConcurrencyExceededError", err)
	}
	if cap.Held != 1 || cap.Cap != 1 || cap.AgentID != env.AgentID {
		t.Fatalf("unexpected fields: %+v", cap)
	}
}
