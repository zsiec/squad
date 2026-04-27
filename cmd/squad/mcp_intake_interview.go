package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/intake"
	"github.com/zsiec/squad/internal/mcp"
)

// registerIntakeInterviewTools wires the four intake-interview MCP tools
// onto srv. By design there is no squad_intake_cancel — Claude does not
// get a tool that throws user work away. Cancellation is a CLI-only
// affordance.
//
// All four tools derive the calling agent id via identity.AgentID() and
// translate intake's typed errors into JSON-RPC error codes per
// intakeErrToToolError.
func registerIntakeInterviewTools(srv *mcp.Server, db *sql.DB, repoID, repoRoot string) {
	srv.Register(mcp.Tool{
		Name:        "squad_intake_open",
		Description: "Open or resume an intake-interview session. Mode is \"new\" or \"refine\"; refine requires refine_item_id. Returns the session id, resumed flag, and (for refine) a snapshot of the original item.",
		InputSchema: json.RawMessage(schemaIntakeOpen),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Mode         string `json:"mode"`
				IdeaSeed     string `json:"idea_seed,omitempty"`
				RefineItemID string `json:"refine_item_id,omitempty"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, &mcp.ToolError{Code: mcp.CodeInvalidParams, Err: err}
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agentID, _ := identity.AgentID()
			sess, snap, resumed, err := intake.Open(ctx, db, intake.OpenParams{
				RepoID:       repoID,
				AgentID:      agentID,
				Mode:         args.Mode,
				IdeaSeed:     args.IdeaSeed,
				RefineItemID: args.RefineItemID,
				SquadDir:     filepath.Join(repoRoot, ".squad"),
			})
			if err != nil {
				return nil, intakeErrToToolError(err)
			}
			out := map[string]any{
				"session_id": sess.ID,
				"mode":       sess.Mode,
				"status":     sess.Status,
				"resumed":    resumed,
			}
			if sess.Mode == intake.ModeRefine {
				out["snapshot"] = snap
			}
			return out, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_intake_turn",
		Description: "Append one transcript turn to an open intake session. role is user|agent|system; fields_filled is the agent's honor-system claim about which checklist fields this turn satisfied.",
		InputSchema: json.RawMessage(schemaIntakeTurn),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				SessionID    string   `json:"session_id"`
				Role         string   `json:"role"`
				Content      string   `json:"content"`
				FieldsFilled []string `json:"fields_filled,omitempty"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, &mcp.ToolError{Code: mcp.CodeInvalidParams, Err: err}
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agentID, _ := identity.AgentID()
			checklist, err := intake.LoadChecklist(filepath.Join(repoRoot, ".squad"))
			if err != nil {
				return nil, &mcp.ToolError{Code: mcp.CodeInternal, Err: err}
			}
			seq, stillRequired, err := intake.AppendTurn(
				ctx, db, checklist, args.SessionID, agentID, args.Role, args.Content, args.FieldsFilled,
			)
			if err != nil {
				return nil, intakeErrToToolError(err)
			}
			return map[string]any{
				"seq":            seq,
				"still_required": stillRequired,
			}, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_intake_status",
		Description: "Read-only: return the full transcript and current still_required for the session. Cancelled or committed sessions still return for audit history.",
		InputSchema: json.RawMessage(schemaIntakeStatus),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				SessionID string `json:"session_id"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, &mcp.ToolError{Code: mcp.CodeInvalidParams, Err: err}
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agentID, _ := identity.AgentID()
			checklist, err := intake.LoadChecklist(filepath.Join(repoRoot, ".squad"))
			if err != nil {
				return nil, &mcp.ToolError{Code: mcp.CodeInternal, Err: err}
			}
			res, err := intake.Status(ctx, db, checklist, args.SessionID, agentID)
			if err != nil {
				return nil, intakeErrToToolError(err)
			}
			return res, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_intake_commit",
		Description: "Validate and commit the bundle. Writes the spec/epic/item files, inserts rows in one tx, marks the session committed. ready=true makes the items immediately claimable; default leaves them captured for triage.",
		InputSchema: json.RawMessage(schemaIntakeCommit),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				SessionID string        `json:"session_id"`
				Bundle    intake.Bundle `json:"bundle"`
				Ready     bool          `json:"ready,omitempty"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, &mcp.ToolError{Code: mcp.CodeInvalidParams, Err: err}
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agentID, _ := identity.AgentID()
			res, err := intake.Commit(
				ctx, db, filepath.Join(repoRoot, ".squad"), args.SessionID, agentID, args.Bundle, args.Ready,
			)
			if err != nil {
				return nil, intakeErrToToolError(err)
			}
			return map[string]any{
				"shape":    res.Shape,
				"item_ids": res.ItemIDs,
				"paths":    res.Paths,
			}, nil
		},
	})
}

// intakeErrToToolError maps the intake package's typed errors onto
// JSON-RPC error codes. Untyped errors (plain fmt.Errorf from intake
// validation) intentionally fall through to the catch-all Internal so
// callTool surfaces them as -32603 — only the enumerated typed
// sentinels get user-facing codes. If you find yourself wanting to map
// a new untyped error here, the right move is to make it a typed
// sentinel in the intake package first.
func intakeErrToToolError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, intake.ErrIntakeNotFound):
		return &mcp.ToolError{Code: mcp.CodeNotFound, Err: err}
	case errors.Is(err, intake.ErrIntakeNotYours):
		return &mcp.ToolError{Code: mcp.CodeInvalidRequest, Err: err}
	case errors.Is(err, intake.ErrIntakeAlreadyClosed):
		return &mcp.ToolError{Code: mcp.CodeInvalidRequest, Err: err}
	case errors.Is(err, intake.ErrIntakeItemNotRefinable),
		errors.Is(err, intake.ErrIntakeRefineItemMismatch):
		return &mcp.ToolError{Code: mcp.CodeInvalidParams, Err: err}
	}
	var slug *intake.IntakeSlugConflict
	if errors.As(err, &slug) {
		return &mcp.ToolError{Code: mcp.CodeConflict, Err: err}
	}
	var incomplete *intake.IntakeIncomplete
	if errors.As(err, &incomplete) {
		return &mcp.ToolError{Code: mcp.CodeInvalidParams, Err: err}
	}
	var shape *intake.IntakeShapeInvalid
	if errors.As(err, &shape) {
		return &mcp.ToolError{Code: mcp.CodeInvalidParams, Err: err}
	}
	return err
}
