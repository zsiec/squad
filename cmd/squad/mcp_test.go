package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCP_ListsAllTools(t *testing.T) {
	env := newTestEnv(t)

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d:\n%s", len(lines), out.String())
	}
	var toolsResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &toolsResp); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	want := []string{
		"squad_register", "squad_whoami", "squad_next", "squad_claim",
		"squad_release", "squad_done", "squad_blocked", "squad_say",
		"squad_ask", "squad_tick", "squad_progress", "squad_review_request",
		"squad_list_items", "squad_get_item",
		"squad_attest", "squad_attestations",
		"squad_learning_propose", "squad_learning_list", "squad_learning_approve", "squad_learning_reject",
		"squad_learning_agents_md_suggest", "squad_learning_agents_md_approve", "squad_learning_agents_md_reject",
	}
	have := map[string]bool{}
	for _, tt := range toolsResp.Result.Tools {
		have[tt.Name] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing tool: %s", w)
		}
	}
}

func TestMCP_ClaimRoundTrip(t *testing.T) {
	env := newTestEnv(t)

	mustWriteItem(t, env.Root, "BUG-001", "broken")

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_claim","arguments":{"item_id":"BUG-001","intent":"fix it","agent_id":"agent-mcp"}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines", len(lines))
	}
	var resp struct {
		Result struct {
			StructuredContent struct {
				ItemID  string `json:"item_id"`
				AgentID string `json:"agent_id"`
			} `json:"structuredContent"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v (line: %s)", resp.Error, lines[1])
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %s", lines[1])
	}
	if resp.Result.StructuredContent.ItemID != "BUG-001" {
		t.Errorf("item_id: %q", resp.Result.StructuredContent.ItemID)
	}
}

func mustWriteItem(t *testing.T, repoRoot, id, title string) {
	t.Helper()
	dir := filepath.Join(repoRoot, ".squad", "items")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\n" +
		"id: " + id + "\n" +
		"title: " + title + "\n" +
		"status: ready\n" +
		"created: 2026-04-25\n" +
		"updated: 2026-04-25\n" +
		"---\n\n## Problem\n" + title + "\n"
	path := filepath.Join(dir, id+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
