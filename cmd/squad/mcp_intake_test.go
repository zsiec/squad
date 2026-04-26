package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/items"
)

const mcpAcceptItemReady = `---
id: FEAT-001
title: Wire up the new accept verb plumbing for inbox
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] does the thing
`

const mcpAcceptItemDoRFails = `---
id: FEAT-002
title: tiny
type: feature
priority: P1
area: <fill-in>
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
no checkboxes here
`

func mcpPersistFixture(t *testing.T, env *testEnv, fileName string) {
	t.Helper()
	path := filepath.Join(env.Root, ".squad", "items", fileName)
	parsed, err := items.Parse(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	parsed.Path = path
	if err := items.Persist(context.Background(), env.DB, env.RepoID, parsed, false); err != nil {
		t.Fatalf("persist %s: %v", fileName, err)
	}
}

func mcpStringSlice(t *testing.T, raw any, key string) []string {
	t.Helper()
	v, ok := raw.([]any)
	if !ok {
		if raw == nil {
			return nil
		}
		t.Fatalf("%s: want []any, got %T (%v)", key, raw, raw)
	}
	out := make([]string, 0, len(v))
	for i, e := range v {
		s, ok := e.(string)
		if !ok {
			t.Fatalf("%s[%d]: want string, got %T", key, i, e)
		}
		out = append(out, s)
	}
	return out
}

func mcpObjSlice(t *testing.T, raw any, key string) []map[string]any {
	t.Helper()
	v, ok := raw.([]any)
	if !ok {
		if raw == nil {
			return nil
		}
		t.Fatalf("%s: want []any, got %T (%v)", key, raw, raw)
	}
	out := make([]map[string]any, 0, len(v))
	for i, e := range v {
		m, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("%s[%d]: want map[string]any, got %T", key, i, e)
		}
		out = append(out, m)
	}
	return out
}

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

func TestMCPSquadAccept_HappyPath(t *testing.T) {
	env := newTestEnv(t)
	writeItemFile(t, env.Root, "FEAT-001-ready.md", mcpAcceptItemReady)
	writeItemFile(t, env.Root, "FEAT-101-also.md", strings.Replace(mcpAcceptItemReady, "FEAT-001", "FEAT-101", 1))
	mcpPersistFixture(t, env, "FEAT-001-ready.md")
	mcpPersistFixture(t, env, "FEAT-101-also.md")

	resp := callMCPTool(t, env, "squad_accept",
		`{"ids":["FEAT-001","FEAT-101"]}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %+v", resp.Result.StructuredContent)
	}
	out := resp.Result.StructuredContent
	accepted := mcpStringSlice(t, out["accepted"], "accepted")
	if len(accepted) != 2 || accepted[0] != "FEAT-001" || accepted[1] != "FEAT-101" {
		t.Fatalf("accepted=%v want [FEAT-001 FEAT-101]", accepted)
	}
	rejected := mcpObjSlice(t, out["rejected"], "rejected")
	if len(rejected) != 0 {
		t.Fatalf("rejected len=%d want 0: %+v", len(rejected), rejected)
	}

	for _, id := range []string{"FEAT-001", "FEAT-101"} {
		var status string
		if err := env.DB.QueryRow(
			`SELECT status FROM items WHERE repo_id=? AND item_id=?`,
			env.RepoID, id,
		).Scan(&status); err != nil {
			t.Fatalf("query %s: %v", id, err)
		}
		if status != "open" {
			t.Errorf("%s status=%q want open", id, status)
		}
	}
}

func TestMCPSquadAccept_MixedDoR(t *testing.T) {
	env := newTestEnv(t)
	writeItemFile(t, env.Root, "FEAT-001-ready.md", mcpAcceptItemReady)
	writeItemFile(t, env.Root, "FEAT-002-bad.md", mcpAcceptItemDoRFails)
	mcpPersistFixture(t, env, "FEAT-001-ready.md")
	mcpPersistFixture(t, env, "FEAT-002-bad.md")

	resp := callMCPTool(t, env, "squad_accept",
		`{"ids":["FEAT-001","FEAT-002"]}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	out := resp.Result.StructuredContent
	accepted := mcpStringSlice(t, out["accepted"], "accepted")
	if len(accepted) != 1 || accepted[0] != "FEAT-001" {
		t.Fatalf("accepted=%v want [FEAT-001]", accepted)
	}
	rejected := mcpObjSlice(t, out["rejected"], "rejected")
	if len(rejected) != 1 {
		t.Fatalf("rejected len=%d want 1: %+v", len(rejected), rejected)
	}
	row := rejected[0]
	if row["id"] != "FEAT-002" {
		t.Fatalf("rejected[0].id=%v want FEAT-002", row["id"])
	}
	violations, ok := row["violations"].([]any)
	if !ok || len(violations) == 0 {
		t.Fatalf("rejected[0].violations missing/empty: %+v", row)
	}
	first, ok := violations[0].(map[string]any)
	if !ok {
		t.Fatalf("violation[0] type %T", violations[0])
	}
	if first["rule"] == nil || first["message"] == nil {
		t.Fatalf("violation[0] missing rule/message: %+v", first)
	}
}

func mcpSeedClaim(t *testing.T, env *testEnv, itemID, agentID string) {
	t.Helper()
	now := time.Now().Unix()
	if _, err := env.DB.Exec(
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		env.RepoID, itemID, agentID, now, now, "", 0,
	); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
}

func TestMCPSquadReject_HappyPath(t *testing.T) {
	env := newTestEnv(t)
	writeItemFile(t, env.Root, "FEAT-001-ready.md", mcpAcceptItemReady)
	writeItemFile(t, env.Root, "FEAT-101-also.md", strings.Replace(mcpAcceptItemReady, "FEAT-001", "FEAT-101", 1))
	mcpPersistFixture(t, env, "FEAT-001-ready.md")
	mcpPersistFixture(t, env, "FEAT-101-also.md")

	resp := callMCPTool(t, env, "squad_reject",
		`{"ids":["FEAT-001","FEAT-101"],"reason":"out of scope"}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %+v", resp.Result.StructuredContent)
	}
	out := resp.Result.StructuredContent
	deleted := mcpStringSlice(t, out["deleted"], "deleted")
	if len(deleted) != 2 || deleted[0] != "FEAT-001" || deleted[1] != "FEAT-101" {
		t.Fatalf("deleted=%v want [FEAT-001 FEAT-101]", deleted)
	}
	refused := mcpObjSlice(t, out["refused"], "refused")
	if len(refused) != 0 {
		t.Fatalf("refused len=%d want 0: %+v", len(refused), refused)
	}

	for _, fn := range []string{"FEAT-001-ready.md", "FEAT-101-also.md"} {
		if _, err := os.Stat(filepath.Join(env.Root, ".squad", "items", fn)); !os.IsNotExist(err) {
			t.Errorf("%s still exists after reject (err=%v)", fn, err)
		}
	}

	logBytes, err := os.ReadFile(filepath.Join(env.Root, ".squad", "rejected.log"))
	if err != nil {
		t.Fatalf("read rejected.log: %v", err)
	}
	for _, want := range []string{"FEAT-001", "FEAT-101", "out of scope"} {
		if !strings.Contains(string(logBytes), want) {
			t.Errorf("rejected.log missing %q:\n%s", want, logBytes)
		}
	}
}

func TestMCPSquadReject_ReasonRequired(t *testing.T) {
	env := newTestEnv(t)
	writeItemFile(t, env.Root, "FEAT-001-ready.md", mcpAcceptItemReady)
	mcpPersistFixture(t, env, "FEAT-001-ready.md")

	resp := callMCPTool(t, env, "squad_reject",
		`{"ids":["FEAT-001"],"reason":""}`)
	if resp.Error == nil {
		t.Fatalf("want rpc error for empty reason, got result: %+v", resp.Result.StructuredContent)
	}
	if !strings.Contains(strings.ToLower(resp.Error.Message), "reason") {
		t.Errorf("error message %q should mention reason", resp.Error.Message)
	}
}

func TestMCPSquadReject_ClaimedItemRefused(t *testing.T) {
	env := newTestEnv(t)
	writeItemFile(t, env.Root, "FEAT-001-ready.md", mcpAcceptItemReady)
	writeItemFile(t, env.Root, "FEAT-002-claimed.md", strings.Replace(mcpAcceptItemReady, "FEAT-001", "FEAT-002", 1))
	mcpPersistFixture(t, env, "FEAT-001-ready.md")
	mcpPersistFixture(t, env, "FEAT-002-claimed.md")
	mcpSeedClaim(t, env, "FEAT-002", "another-agent")

	resp := callMCPTool(t, env, "squad_reject",
		`{"ids":["FEAT-001","FEAT-002"],"reason":"duplicate"}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	out := resp.Result.StructuredContent
	deleted := mcpStringSlice(t, out["deleted"], "deleted")
	if len(deleted) != 1 || deleted[0] != "FEAT-001" {
		t.Fatalf("deleted=%v want [FEAT-001]", deleted)
	}
	refused := mcpObjSlice(t, out["refused"], "refused")
	if len(refused) != 1 {
		t.Fatalf("refused len=%d want 1: %+v", len(refused), refused)
	}
	row := refused[0]
	if row["id"] != "FEAT-002" {
		t.Fatalf("refused[0].id=%v want FEAT-002", row["id"])
	}
	errMsg, _ := row["error"].(string)
	if !strings.Contains(strings.ToLower(errMsg), "claimed") {
		t.Fatalf("refused[0].error=%q want substring 'claimed'", errMsg)
	}
}

func TestMCPSquadAccept_UnknownIDIsRejected(t *testing.T) {
	env := newTestEnv(t)

	resp := callMCPTool(t, env, "squad_accept",
		`{"ids":["FEAT-999"]}`)
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	out := resp.Result.StructuredContent
	accepted := mcpStringSlice(t, out["accepted"], "accepted")
	if len(accepted) != 0 {
		t.Fatalf("accepted=%v want empty", accepted)
	}
	rejected := mcpObjSlice(t, out["rejected"], "rejected")
	if len(rejected) != 1 {
		t.Fatalf("rejected len=%d want 1: %+v", len(rejected), rejected)
	}
	row := rejected[0]
	if row["id"] != "FEAT-999" {
		t.Fatalf("rejected[0].id=%v want FEAT-999", row["id"])
	}
	errMsg, _ := row["error"].(string)
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("rejected[0].error=%q want substring 'not found'", errMsg)
	}
}
