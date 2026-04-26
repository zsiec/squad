package main

import (
	"context"
	"encoding/json"
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
