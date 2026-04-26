package main

import (
	"context"
	"errors"
	"testing"
)

func TestProgress_PureRecordsHolderProgress(t *testing.T) {
	f := newChatFixture(t)
	f.insertClaim(t, "BUG-9")
	res, err := Progress(context.Background(), ProgressArgs{
		DB:      f.db,
		RepoID:  f.repoID,
		Chat:    f.chat,
		AgentID: f.agentID,
		ItemID:  "BUG-9",
		Pct:     42,
		Note:    "halfway",
	})
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}
	if res == nil || res.ItemID != "BUG-9" || res.Pct != 42 {
		t.Fatalf("unexpected result: %+v", res)
	}
	var pct int
	var note string
	_ = f.db.QueryRow(`SELECT pct, note FROM progress WHERE item_id='BUG-9'`).Scan(&pct, &note)
	if pct != 42 || note != "halfway" {
		t.Fatalf("pct=%d note=%q", pct, note)
	}
}

func TestProgress_PureRejectsUnclaimed(t *testing.T) {
	f := newChatFixture(t)
	_, err := Progress(context.Background(), ProgressArgs{
		DB:      f.db,
		RepoID:  f.repoID,
		Chat:    f.chat,
		AgentID: f.agentID,
		ItemID:  "BUG-NEVER",
		Pct:     50,
	})
	if !errors.Is(err, ErrNotClaimed) {
		t.Fatalf("err=%v want ErrNotClaimed", err)
	}
}

func TestProgress_PureRejectsNonHolder(t *testing.T) {
	f := newChatFixture(t)
	if _, err := f.db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, 'agent-other', 0, 0, '', 0)
	`, f.repoID, "BUG-9"); err != nil {
		t.Fatal(err)
	}
	_, err := Progress(context.Background(), ProgressArgs{
		DB:      f.db,
		RepoID:  f.repoID,
		Chat:    f.chat,
		AgentID: f.agentID,
		ItemID:  "BUG-9",
		Pct:     50,
	})
	if !errors.Is(err, ErrNotYours) {
		t.Fatalf("err=%v want ErrNotYours", err)
	}
}
