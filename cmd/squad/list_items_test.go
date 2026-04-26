package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeItem(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListItems_FiltersAndSort(t *testing.T) {
	itemsDir := t.TempDir()
	doneDir := t.TempDir()
	writeItem(t, itemsDir, "FEAT-001.md",
		"---\nid: FEAT-001\ntitle: a\ntype: feat\npriority: P2\nstatus: ready\ncreated: 2026-04-25\n---\n")
	writeItem(t, itemsDir, "BUG-002.md",
		"---\nid: BUG-002\ntitle: b\ntype: bug\npriority: P0\nstatus: ready\ncreated: 2026-04-24\n---\n")
	writeItem(t, itemsDir, "FEAT-003.md",
		"---\nid: FEAT-003\ntitle: c\ntype: feat\npriority: P1\nstatus: ready\ncreated: 2026-04-23\n---\n")
	writeItem(t, doneDir, "FEAT-004.md",
		"---\nid: FEAT-004\ntitle: d\ntype: feat\npriority: P1\nstatus: done\ncreated: 2026-04-22\n---\n")

	rows, err := ListItems(context.Background(), ListItemsArgs{
		ItemsDir: itemsDir,
		DoneDir:  doneDir,
	})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("len=%d want 4 rows: %+v", len(rows), rows)
	}
	if rows[0].ID != "BUG-002" {
		t.Fatalf("first row %q want BUG-002 (P0 sorts first)", rows[0].ID)
	}

	rows, err = ListItems(context.Background(), ListItemsArgs{
		ItemsDir: itemsDir,
		DoneDir:  doneDir,
		Type:     "bug",
	})
	if err != nil {
		t.Fatalf("ListItems(type=bug): %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "BUG-002" {
		t.Fatalf("type filter wrong: %+v", rows)
	}

	rows, err = ListItems(context.Background(), ListItemsArgs{
		ItemsDir: itemsDir,
		DoneDir:  doneDir,
		Status:   "done",
	})
	if err != nil {
		t.Fatalf("ListItems(status=done): %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "FEAT-004" {
		t.Fatalf("status=done filter wrong: %+v", rows)
	}

	rows, err = ListItems(context.Background(), ListItemsArgs{
		ItemsDir: itemsDir,
		DoneDir:  doneDir,
		Priority: "P1",
	})
	if err != nil {
		t.Fatalf("ListItems(priority=P1): %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("priority=P1 expected 2 rows: %+v", rows)
	}

	rows, err = ListItems(context.Background(), ListItemsArgs{
		ItemsDir: itemsDir,
		DoneDir:  doneDir,
		Limit:    2,
	})
	if err != nil {
		t.Fatalf("ListItems(limit=2): %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("limit=2 wrong: %+v", rows)
	}
}

func TestListItems_AgentFilterFiltersByClaim(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-501")
	writeMinimalItem(t, env.ItemsDir, "BUG-502")
	writeMinimalItem(t, env.ItemsDir, "BUG-503")

	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, 1000, 1000, '', 0)
	`, env.RepoID, "BUG-501", "agent-alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, 1000, 1000, '', 0)
	`, env.RepoID, "BUG-502", "agent-bob"); err != nil {
		t.Fatal(err)
	}

	rows, err := ListItems(context.Background(), ListItemsArgs{
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
		DB:       env.DB,
		RepoID:   env.RepoID,
		Agent:    "agent-alice",
	})
	if err != nil {
		t.Fatalf("ListItems(agent=agent-alice): %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "BUG-501" {
		t.Fatalf("agent filter wrong: %+v", rows)
	}
	if rows[0].Agent != "agent-alice" {
		t.Fatalf("Agent field not populated: %+v", rows[0])
	}
}

func TestListItems_AgentFilterRequiresDB(t *testing.T) {
	itemsDir := t.TempDir()
	_, err := ListItems(context.Background(), ListItemsArgs{
		ItemsDir: itemsDir,
		Agent:    "agent-x",
	})
	if err == nil {
		t.Fatal("want error when Agent set without DB, got nil")
	}
}

func TestListItems_DefaultLimitCappedAt200(t *testing.T) {
	itemsDir := t.TempDir()
	rows, err := ListItems(context.Background(), ListItemsArgs{
		ItemsDir: itemsDir,
		Limit:    500,
	})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("empty dir produced %d rows", len(rows))
	}
}
