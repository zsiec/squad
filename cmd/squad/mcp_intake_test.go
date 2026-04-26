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

type mcpCallResponse struct {
	Result struct {
		StructuredContent map[string]any `json:"structuredContent"`
		IsError           bool           `json:"isError"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func callMCPTool(t *testing.T, env *testEnv, name string, args string) mcpCallResponse {
	t.Helper()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines:\n%s", len(lines), out.String())
	}
	var resp mcpCallResponse
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, lines[1])
	}
	return resp
}

func TestMCPSquadNew_CapturesItem(t *testing.T) {
	env := newTestEnv(t)

	resp := callMCPTool(t, env, "squad_new",
		`{"type":"feat","title":"investigate the flaky test we have"}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %+v", resp.Result.StructuredContent)
	}
	out := resp.Result.StructuredContent
	if got := out["status"]; got != "captured" {
		t.Fatalf("status=%v want captured", got)
	}
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("id empty: %+v", out)
	}
	path, _ := out["path"].(string)
	if path == "" {
		t.Fatalf("path empty: %+v", out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not on disk: %v", err)
	}

	var dbStatus, dbCapturedBy string
	if err := env.DB.QueryRow(
		`SELECT status, COALESCE(captured_by,'') FROM items WHERE repo_id=? AND item_id=?`,
		env.RepoID, id,
	).Scan(&dbStatus, &dbCapturedBy); err != nil {
		t.Fatalf("query items row: %v", err)
	}
	if dbStatus != "captured" {
		t.Fatalf("db status=%q want captured", dbStatus)
	}
	if dbCapturedBy == "" {
		t.Fatalf("db captured_by empty — handler should stamp identity.AgentID()")
	}
	if dbCapturedBy != env.AgentID {
		t.Fatalf("db captured_by=%q want %q", dbCapturedBy, env.AgentID)
	}
}

func TestMCPSquadNew_ReadyFlagCreatesOpen(t *testing.T) {
	env := newTestEnv(t)

	resp := callMCPTool(t, env, "squad_new",
		`{"type":"feat","title":"ship it","ready":true}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %+v", resp.Result.StructuredContent)
	}
	out := resp.Result.StructuredContent
	if got := out["status"]; got != "open" {
		t.Fatalf("status=%v want open", got)
	}
}

func TestMCPSquadNew_RejectsUnknownType(t *testing.T) {
	env := newTestEnv(t)

	resp := callMCPTool(t, env, "squad_new",
		`{"type":"nope","title":"x"}`)
	if resp.Error == nil {
		t.Fatalf("want rpc error for unknown type, got result: %+v", resp.Result.StructuredContent)
	}
	if !strings.Contains(resp.Error.Message, "nope") {
		t.Errorf("error message %q should mention the rejected type", resp.Error.Message)
	}
}

func TestMCPSquadNew_PersistsAreaFromArgs(t *testing.T) {
	env := newTestEnv(t)

	resp := callMCPTool(t, env, "squad_new",
		`{"type":"feat","title":"x","area":"auth","priority":"P1","risk":"medium"}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	out := resp.Result.StructuredContent
	path, _ := out["path"].(string)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	for _, want := range []string{"area: auth", "priority: P1", "risk: medium"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("item missing %q:\n%s", want, body)
		}
	}
	if !strings.HasPrefix(filepath.Base(path), "FEAT-") {
		t.Errorf("path %q should start with FEAT-", filepath.Base(path))
	}
}
