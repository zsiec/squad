package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/config"
)

func TestTouchesPolicy_NoConflictReportsClean(t *testing.T) {
	env := newTestEnv(t)

	out, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		Path:    "internal/foo/bar.go",
	})
	if err != nil {
		t.Fatalf("TouchesPolicy: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if v, ok := parsed["conflict"].(bool); !ok || v {
		t.Fatalf("conflict=%v ok=%v want false/true", v, ok)
	}
}

func TestTouchesPolicy_WarnModeAsks(t *testing.T) {
	env := newTestEnv(t)
	insertOtherTouch(t, env, "agent-bbbb", "internal/foo/bar.go")

	out, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		Path:    "internal/foo/bar.go",
	})
	if err != nil {
		t.Fatalf("TouchesPolicy: %v", err)
	}
	hso := decodeHookOutput(t, out)
	if hso.HookEventName != "PreToolUse" {
		t.Fatalf("hookEventName=%q want PreToolUse", hso.HookEventName)
	}
	if hso.PermissionDecision != "ask" {
		t.Fatalf("permissionDecision=%q want ask", hso.PermissionDecision)
	}
	if !strings.Contains(hso.AdditionalContext, "agent-bbbb") {
		t.Fatalf("additionalContext missing owner: %q", hso.AdditionalContext)
	}
	if !strings.Contains(hso.AdditionalContext, "internal/foo/bar.go") {
		t.Fatalf("additionalContext missing path: %q", hso.AdditionalContext)
	}
}

func TestTouchesPolicy_DenyModeMatchingPathBlocks(t *testing.T) {
	env := newTestEnv(t)
	insertOtherTouch(t, env, "agent-cccc", "go.mod")

	out, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		Path:    "go.mod",
		Cfg: config.TouchConfig{
			Enforcement:      config.TouchEnforcementDeny,
			EnforcementPaths: []string{"go.mod", "**/*.lock"},
		},
	})
	if err != nil {
		t.Fatalf("TouchesPolicy: %v", err)
	}
	hso := decodeHookOutput(t, out)
	if hso.PermissionDecision != "deny" {
		t.Fatalf("permissionDecision=%q want deny", hso.PermissionDecision)
	}
	if !strings.Contains(hso.AdditionalContext, "agent-cccc") {
		t.Fatalf("additionalContext missing owner: %q", hso.AdditionalContext)
	}
	if !strings.Contains(strings.ToLower(hso.AdditionalContext), "blocked") {
		t.Fatalf("deny message should say blocked: %q", hso.AdditionalContext)
	}
}

func TestTouchesPolicy_DenyModeNonMatchingPathFallsBackToAsk(t *testing.T) {
	env := newTestEnv(t)
	insertOtherTouch(t, env, "agent-dddd", "internal/foo/bar.go")

	out, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		Path:    "internal/foo/bar.go",
		Cfg: config.TouchConfig{
			Enforcement:      config.TouchEnforcementDeny,
			EnforcementPaths: []string{"go.mod", "**/*.lock"},
		},
	})
	if err != nil {
		t.Fatalf("TouchesPolicy: %v", err)
	}
	hso := decodeHookOutput(t, out)
	if hso.PermissionDecision != "ask" {
		t.Fatalf("permissionDecision=%q want ask (path not in enforcement_paths)", hso.PermissionDecision)
	}
}

func TestTouchesPolicy_DenyModeNestedGlobMatch(t *testing.T) {
	env := newTestEnv(t)
	insertOtherTouch(t, env, "agent-eeee", "vendor/foo/yarn.lock")

	out, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		Path:    "vendor/foo/yarn.lock",
		Cfg: config.TouchConfig{
			Enforcement:      config.TouchEnforcementDeny,
			EnforcementPaths: []string{"go.mod", "**/*.lock"},
		},
	})
	if err != nil {
		t.Fatalf("TouchesPolicy: %v", err)
	}
	hso := decodeHookOutput(t, out)
	if hso.PermissionDecision != "deny" {
		t.Fatalf("permissionDecision=%q want deny (matched **/*.lock)", hso.PermissionDecision)
	}
}

func TestTouchesPolicy_RecordsTouchOnFirstEdit(t *testing.T) {
	env := newTestEnv(t)

	out, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB:      env.DB,
		RepoID:  env.RepoID,
		AgentID: env.AgentID,
		ItemID:  "FEAT-001",
		Path:    "internal/foo/bar.go",
	})
	if err != nil {
		t.Fatalf("TouchesPolicy: %v", err)
	}
	if !strings.Contains(out, `"conflict":false`) {
		t.Fatalf("expected clean JSON, got: %s", out)
	}
	var n int
	if err := env.DB.QueryRow(`
		SELECT count(*) FROM touches
		WHERE repo_id=? AND agent_id=? AND path=? AND released_at IS NULL
	`, env.RepoID, env.AgentID, "internal/foo/bar.go").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("touches rows after first edit=%d, want 1", n)
	}

	if _, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "FEAT-001", Path: "internal/foo/bar.go",
	}); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if err := env.DB.QueryRow(`
		SELECT count(*) FROM touches
		WHERE repo_id=? AND agent_id=? AND path=? AND released_at IS NULL
	`, env.RepoID, env.AgentID, "internal/foo/bar.go").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("touches rows after second edit=%d, want 1 (idempotent)", n)
	}
}

func TestTouchesPolicy_ReleaseClosesTouchRow(t *testing.T) {
	env := newTestEnv(t)

	if _, err := TouchesPolicy(context.Background(), TouchesPolicyArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "FEAT-002", Path: "internal/foo/bar.go",
	}); err != nil {
		t.Fatalf("policy: %v", err)
	}

	if _, err := env.DB.Exec(`
		UPDATE touches SET released_at = strftime('%s','now')
		WHERE repo_id=? AND agent_id=? AND released_at IS NULL
	`, env.RepoID, env.AgentID); err != nil {
		t.Fatal(err)
	}

	var releasedAt sql.NullInt64
	if err := env.DB.QueryRow(`
		SELECT released_at FROM touches
		WHERE repo_id=? AND agent_id=? AND path=?
		ORDER BY started_at DESC LIMIT 1
	`, env.RepoID, env.AgentID, "internal/foo/bar.go").Scan(&releasedAt); err != nil {
		t.Fatal(err)
	}
	if !releasedAt.Valid {
		t.Fatalf("released_at not set after release")
	}
}

func TestPluginHooksJSON_PreEditTouchCheckRegistered(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "plugin", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("hooks.json invalid: %v", err)
	}
	hooks, _ := doc["hooks"].(map[string]any)
	pre, _ := hooks["PreToolUse"].([]any)
	var found bool
	for _, group := range pre {
		g, _ := group.(map[string]any)
		if matcher, _ := g["matcher"].(string); matcher != "Edit|Write" {
			continue
		}
		entries, _ := g["hooks"].([]any)
		for _, entry := range entries {
			e, _ := entry.(map[string]any)
			cmd, _ := e["command"].(string)
			if strings.HasSuffix(cmd, "/pre_edit_touch_check.sh") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("plugin/hooks.json: PreToolUse matcher Edit|Write missing pre_edit_touch_check.sh entry")
	}
}

func TestTouchesPolicyCmd_WarnsOnUnknownEnforcement(t *testing.T) {
	root := setupSquadRepo(t)
	t.Chdir(root)
	cfgPath := filepath.Join(root, ".squad", "config.yaml")
	body := "touch:\n  enforcement: denied\n  enforcement_paths:\n    - go.mod\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newTouchesPolicyCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"internal/foo/bar.go"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stderr.String()
	for _, sub := range []string{"squad: warning:", "denied", "warn", "deny"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("stderr missing %q: %q", sub, got)
		}
	}
}

type hookOutputForTest struct {
	HookSpecificOutput struct {
		HookEventName      string `json:"hookEventName"`
		PermissionDecision string `json:"permissionDecision"`
		AdditionalContext  string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

type flatHookOutput struct {
	HookEventName      string
	PermissionDecision string
	AdditionalContext  string
}

func decodeHookOutput(t *testing.T, raw string) flatHookOutput {
	t.Helper()
	var v hookOutputForTest
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, raw)
	}
	return flatHookOutput{
		HookEventName:      v.HookSpecificOutput.HookEventName,
		PermissionDecision: v.HookSpecificOutput.PermissionDecision,
		AdditionalContext:  v.HookSpecificOutput.AdditionalContext,
	}
}

func insertOtherTouch(t *testing.T, env *testEnv, agentID, path string) {
	t.Helper()
	if _, err := env.DB.Exec(`
		INSERT INTO touches (repo_id, agent_id, item_id, path, started_at)
		VALUES (?, ?, NULL, ?, 1000)
	`, env.RepoID, agentID, path); err != nil {
		t.Fatalf("insert touch: %v", err)
	}
}
