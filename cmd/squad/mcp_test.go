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
		"squad_handoff", "squad_knock", "squad_answer",
		"squad_force_release", "squad_reassign", "squad_archive",
		"squad_history", "squad_who", "squad_status",
		"squad_touch", "squad_untouch", "squad_touches_list_others",
		"squad_pr_link", "squad_pr_close",
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

func TestMCP_NextOnEmptyQueueReturnsEmpty(t *testing.T) {
	env := newTestEnv(t)
	// setupSquadRepo seeds EXAMPLE-001 as a tutorial item; remove it so the
	// queue is genuinely empty for this regression test.
	_ = os.Remove(filepath.Join(env.ItemsDir, "EXAMPLE-001-try-the-loop.md"))

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_next","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines:\n%s", len(lines), out.String())
	}
	var resp struct {
		Result struct {
			StructuredContent struct {
				Items []map[string]any `json:"items"`
				Total int              `json:"total"`
			} `json:"structuredContent"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, lines[1])
	}
	if resp.Error != nil {
		t.Fatalf("rpc error on empty queue: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error on empty queue: %s", lines[1])
	}
	if resp.Result.StructuredContent.Total != 0 {
		t.Fatalf("total = %d, want 0", resp.Result.StructuredContent.Total)
	}
	if len(resp.Result.StructuredContent.Items) != 0 {
		t.Fatalf("items len = %d, want 0", len(resp.Result.StructuredContent.Items))
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

// TestMCP_StatusRoundTrip is a smoke test for the new coordination tools:
// it calls squad_status (a read-only tool from the registerCoordinationTools
// batch) and confirms the structured response shape decodes cleanly.
func TestMCP_StatusRoundTrip(t *testing.T) {
	env := newTestEnv(t)

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_status","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines:\n%s", len(lines), out.String())
	}
	var resp struct {
		Result struct {
			StructuredContent struct {
				Claimed int `json:"claimed"`
				Ready   int `json:"ready"`
				Blocked int `json:"blocked"`
				Done    int `json:"done"`
			} `json:"structuredContent"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, lines[1])
	}
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %s", lines[1])
	}
	// EXAMPLE-001 ships as a ready item by default, so Ready should be ≥ 1.
	if resp.Result.StructuredContent.Ready < 1 {
		t.Errorf("expected ≥1 ready item; got %+v", resp.Result.StructuredContent)
	}
}
