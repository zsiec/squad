package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// seedAgentCapabilities writes the agents row with the given capability set
// so NextItem's filter has something to intersect against.
func seedAgentCapabilities(t *testing.T, env *testEnv, agentID, capsJSON string) {
	t.Helper()
	if _, err := env.DB.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status, capabilities)
		VALUES (?, ?, ?, '', 0, 0, 0, 'active', ?)
		ON CONFLICT(id) DO UPDATE SET capabilities = excluded.capabilities
	`, agentID, env.RepoID, agentID, capsJSON); err != nil {
		t.Fatalf("seed agent capabilities: %v", err)
	}
}

func writeItemWithCaps(t *testing.T, dir, id, title, prio, capsYAML string) {
	t.Helper()
	body := "---\nid: " + id + "\ntitle: " + title + "\ntype: feature\npriority: " + prio +
		"\nstatus: open\nestimate: 1h\n"
	if capsYAML != "" {
		body += "requires_capability: " + capsYAML + "\n"
	}
	body += "---\n"
	if err := os.WriteFile(filepath.Join(dir, id+"-x.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNextItem_FiltersByCapabilitySubset(t *testing.T) {
	env := newTestEnv(t)
	writeItemWithCaps(t, env.ItemsDir, "FEAT-101", "needs go", "P1", "[go]")
	writeItemWithCaps(t, env.ItemsDir, "FEAT-102", "needs go and sql", "P1", "[go, sql]")
	writeItemWithCaps(t, env.ItemsDir, "FEAT-103", "needs frontend", "P1", "[frontend]")
	writeItemWithCaps(t, env.ItemsDir, "FEAT-104", "no capability requirement", "P1", "")

	seedAgentCapabilities(t, env, env.AgentID, `["go","sql"]`)

	got, err := NextItem(context.Background(), NextArgs{
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
	})
	if err != nil {
		t.Fatalf("NextItem: %v", err)
	}
	gotIDs := map[string]bool{}
	for _, it := range got.Items {
		gotIDs[it.ID] = true
	}
	if !gotIDs["FEAT-101"] {
		t.Errorf("agent {go,sql} should see FEAT-101 (needs {go}); items=%v", gotIDs)
	}
	if !gotIDs["FEAT-102"] {
		t.Errorf("agent {go,sql} should see FEAT-102 (needs {go,sql}); items=%v", gotIDs)
	}
	if !gotIDs["FEAT-104"] {
		t.Errorf("agent {go,sql} should see FEAT-104 (empty req); items=%v", gotIDs)
	}
	if gotIDs["FEAT-103"] {
		t.Errorf("agent {go,sql} should NOT see FEAT-103 (needs {frontend}); items=%v", gotIDs)
	}
}

func TestNextItem_DisjointCapabilitiesHidesAllTagged(t *testing.T) {
	env := newTestEnv(t)
	writeItemWithCaps(t, env.ItemsDir, "FEAT-201", "needs go", "P1", "[go]")
	writeItemWithCaps(t, env.ItemsDir, "FEAT-202", "needs frontend", "P1", "[frontend]")
	writeItemWithCaps(t, env.ItemsDir, "FEAT-203", "no req", "P1", "")

	seedAgentCapabilities(t, env, env.AgentID, `["design"]`)

	got, err := NextItem(context.Background(), NextArgs{
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
	})
	if err != nil {
		t.Fatalf("NextItem: %v", err)
	}
	gotIDs := map[string]bool{}
	for _, it := range got.Items {
		gotIDs[it.ID] = true
	}
	if !gotIDs["FEAT-203"] {
		t.Errorf("agent {design} should still see FEAT-203 (empty req); items=%v", gotIDs)
	}
	if gotIDs["FEAT-201"] || gotIDs["FEAT-202"] {
		t.Errorf("disjoint agent should not see tagged items; got=%v", gotIDs)
	}
}

func TestNextItem_AllBypassesFilter(t *testing.T) {
	env := newTestEnv(t)
	writeItemWithCaps(t, env.ItemsDir, "FEAT-301", "needs frontend", "P1", "[frontend]")
	writeItemWithCaps(t, env.ItemsDir, "FEAT-302", "no req", "P1", "")

	seedAgentCapabilities(t, env, env.AgentID, `["go"]`)

	got, err := NextItem(context.Background(), NextArgs{
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		All: true,
	})
	if err != nil {
		t.Fatalf("NextItem --all: %v", err)
	}
	gotIDs := map[string]bool{}
	for _, it := range got.Items {
		gotIDs[it.ID] = true
	}
	if !gotIDs["FEAT-301"] || !gotIDs["FEAT-302"] {
		t.Errorf("--all should return both tagged and untagged; got=%v", gotIDs)
	}
}

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
