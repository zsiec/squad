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
		"squad_doctor",
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

// TestMCP_WhoamiDoesNotLeakOtherRepoClaim guards a cross-repo isolation bug:
// when an agent holds a claim in repo A and runs an MCP whoami from repo B
// (or from a directory with no .squad/ at all), the response must not echo
// the foreign claim. Previously the SELECT filtered by agent_id only and
// happily returned a claim row tied to a different repo_id.
func TestMCP_WhoamiDoesNotLeakOtherRepoClaim(t *testing.T) {
	env := newTestEnv(t)

	// Plant a claim in env.RepoID (repo A).
	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, ?, 0)`,
		env.RepoID, "BUG-foreign", env.AgentID, 1700000000, 1700000000, "foreign work"); err != nil {
		t.Fatalf("plant claim: %v", err)
	}

	// Run MCP with empty repoID/repoRoot — i.e. invoked from a directory
	// with no .squad/. The agent identity still resolves, but no per-repo
	// state should bleed into the response.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_whoami","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, "", "", in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines:\n%s", len(lines), out.String())
	}
	var resp struct {
		Result struct {
			StructuredContent struct {
				AgentID string `json:"id"`
				ItemID  string `json:"item_id"`
				Intent  string `json:"intent"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, lines[1])
	}
	if resp.Result.StructuredContent.AgentID == "" {
		t.Fatalf("expected agent id; got %q", resp.Result.StructuredContent.AgentID)
	}
	if resp.Result.StructuredContent.ItemID != "" {
		t.Errorf("cross-repo claim leak: item_id = %q (must be empty when repo not discovered)",
			resp.Result.StructuredContent.ItemID)
	}
	if resp.Result.StructuredContent.Intent != "" {
		t.Errorf("cross-repo claim leak: intent = %q", resp.Result.StructuredContent.Intent)
	}
}

// TestMCP_WhoamiInDifferentRepoIsolatesClaim plants a claim in repo A and
// invokes MCP with repo B's id; the response must show no claim — the
// caller's repo doesn't hold one.
func TestMCP_WhoamiInDifferentRepoIsolatesClaim(t *testing.T) {
	env := newTestEnv(t)

	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, ?, 0)`,
		env.RepoID, "BUG-foreign", env.AgentID, 1700000000, 1700000000, "foreign work"); err != nil {
		t.Fatalf("plant claim: %v", err)
	}

	otherRepo := "repo-other-not-the-one-with-the-claim"
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_whoami","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, otherRepo, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var resp struct {
		Result struct {
			StructuredContent struct {
				ItemID string `json:"item_id"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, lines[1])
	}
	if resp.Result.StructuredContent.ItemID != "" {
		t.Errorf("cross-repo claim leak: item_id = %q (claim is in %q, not %q)",
			resp.Result.StructuredContent.ItemID, env.RepoID, otherRepo)
	}
}

// TestMCP_ClaimResponseCarriesTipsForHighStakesItem exercises the parity fix:
// MCP-using agents must see the same nudges the cobra CLI prints to stderr.
// A P1 multi-AC item should get all three claim-time tips; a P3 single-AC
// item gets none (the optional field is omitted from JSON).
func TestMCP_ClaimResponseCarriesTipsForHighStakesItem(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")

	mustWriteItemDetailed(t, env.Root, "BUG-100", itemSpec{
		Title: "high-stakes", Type: "bug", Priority: "P1", AC: 3,
	})

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_claim","arguments":{"item_id":"BUG-100","intent":"fix it","agent_id":"agent-mcp-tips"}}}` + "\n")
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
				ItemID string   `json:"item_id"`
				Tips   []string `json:"tips"`
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
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %s", lines[1])
	}
	if resp.Result.StructuredContent.ItemID != "BUG-100" {
		t.Fatalf("item_id = %q, want BUG-100", resp.Result.StructuredContent.ItemID)
	}
	tips := resp.Result.StructuredContent.Tips
	if len(tips) != 3 {
		t.Fatalf("tips len = %d, want 3 (cadence + second-opinion + milestone-target); raw=%s", len(tips), lines[1])
	}
	if !strings.Contains(tips[0], "squad thinking") {
		t.Errorf("tips[0] should be the cadence claim nudge, got %q", tips[0])
	}
	if !strings.Contains(tips[1], "squad ask @") {
		t.Errorf("tips[1] should be the second-opinion nudge, got %q", tips[1])
	}
	if !strings.Contains(tips[2], "3 AC") {
		t.Errorf("tips[2] should be the milestone-target nudge naming the AC total, got %q", tips[2])
	}
}

func TestMCP_ClaimResponseOmitsTipsForLowStakesItem(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")

	// P3 single-AC: cadence still fires, but second-opinion and
	// milestone-target both stay quiet — so the tips array has exactly the
	// claim cadence line.
	mustWriteItemDetailed(t, env.Root, "BUG-101", itemSpec{
		Title: "low-stakes", Type: "bug", Priority: "P3", AC: 1,
	})

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_claim","arguments":{"item_id":"BUG-101","intent":"trivial","agent_id":"agent-mcp-low"}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var resp struct {
		Result struct {
			StructuredContent struct {
				ItemID string   `json:"item_id"`
				Tips   []string `json:"tips"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, lines[1])
	}
	tips := resp.Result.StructuredContent.Tips
	if len(tips) != 1 {
		t.Fatalf("tips len = %d, want 1 (cadence only); raw=%s", len(tips), lines[1])
	}
	if !strings.Contains(tips[0], "squad thinking") {
		t.Errorf("tips[0] should be the cadence claim nudge, got %q", tips[0])
	}
	// Verify the JSON wire actually omits the field when nil — assert on
	// the raw bytes that "tips" appears (because it's populated here),
	// then assert on a separate suppressed call that it does NOT appear.
	if !strings.Contains(lines[1], `"tips"`) {
		t.Errorf("expected tips field in JSON output, got: %s", lines[1])
	}
}

func TestMCP_ClaimResponseOmitsTipsWhenSilenced(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")

	mustWriteItemDetailed(t, env.Root, "BUG-102", itemSpec{
		Title: "silenced", Type: "bug", Priority: "P1", AC: 3,
	})

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_claim","arguments":{"item_id":"BUG-102","intent":"silenced","agent_id":"agent-silenced"}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if strings.Contains(lines[1], `"tips"`) {
		t.Errorf("env=1 should suppress tips entirely (json:omitempty), got: %s", lines[1])
	}
}

func TestMCP_DoneResponseCarriesTipForBugItem(t *testing.T) {
	env := newTestEnv(t)
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")

	mustWriteItemDetailed(t, env.Root, "BUG-110", itemSpec{
		Title: "ship it", Type: "bug", Priority: "P2", AC: 1,
	})
	// Plant a claim so squad_done has something to release.
	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, ?, 0)`,
		env.RepoID, "BUG-110", "agent-mcp-done", 1700000000, 1700000000, "fix"); err != nil {
		t.Fatalf("plant claim: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_done","arguments":{"item_id":"BUG-110","summary":"fixed","agent_id":"agent-mcp-done","force":true}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var resp struct {
		Result struct {
			StructuredContent struct {
				ItemID string   `json:"item_id"`
				Tips   []string `json:"tips"`
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
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %s", lines[1])
	}
	if len(resp.Result.StructuredContent.Tips) != 1 {
		t.Fatalf("tips len = %d, want 1 (bug-typed done nudge); raw=%s",
			len(resp.Result.StructuredContent.Tips), lines[1])
	}
	if !strings.Contains(resp.Result.StructuredContent.Tips[0], "gotcha") {
		t.Errorf("done tip should be the bug-typed gotcha nudge, got %q",
			resp.Result.StructuredContent.Tips[0])
	}
}

type itemSpec struct {
	Title    string
	Type     string
	Priority string
	Risk     string
	AC       int
}

func mustWriteItemDetailed(t *testing.T, repoRoot, id string, spec itemSpec) {
	t.Helper()
	dir := filepath.Join(repoRoot, ".squad", "items")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + id + "\n")
	b.WriteString("title: " + spec.Title + "\n")
	if spec.Type != "" {
		b.WriteString("type: " + spec.Type + "\n")
	}
	if spec.Priority != "" {
		b.WriteString("priority: " + spec.Priority + "\n")
	}
	if spec.Risk != "" {
		b.WriteString("risk: " + spec.Risk + "\n")
	}
	b.WriteString("status: ready\n")
	b.WriteString("created: 2026-04-25\n")
	b.WriteString("updated: 2026-04-25\n")
	b.WriteString("---\n\n## Acceptance criteria\n")
	for i := 0; i < spec.AC; i++ {
		b.WriteString("- [ ] step\n")
	}
	path := filepath.Join(dir, id+".md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
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

// TestMCP_NoRepoReturnsInvalidParams covers the JSON-RPC error mapping for
// the "no .squad/ here" condition. Per the spec, -32602 (Invalid params) is
// the right code: the call's arguments imply a context (a repo) that does
// not exist. Previously every per-repo handler returned the catch-all
// -32603 (Internal error), which is what JSON-RPC reserves for unexpected
// server-side faults — the opposite of a user-input issue.
func TestMCP_NoRepoReturnsInvalidParams(t *testing.T) {
	env := newTestEnv(t)

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_next","arguments":{}}}` + "\n")
	var out bytes.Buffer
	// Call with empty repoID/repoRoot to simulate a non-squad working dir.
	if err := runMCP(context.Background(), env.DB, "", "", in, &out); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines:\n%s", len(lines), out.String())
	}
	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, lines[1])
	}
	if resp.Error == nil {
		t.Fatalf("expected an error on no-repo call; got: %s", lines[1])
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602 (Invalid params); message=%q",
			resp.Error.Code, resp.Error.Message)
	}
}

// TestMCP_DoctorRoundTrip ensures squad_doctor is registered and returns
// a structured findings list. README documents this tool but it was missing
// from registerTools, so tools/list omitted it and tools/call returned
// method-not-found.
func TestMCP_DoctorRoundTrip(t *testing.T) {
	env := newTestEnv(t)

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_doctor","arguments":{}}}` + "\n")
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
				Findings []map[string]any `json:"findings"`
				Total    int              `json:"total"`
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
		t.Fatalf("rpc error: code=%d msg=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result.IsError {
		t.Fatalf("tool error: %s", lines[1])
	}
	// A fresh repo may have zero findings; assert the response shape is sane
	// (Findings is non-nil and Total matches len) rather than the count.
	if resp.Result.StructuredContent.Total != len(resp.Result.StructuredContent.Findings) {
		t.Errorf("total = %d but findings len = %d",
			resp.Result.StructuredContent.Total,
			len(resp.Result.StructuredContent.Findings))
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
