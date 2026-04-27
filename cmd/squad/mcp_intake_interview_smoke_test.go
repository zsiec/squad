package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

// TestMCP_IntakeSmoke_GreenfieldProducesItemOnlyBundle drives the full
// new-mode loop end-to-end via the MCP tools — open, several turns,
// status, commit — and verifies the result is an item_only bundle on
// disk + in the DB. Stand-in for the AC's "manual smoke test in a
// fresh tmp repo" requirement.
func TestMCP_IntakeSmoke_GreenfieldProducesItemOnlyBundle(t *testing.T) {
	env := newTestEnv(t)

	openResp := callMCPTool(t, env, "squad_intake_open",
		`{"mode":"new","idea_seed":"add a bulk reassign command for stale claims"}`)
	if openResp.Error != nil {
		t.Fatalf("open: %+v", openResp.Error)
	}
	sessionID, _ := openResp.Result.StructuredContent["session_id"].(string)

	for _, turn := range []struct {
		role, content string
		fields        []string
	}{
		{"user", "let's add a bulk reassign verb so we don't have to one-at-a-time stale claims", []string{"title"}},
		{"user", "the goal is one command, multiple items, prints a summary table", []string{"intent"}},
		{"user", "succeeds with all reassigns recorded; refuses if any item id is unknown", []string{"acceptance"}},
		{"user", "claims", []string{"area"}},
	} {
		args, _ := json.Marshal(map[string]any{
			"session_id":    sessionID,
			"role":          turn.role,
			"content":       turn.content,
			"fields_filled": turn.fields,
		})
		resp := callMCPTool(t, env, "squad_intake_turn", string(args))
		if resp.Error != nil {
			t.Fatalf("turn (%s): %+v", turn.fields, resp.Error)
		}
	}

	statusResp := callMCPTool(t, env, "squad_intake_status",
		`{"session_id":"`+sessionID+`"}`)
	if statusResp.Error != nil {
		t.Fatalf("status: %+v", statusResp.Error)
	}
	stillRequired, _ := statusResp.Result.StructuredContent["still_required"].([]any)
	if len(stillRequired) != 0 {
		t.Errorf("still_required = %v; want empty before commit", stillRequired)
	}

	bundle := map[string]any{
		"items": []map[string]any{
			{
				"title":      "bulk reassign command for stale claims",
				"intent":     "one command for many reassignments with a printed summary",
				"acceptance": []string{"all reassigns recorded", "unknown ids cause refusal"},
				"area":       "claims",
				"kind":       "feat",
			},
		},
	}
	bundleJSON, _ := json.Marshal(bundle)
	commitArgs := `{"session_id":"` + sessionID + `","bundle":` + string(bundleJSON) + `}`
	commitResp := callMCPTool(t, env, "squad_intake_commit", commitArgs)
	if commitResp.Error != nil {
		t.Fatalf("commit: %+v", commitResp.Error)
	}
	if shape := commitResp.Result.StructuredContent["shape"]; shape != "item_only" {
		t.Errorf("shape = %v want item_only", shape)
	}
	itemIDs, _ := commitResp.Result.StructuredContent["item_ids"].([]any)
	if len(itemIDs) != 1 {
		t.Fatalf("item_ids = %v want 1", itemIDs)
	}
	id := itemIDs[0].(string)

	var status, sessionLink string
	if err := env.DB.QueryRow(
		`SELECT status, COALESCE(intake_session_id,'') FROM items WHERE repo_id=? AND item_id=?`,
		env.RepoID, id,
	).Scan(&status, &sessionLink); err != nil {
		t.Fatalf("scan item row: %v", err)
	}
	if status != "captured" {
		t.Errorf("item status=%q want captured", status)
	}
	if sessionLink != sessionID {
		t.Errorf("intake_session_id=%q want %q", sessionLink, sessionID)
	}

	paths, _ := commitResp.Result.StructuredContent["paths"].([]any)
	if len(paths) != 1 {
		t.Fatalf("paths = %v want 1", paths)
	}
	if _, err := os.Stat(paths[0].(string)); err != nil {
		t.Errorf("item file missing on disk: %v", err)
	}
}

// TestMCP_IntakeSmoke_RefineSupersedesStubItem drives the refine-mode
// loop end-to-end: pre-seed a captured stub item, open in refine mode,
// turn, commit, then verify the original is archived (status=done,
// archived=1) and a fresh item replaces it.
func TestMCP_IntakeSmoke_RefineSupersedesStubItem(t *testing.T) {
	env := newTestEnv(t)

	stubPath, err := items.NewWithOptions(filepath.Join(env.Root, ".squad"), "FEAT", "rough idea about retries", items.Options{
		CapturedBy: "agent-stub",
	})
	if err != nil {
		t.Fatal(err)
	}
	stub, err := items.Parse(stubPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := items.Persist(context.Background(), env.DB, env.RepoID, stub, false); err != nil {
		t.Fatal(err)
	}

	openResp := callMCPTool(t, env, "squad_intake_open",
		`{"mode":"refine","refine_item_id":"`+stub.ID+`"}`)
	if openResp.Error != nil {
		t.Fatalf("open refine: %+v", openResp.Error)
	}
	sessionID, _ := openResp.Result.StructuredContent["session_id"].(string)
	if mode := openResp.Result.StructuredContent["mode"]; mode != "refine" {
		t.Errorf("mode=%v want refine", mode)
	}

	turnArgs, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"role":       "user",
		"content":    "we want exponential backoff capped at 30s with jitter",
		"fields_filled": []string{"title", "intent", "acceptance", "area"},
	})
	turnResp := callMCPTool(t, env, "squad_intake_turn", string(turnArgs))
	if turnResp.Error != nil {
		t.Fatalf("turn: %+v", turnResp.Error)
	}

	bundle := map[string]any{
		"items": []map[string]any{
			{
				"title":      "exponential backoff with jitter for retries",
				"intent":     "stop hammering deps when they're degraded",
				"acceptance": []string{"capped at 30s", "jitter applied"},
				"area":       "client",
				"kind":       "feat",
			},
		},
	}
	bundleJSON, _ := json.Marshal(bundle)
	commitResp := callMCPTool(t, env, "squad_intake_commit",
		`{"session_id":"`+sessionID+`","bundle":`+string(bundleJSON)+`}`)
	if commitResp.Error != nil {
		t.Fatalf("commit refine: %+v", commitResp.Error)
	}

	var origStatus string
	var archived int
	var origPath string
	if err := env.DB.QueryRow(
		`SELECT status, archived, path FROM items WHERE repo_id=? AND item_id=?`,
		env.RepoID, stub.ID,
	).Scan(&origStatus, &archived, &origPath); err != nil {
		t.Fatalf("scan original: %v", err)
	}
	if origStatus != "done" || archived != 1 {
		t.Errorf("original after refine: status=%q archived=%d; want done/1", origStatus, archived)
	}
	if !strings.Contains(origPath, ".archive") {
		t.Errorf("original path should be in .archive, got %q", origPath)
	}

	itemIDs, _ := commitResp.Result.StructuredContent["item_ids"].([]any)
	if len(itemIDs) != 1 {
		t.Fatalf("refine should produce exactly 1 new item, got %v", itemIDs)
	}
	newID := itemIDs[0].(string)
	if newID == stub.ID {
		t.Errorf("new item id %q should differ from original %q", newID, stub.ID)
	}
}
