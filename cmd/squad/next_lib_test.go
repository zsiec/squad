package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNextItem_PureReturnsPriorityOrdered(t *testing.T) {
	env := newTestEnv(t)
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(env.ItemsDir, name),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("BUG-001-low.md",
		"---\nid: BUG-001\ntitle: low\ntype: bug\npriority: P2\nstatus: open\nestimate: 1h\n---\n")
	write("FEAT-002-high.md",
		"---\nid: FEAT-002\ntitle: high\ntype: feature\npriority: P0\nstatus: open\nestimate: 2h\n---\n")

	got, err := NextItem(context.Background(), NextArgs{
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
	})
	if err != nil {
		t.Fatalf("NextItem: %v", err)
	}
	if len(got.Items) < 2 {
		t.Fatalf("expected >=2 items, got %+v", got.Items)
	}
	if got.Items[0].ID != "FEAT-002" {
		t.Fatalf("first=%q want FEAT-002", got.Items[0].ID)
	}
}
