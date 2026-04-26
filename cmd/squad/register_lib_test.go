package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func TestRegister_PureWritesAgentRow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-pure-1")
	t.Setenv("SQUAD_AGENT", "")

	res, err := Register(context.Background(), RegisterArgs{
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
