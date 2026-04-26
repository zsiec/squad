package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/mcp"
)

// errNoRepo is returned by per-repo handlers when MCP was started outside a
// squad repo (so repoRoot/repoID are empty). The CLI surfaces this as a
// structured RPC error rather than crashing init.
var errNoRepo = errors.New("squad mcp: no repo discovered (run from inside a squad repo, or pass repo_root)")

func registerTools(srv *mcp.Server, db *sql.DB, repoID, repoRoot string) {
	registerLifecycleTools(srv, db, repoID, repoRoot)
	registerChatTools(srv, db, repoID, repoRoot)
	registerInspectionTools(srv, db, repoID, repoRoot)
	registerEvidenceTools(srv, db, repoID, repoRoot)
	registerLearningTools(srv, repoRoot)
}

func resolveAgentID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return identity.AgentID()
}

func requireRepo(repoRoot, repoID string) error {
	if repoRoot == "" || repoID == "" {
		return errNoRepo
	}
	return nil
}

func itemsDirOf(repoRoot string) string { return filepath.Join(repoRoot, ".squad", "items") }
func doneDirOf(repoRoot string) string  { return filepath.Join(repoRoot, ".squad", "done") }
func attDirOf(repoRoot string) string   { return filepath.Join(repoRoot, ".squad", "attestations") }

func registerLifecycleTools(srv *mcp.Server, db *sql.DB, repoID, repoRoot string) {
	srv.Register(mcp.Tool{
		Name:        "squad_register",
		Description: "Register this agent in the squad global database.",
		InputSchema: json.RawMessage(schemaRegister),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				As   string `json:"as"`
				Name string `json:"name"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			return Register(ctx, RegisterArgs{As: args.As, Name: args.Name})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_whoami",
		Description: "Return the agent id this session resolves to, plus any current claim.",
		InputSchema: json.RawMessage(schemaWhoami),
		Handler: func(ctx context.Context, _ json.RawMessage) (any, error) {
			return Whoami(ctx, WhoamiArgs{})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_next",
		Description: "List ready items in priority order (excludes items already claimed).",
		InputSchema: json.RawMessage(schemaNext),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Limit          int  `json:"limit"`
				IncludeClaimed bool `json:"include_claimed"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			res, err := NextItem(ctx, NextArgs{
				ItemsDir:       itemsDirOf(repoRoot),
				DoneDir:        doneDirOf(repoRoot),
				DB:             db,
				RepoID:         repoID,
				Limit:          args.Limit,
				IncludeClaimed: args.IncludeClaimed,
			})
			if err != nil {
				return nil, err
			}
			return res, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_claim",
		Description: "Atomically claim an item by ID for the current agent. Fails if another agent holds it.",
		InputSchema: json.RawMessage(schemaClaim),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID  string   `json:"item_id"`
				Intent  string   `json:"intent"`
				AgentID string   `json:"agent_id"`
				Touches []string `json:"touches"`
				Long    bool     `json:"long"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return Claim(ctx, ClaimArgs{
				DB:             db,
				RepoID:         repoID,
				AgentID:        agent,
				ItemID:         args.ItemID,
				Intent:         args.Intent,
				Touches:        args.Touches,
				Long:           args.Long,
				ItemsDir:       itemsDirOf(repoRoot),
				DoneDir:        doneDirOf(repoRoot),
				ConcurrencyCap: claimConcurrencyCap(),
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_release",
		Description: "Release the caller's claim on an item.",
		InputSchema: json.RawMessage(schemaRelease),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID  string `json:"item_id"`
				Outcome string `json:"outcome"`
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return Release(ctx, ReleaseArgs{
				DB: db, RepoID: repoID, AgentID: agent,
				ItemID: args.ItemID, Outcome: args.Outcome,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_done",
		Description: "Mark an item done: release claim, rewrite frontmatter, move to .squad/done/.",
		InputSchema: json.RawMessage(schemaDone),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID  string `json:"item_id"`
				Summary string `json:"summary"`
				AgentID string `json:"agent_id"`
				Force   bool   `json:"force"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return Done(ctx, DoneArgs{
				DB: db, RepoID: repoID, AgentID: agent,
				ItemID:   args.ItemID,
				Summary:  args.Summary,
				ItemsDir: itemsDirOf(repoRoot),
				DoneDir:  doneDirOf(repoRoot),
				RepoRoot: repoRoot,
				Force:    args.Force,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_blocked",
		Description: "Mark item blocked: release claim, set status: blocked, ensure ## Blocker section.",
		InputSchema: json.RawMessage(schemaBlocked),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID  string `json:"item_id"`
				Reason  string `json:"reason"`
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return Blocked(ctx, BlockedArgs{
				DB: db, RepoID: repoID, AgentID: agent,
				ItemID:   args.ItemID,
				Reason:   args.Reason,
				ItemsDir: itemsDirOf(repoRoot),
			})
		},
	})
}

func registerChatTools(srv *mcp.Server, db *sql.DB, repoID, repoRoot string) {
	srv.Register(mcp.Tool{
		Name:        "squad_say",
		Description: "Post a message to the current claim's thread, the global thread, or a specific item thread.",
		InputSchema: json.RawMessage(schemaSay),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Message string   `json:"message"`
				To      string   `json:"to"`
				Mention []string `json:"mention"`
				Verb    string   `json:"verb"`
				AgentID string   `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			c := newChatService(db, repoID)
			to := c.ResolveThread(ctx, agent, args.To)
			return Say(ctx, SayArgs{
				Chat:     c,
				AgentID:  agent,
				To:       to,
				Body:     args.Message,
				Mentions: args.Mention,
				Verb:     args.Verb,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_ask",
		Description: "Ask a question of a specific agent or thread.",
		InputSchema: json.RawMessage(schemaAsk),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Target   string `json:"target"`
				Question string `json:"question"`
				To       string `json:"to"`
				AgentID  string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			c := newChatService(db, repoID)
			return Ask(ctx, AskArgs{
				Chat:     c,
				AgentID:  agent,
				To:       args.To,
				Target:   args.Target,
				Question: args.Question,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_tick",
		Description: "Show new messages since the last tick and advance the read cursor.",
		InputSchema: json.RawMessage(schemaTick),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				AgentID string `json:"agent_id"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return Tick(ctx, TickArgs{Chat: newChatService(db, repoID), AgentID: agent})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_progress",
		Description: "Report progress on a held item.",
		InputSchema: json.RawMessage(schemaProgress),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID  string `json:"item_id"`
				Note    string `json:"note"`
				Pct     int    `json:"pct"`
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return Progress(ctx, ProgressArgs{
				DB: db, RepoID: repoID,
				Chat:    newChatService(db, repoID),
				AgentID: agent,
				ItemID:  args.ItemID,
				Pct:     args.Pct,
				Note:    args.Note,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_review_request",
		Description: "Request review on an item, optionally mentioning a reviewer.",
		InputSchema: json.RawMessage(schemaReviewRequest),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID   string `json:"item_id"`
				Reviewer string `json:"reviewer"`
				AgentID  string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return ReviewRequest(ctx, ReviewRequestArgs{
				Chat:     newChatService(db, repoID),
				AgentID:  agent,
				ItemID:   args.ItemID,
				Reviewer: args.Reviewer,
			})
		},
	})
}

func registerInspectionTools(srv *mcp.Server, db *sql.DB, repoID, repoRoot string) {
	srv.Register(mcp.Tool{
		Name:        "squad_list_items",
		Description: "List items filtered by status/type/priority/agent.",
		InputSchema: json.RawMessage(schemaListItems),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Status   string `json:"status"`
				Type     string `json:"type"`
				Priority string `json:"priority"`
				Agent    string `json:"agent"`
				Limit    int    `json:"limit"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			rows, err := ListItems(ctx, ListItemsArgs{
				ItemsDir: itemsDirOf(repoRoot),
				DoneDir:  doneDirOf(repoRoot),
				DB:       db,
				RepoID:   repoID,
				Status:   args.Status,
				Type:     args.Type,
				Priority: args.Priority,
				Agent:    args.Agent,
				Limit:    args.Limit,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"items": rows, "count": len(rows)}, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_get_item",
		Description: "Return the full item record (frontmatter + body + acceptance criteria).",
		InputSchema: json.RawMessage(schemaGetItem),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID string `json:"item_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			return GetItem(ctx, GetItemArgs{
				ItemsDir: itemsDirOf(repoRoot),
				ItemID:   args.ItemID,
			})
		},
	})
}

func registerEvidenceTools(srv *mcp.Server, db *sql.DB, repoID, repoRoot string) {
	srv.Register(mcp.Tool{
		Name:        "squad_attest",
		Description: "Record a verification artifact (test/lint/build/typecheck/review/manual) into the evidence ledger.",
		InputSchema: json.RawMessage(schemaAttest),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID        string `json:"item_id"`
				Kind          string `json:"kind"`
				Command       string `json:"command"`
				FindingsFile  string `json:"findings_file"`
				ReviewerAgent string `json:"reviewer_agent"`
				AgentID       string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return Attest(ctx, AttestArgs{
				DB: db, RepoID: repoID, AgentID: agent,
				ItemID:        args.ItemID,
				Kind:          args.Kind,
				Command:       args.Command,
				FindingsFile:  args.FindingsFile,
				ReviewerAgent: args.ReviewerAgent,
				AttDir:        attDirOf(repoRoot),
				RepoRoot:      repoRoot,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_attestations",
		Description: "List recorded attestations for an item.",
		InputSchema: json.RawMessage(schemaAttestations),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID string `json:"item_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			return Attestations(ctx, AttestationsArgs{
				DB: db, RepoID: repoID, ItemID: args.ItemID,
			})
		},
	})
}

func registerLearningTools(srv *mcp.Server, repoRoot string) {
	srv.Register(mcp.Tool{
		Name:        "squad_learning_propose",
		Description: "Stub a new learning artifact under .squad/learnings/<kind>s/proposed/.",
		InputSchema: json.RawMessage(schemaLearningPropose),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Kind      string   `json:"kind"`
				Slug      string   `json:"slug"`
				Title     string   `json:"title"`
				Area      string   `json:"area"`
				Paths     []string `json:"paths"`
				SessionID string   `json:"session_id"`
				AgentID   string   `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if repoRoot == "" {
				return nil, errNoRepo
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return LearningPropose(ctx, LearningProposeArgs{
				RepoRoot:  repoRoot,
				Kind:      args.Kind,
				Slug:      args.Slug,
				Title:     args.Title,
				Area:      args.Area,
				SessionID: args.SessionID,
				Paths:     args.Paths,
				CreatedBy: agent,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_learning_list",
		Description: "List learning artifacts (filterable by area, state, kind).",
		InputSchema: json.RawMessage(schemaLearningList),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Area  string `json:"area"`
				State string `json:"state"`
				Kind  string `json:"kind"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if repoRoot == "" {
				return nil, errNoRepo
			}
			return LearningList(ctx, LearningListArgs{
				RepoRoot: repoRoot,
				Area:     args.Area,
				State:    args.State,
				Kind:     args.Kind,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_learning_approve",
		Description: "Promote a proposed learning to approved/.",
		InputSchema: json.RawMessage(schemaLearningApprove),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Slug string `json:"slug"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if repoRoot == "" {
				return nil, errNoRepo
			}
			return LearningApprove(ctx, LearningApproveArgs{RepoRoot: repoRoot, Slug: args.Slug})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_learning_reject",
		Description: "Archive a proposed learning under rejected/ with a reason.",
		InputSchema: json.RawMessage(schemaLearningReject),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Slug   string `json:"slug"`
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if repoRoot == "" {
				return nil, errNoRepo
			}
			return LearningReject(ctx, LearningRejectArgs{
				RepoRoot: repoRoot, Slug: args.Slug, Reason: args.Reason,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_learning_agents_md_suggest",
		Description: "Propose a unified-diff change to AGENTS.md (human applies via agents-md-approve).",
		InputSchema: json.RawMessage(schemaLearningAgentsMdSuggest),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				DiffPath  string `json:"diff_path"`
				Rationale string `json:"rationale"`
				Slug      string `json:"slug"`
				AgentID   string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if repoRoot == "" {
				return nil, errNoRepo
			}
			agent, err := resolveAgentID(args.AgentID)
			if err != nil {
				return nil, err
			}
			return LearningAgentsMdSuggest(ctx, LearningAgentsMdSuggestArgs{
				RepoRoot:  repoRoot,
				DiffPath:  args.DiffPath,
				Rationale: args.Rationale,
				Slug:      args.Slug,
				CreatedBy: agent,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_learning_agents_md_approve",
		Description: "Apply a proposed AGENTS.md diff via `git apply`; on success, archive the proposal.",
		InputSchema: json.RawMessage(schemaLearningAgentsMdApprove),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if repoRoot == "" {
				return nil, errNoRepo
			}
			res, err := LearningAgentsMdApprove(ctx, LearningAgentsMdApproveArgs{
				RepoRoot: repoRoot, ID: args.ID,
			})
			if err != nil {
				var af *ApplyFailedError
				if errors.As(err, &af) {
					return nil, fmt.Errorf("git apply failed: %s", af.Stderr)
				}
				return nil, err
			}
			return res, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_learning_agents_md_reject",
		Description: "Archive a proposed AGENTS.md change under rejected/ with a reason.",
		InputSchema: json.RawMessage(schemaLearningAgentsMdReject),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ID     string `json:"id"`
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if repoRoot == "" {
				return nil, errNoRepo
			}
			return LearningAgentsMdReject(ctx, LearningAgentsMdRejectArgs{
				RepoRoot: repoRoot, ID: args.ID, Reason: args.Reason,
			})
		},
	})
}
