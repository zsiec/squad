package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

const arPlaceholderItem = `---
id: %s
title: a sufficiently long title for dor pass
type: feature
priority: P2
area: auth
status: %s
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
captured_by: agent-test
captured_at: 1700000000
---

## Acceptance criteria
- [ ] Specific, testable thing 1
- [ ] Specific, testable thing 2
`

const arDoRCleanBody = "## Problem\n\nreal prose.\n\n## Context\n\nreal context.\n\n## Acceptance criteria\n- [ ] real, testable acceptance criterion\n"

func writeAutoRefineFixture(t *testing.T, dir, id, status string) string {
	t.Helper()
	body := fmt.Sprintf(arPlaceholderItem, id, status)
	path := filepath.Join(dir, id+"-x.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mcpCallAutoRefine(t *testing.T, env *testEnv, itemID, newBody string) []string {
	t.Helper()
	args, err := json.Marshal(map[string]any{"item_id": itemID, "new_body": newBody})
	if err != nil {
		t.Fatal(err)
	}
	callReq, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{
			"name":      "squad_auto_refine_apply",
			"arguments": json.RawMessage(args),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" + string(callReq) + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	return strings.Split(strings.TrimSpace(out.String()), "\n")
}

type autoRefineRPCResp struct {
	Result struct {
		StructuredContent map[string]any `json:"structuredContent"`
		IsError           bool           `json:"isError"`
		Content           []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeAutoRefineResp(t *testing.T, line string) autoRefineRPCResp {
	t.Helper()
	var resp autoRefineRPCResp
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, line)
	}
	return resp
}

func TestMCP_AutoRefineApply_HappyPath(t *testing.T) {
	env := newTestEnv(t)
	writeAutoRefineFixture(t, env.ItemsDir, "BUG-700", "captured")

	lines := mcpCallAutoRefine(t, env, "BUG-700", arDoRCleanBody)
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 response lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	resp := decodeAutoRefineResp(t, lines[1])
	if resp.Error != nil {
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %s", lines[1])
	}
	sc := resp.Result.StructuredContent
	if ok, _ := sc["ok"].(bool); !ok {
		t.Errorf("ok = %v, want true", sc["ok"])
	}
	if id, _ := sc["item_id"].(string); id != "BUG-700" {
		t.Errorf("item_id = %v, want BUG-700", sc["item_id"])
	}
	at, _ := sc["auto_refined_at"].(float64)
	if at <= 0 {
		t.Errorf("auto_refined_at = %v, want > 0", sc["auto_refined_at"])
	}
	for _, k := range []string{"new_body", "body", "path", "file"} {
		if _, has := sc[k]; has {
			t.Errorf("structuredContent must not include %q", k)
		}
	}

	on, err := items.Parse(filepath.Join(env.ItemsDir, "BUG-700-x.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if on.AutoRefinedBy != "claude" {
		t.Errorf("auto_refined_by = %q, want claude", on.AutoRefinedBy)
	}
	if on.AutoRefinedAt != int64(at) {
		t.Errorf("file auto_refined_at=%d response=%d (mismatch)", on.AutoRefinedAt, int64(at))
	}
	if !strings.Contains(on.Body, "real, testable acceptance criterion") {
		t.Errorf("body not rewritten:\n%s", on.Body)
	}
	if on.Status != "captured" {
		t.Errorf("status flipped to %q; auto-refine must keep captured", on.Status)
	}
}

func TestMCP_AutoRefineApply_RejectsDoRFailingBody(t *testing.T) {
	env := newTestEnv(t)
	writeAutoRefineFixture(t, env.ItemsDir, "BUG-701", "captured")

	failing := "## Acceptance criteria\n- [ ] " + items.TemplateACPlaceholders[0] + "\n- [ ] " + items.TemplateACPlaceholders[1] + "\n"
	lines := mcpCallAutoRefine(t, env, "BUG-701", failing)
	resp := decodeAutoRefineResp(t, lines[1])

	msg := errOrToolErrMessage(resp)
	if msg == "" {
		t.Fatalf("expected failure response, got success: %s", lines[1])
	}
	if !strings.Contains(msg, "template-not-placeholder") {
		t.Errorf("error must name failing rule template-not-placeholder; got %q", msg)
	}

	raw, err := os.ReadFile(filepath.Join(env.ItemsDir, "BUG-701-x.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("Specific, testable thing 1")) {
		t.Errorf("file body changed despite refusal:\n%s", raw)
	}
}

func TestMCP_AutoRefineApply_RejectsNonCapturedStatus(t *testing.T) {
	env := newTestEnv(t)
	writeAutoRefineFixture(t, env.ItemsDir, "BUG-702", "open")

	lines := mcpCallAutoRefine(t, env, "BUG-702", arDoRCleanBody)
	resp := decodeAutoRefineResp(t, lines[1])

	msg := errOrToolErrMessage(resp)
	if msg == "" {
		t.Fatalf("expected failure response, got success: %s", lines[1])
	}
	if !strings.Contains(msg, "open") {
		t.Errorf("error must include current status %q; got %q", "open", msg)
	}
}

func errOrToolErrMessage(resp autoRefineRPCResp) string {
	if resp.Error != nil {
		return resp.Error.Message
	}
	if resp.Result.IsError {
		var b strings.Builder
		for _, c := range resp.Result.Content {
			b.WriteString(c.Text)
		}
		return b.String()
	}
	return ""
}
