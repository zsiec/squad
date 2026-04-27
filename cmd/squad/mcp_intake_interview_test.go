package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/intake"
	"github.com/zsiec/squad/internal/mcp"
)

// TestIntakeErrToToolError_RefineSentinels covers the two refine-mode
// typed sentinels that previously fell through to CodeInternal. Both
// signal user-input problems (bad refine_item_id, mismatched resume id)
// and must surface as CodeInvalidParams. The wrapped form mirrors the
// fmt.Errorf("%w: ...") shape produced by commit_run.go and session.go.
func TestIntakeErrToToolError_RefineSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"bare ErrIntakeItemNotRefinable", intake.ErrIntakeItemNotRefinable},
		{"wrapped ErrIntakeItemNotRefinable",
			fmt.Errorf("%w: %s is %q", intake.ErrIntakeItemNotRefinable, "BUG-099", "done")},
		{"bare ErrIntakeRefineItemMismatch", intake.ErrIntakeRefineItemMismatch},
		{"wrapped ErrIntakeRefineItemMismatch",
			fmt.Errorf("%w: session targets %s", intake.ErrIntakeRefineItemMismatch, "FEAT-001")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := intakeErrToToolError(tc.err)
			var te *mcp.ToolError
			if !errors.As(got, &te) {
				t.Fatalf("intakeErrToToolError(%v) = %T (%v); want *mcp.ToolError", tc.err, got, got)
			}
			if te.Code != mcp.CodeInvalidParams {
				t.Errorf("Code = %d; want %d (CodeInvalidParams)", te.Code, mcp.CodeInvalidParams)
			}
		})
	}
}

// TestMCP_IntakeOpen_ThenTurnThenStatus_Roundtrip drives the happy
// path: open a fresh new-mode session, append one turn, status returns
// the transcript with one entry. Asserts each tool's structured
// content shape against the design.
func TestMCP_IntakeOpen_ThenTurnThenStatus_Roundtrip(t *testing.T) {
	env := newTestEnv(t)

	openResp := callMCPTool(t, env, "squad_intake_open",
		`{"mode":"new","idea_seed":"rotate signing keys without downtime"}`)
	if openResp.Error != nil {
		t.Fatalf("open: %+v", openResp.Error)
	}
	sessionID, _ := openResp.Result.StructuredContent["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("open: session_id empty in %+v", openResp.Result.StructuredContent)
	}
	if mode := openResp.Result.StructuredContent["mode"]; mode != "new" {
		t.Errorf("open mode=%v want new", mode)
	}
	if resumed, _ := openResp.Result.StructuredContent["resumed"].(bool); resumed {
		t.Errorf("first open should not be resumed")
	}

	turnResp := callMCPTool(t, env, "squad_intake_turn",
		`{"session_id":"`+sessionID+`","role":"agent","content":"What's the rotation cadence?","fields_filled":["title"]}`)
	if turnResp.Error != nil {
		t.Fatalf("turn: %+v", turnResp.Error)
	}
	if seq, _ := turnResp.Result.StructuredContent["seq"].(float64); seq != 1 {
		t.Errorf("turn seq=%v want 1", seq)
	}

	statusResp := callMCPTool(t, env, "squad_intake_status",
		`{"session_id":"`+sessionID+`"}`)
	if statusResp.Error != nil {
		t.Fatalf("status: %+v", statusResp.Error)
	}
	transcript, ok := statusResp.Result.StructuredContent["transcript"].([]any)
	if !ok || len(transcript) != 1 {
		t.Errorf("status transcript=%v want 1 entry", statusResp.Result.StructuredContent["transcript"])
	}
}

// TestMCP_IntakeOpen_ResumeReturnsResumedTrue confirms that calling
// open twice for the same agent returns the same session id with the
// resumed flag flipped.
func TestMCP_IntakeOpen_ResumeReturnsResumedTrue(t *testing.T) {
	env := newTestEnv(t)

	first := callMCPTool(t, env, "squad_intake_open",
		`{"mode":"new","idea_seed":"x"}`)
	if first.Error != nil {
		t.Fatalf("open #1: %+v", first.Error)
	}
	id1, _ := first.Result.StructuredContent["session_id"].(string)

	second := callMCPTool(t, env, "squad_intake_open",
		`{"mode":"new","idea_seed":"x"}`)
	if second.Error != nil {
		t.Fatalf("open #2: %+v", second.Error)
	}
	id2, _ := second.Result.StructuredContent["session_id"].(string)
	if id1 != id2 {
		t.Errorf("resume should return same session id, got %q and %q", id1, id2)
	}
	if resumed, _ := second.Result.StructuredContent["resumed"].(bool); !resumed {
		t.Errorf("second open should be resumed=true")
	}
}

// TestMCP_IntakeStatus_NotFoundMapsToNotFoundCode exercises the
// IntakeNotFound → CodeNotFound mapping.
func TestMCP_IntakeStatus_NotFoundMapsToNotFoundCode(t *testing.T) {
	env := newTestEnv(t)

	resp := callMCPTool(t, env, "squad_intake_status",
		`{"session_id":"intake-19700101-deadbeef"}`)
	if resp.Error == nil {
		t.Fatalf("expected rpc error for unknown session, got result %+v", resp.Result.StructuredContent)
	}
	if resp.Error.Code != mcp.CodeNotFound {
		t.Errorf("error code = %d, want %d (CodeNotFound)", resp.Error.Code, mcp.CodeNotFound)
	}
}

// TestMCP_IntakeTurn_InvalidParamsOnEmptyContent verifies non-typed
// validation errors (intake's plain fmt.Errorf for empty content) fall
// through to the catch-all internal code, NOT the typed error mappings.
func TestMCP_IntakeTurn_InvalidParamsOnEmptyContent(t *testing.T) {
	env := newTestEnv(t)
	openResp := callMCPTool(t, env, "squad_intake_open", `{"mode":"new","idea_seed":"x"}`)
	sessionID, _ := openResp.Result.StructuredContent["session_id"].(string)

	// Empty content; intake.AppendTurn returns a plain error (not a
	// typed sentinel), so it should surface as the catch-all internal
	// code rather than be silently mapped to one of the AC-mapped codes.
	resp := callMCPTool(t, env, "squad_intake_turn",
		`{"session_id":"`+sessionID+`","role":"agent","content":""}`)
	if resp.Error == nil {
		t.Fatalf("expected rpc error for empty content")
	}
	if resp.Error.Code != mcp.CodeInternal {
		t.Errorf("untyped error should land at CodeInternal=%d, got %d", mcp.CodeInternal, resp.Error.Code)
	}
}

// TestMCP_IntakeCommit_SlugConflictMapsToConflictCode pre-seeds a spec
// row that collides with the bundle's spec slug, then asserts commit
// surfaces the *IntakeSlugConflict typed error as CodeConflict.
func TestMCP_IntakeCommit_SlugConflictMapsToConflictCode(t *testing.T) {
	env := newTestEnv(t)

	openResp := callMCPTool(t, env, "squad_intake_open", `{"mode":"new","idea_seed":"x"}`)
	sessionID, _ := openResp.Result.StructuredContent["session_id"].(string)

	// Pre-seed the spec slug "auth-rotation" so the bundle below
	// triggers IntakeSlugConflict on commit.
	if _, err := env.DB.Exec(
		`INSERT INTO specs (repo_id, name, title, path, updated_at) VALUES (?, ?, ?, ?, 0)`,
		env.RepoID, "auth-rotation", "preexisting", env.Root+"/.squad/specs/auth-rotation.md",
	); err != nil {
		t.Fatalf("seed conflict: %v", err)
	}

	bundle := map[string]any{
		"spec": map[string]any{
			"title":       "auth rotation",
			"motivation":  "rotate keys",
			"acceptance":  []string{"keys rotate"},
			"non_goals":   []string{"changing algo"},
			"integration": []string{"middleware"},
		},
		"epics": []map[string]any{
			{"title": "core", "parallelism": "serial", "dependencies": []string{}},
		},
		"items": []map[string]any{
			{"title": "first item", "intent": "do x", "acceptance": []string{"works"}, "area": "auth", "epic": "core"},
		},
	}
	bundleJSON, _ := json.Marshal(bundle)
	args := `{"session_id":"` + sessionID + `","bundle":` + string(bundleJSON) + `}`

	resp := callMCPTool(t, env, "squad_intake_commit", args)
	if resp.Error == nil {
		t.Fatalf("expected rpc error for slug conflict, got result %+v", resp.Result.StructuredContent)
	}
	if resp.Error.Code != mcp.CodeConflict {
		t.Errorf("error code = %d, want %d (CodeConflict). Message: %s",
			resp.Error.Code, mcp.CodeConflict, resp.Error.Message)
	}
	if !strings.Contains(resp.Error.Message, "auth-rotation") {
		t.Errorf("error message should name the conflicting slug, got %q", resp.Error.Message)
	}
}

// TestMCP_IntakeCommit_IncompleteBundleMapsToInvalidParams: missing
// required field in the bundle should surface IntakeIncomplete →
// CodeInvalidParams.
func TestMCP_IntakeCommit_IncompleteBundleMapsToInvalidParams(t *testing.T) {
	env := newTestEnv(t)

	openResp := callMCPTool(t, env, "squad_intake_open", `{"mode":"new","idea_seed":"x"}`)
	sessionID, _ := openResp.Result.StructuredContent["session_id"].(string)

	// item missing required `intent` field — Validate should reject it
	// with IntakeIncomplete.
	args := `{"session_id":"` + sessionID + `","bundle":{"items":[{"title":"x","acceptance":["y"],"area":"z"}]}}`
	resp := callMCPTool(t, env, "squad_intake_commit", args)
	if resp.Error == nil {
		t.Fatalf("expected rpc error for incomplete bundle")
	}
	if resp.Error.Code != mcp.CodeInvalidParams {
		t.Errorf("error code = %d, want %d (CodeInvalidParams). Message: %s",
			resp.Error.Code, mcp.CodeInvalidParams, resp.Error.Message)
	}
}

// TestMCP_NoIntakeCancel asserts that the explicit no-cancel design
// holds: the tool MUST NOT be registered. Mirrors the AC's
// "by design, Claude does not have a tool for throwing user work away".
func TestMCP_NoIntakeCancel(t *testing.T) {
	env := newTestEnv(t)
	resp := callMCPTool(t, env, "squad_intake_cancel", `{}`)
	if resp.Error == nil {
		t.Fatalf("squad_intake_cancel should not be a registered tool, got result %+v", resp.Result.StructuredContent)
	}
	if resp.Error.Code != mcp.CodeMethodNotFound {
		t.Errorf("error code = %d, want %d (CodeMethodNotFound)", resp.Error.Code, mcp.CodeMethodNotFound)
	}
}
