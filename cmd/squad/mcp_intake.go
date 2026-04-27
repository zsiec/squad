package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/mcp"
)

func registerIntakeTools(srv *mcp.Server, db *sql.DB, repoID, repoRoot string) {
	srv.Register(mcp.Tool{
		Name:        "squad_new",
		Description: "Create a new work item. Defaults to captured/inbox; pass ready:true to file as immediately claimable.",
		InputSchema: json.RawMessage(schemaNew),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Type     string `json:"type"`
				Title    string `json:"title"`
				Priority string `json:"priority,omitempty"`
				Area     string `json:"area,omitempty"`
				Estimate string `json:"estimate,omitempty"`
				Risk     string `json:"risk,omitempty"`
				Ready    bool   `json:"ready,omitempty"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			if strings.TrimSpace(args.Title) == "" {
				return nil, fmt.Errorf("title required")
			}
			typ := strings.ToLower(args.Type)
			prefix, ok := typeToPrefix[typ]
			if !ok {
				prefix = strings.ToUpper(typ)
			}
			cfg, err := config.Load(repoRoot)
			if err != nil {
				return nil, fmt.Errorf("load config: %w", err)
			}
			if !containsString(cfg.IDPrefixes, prefix) {
				return nil, fmt.Errorf("type %q maps to prefix %q which is not in id_prefixes %v",
					typ, prefix, cfg.IDPrefixes)
			}
			agentID, _ := identity.AgentID()
			path, err := items.NewWithOptions(filepath.Join(repoRoot, ".squad"), prefix, args.Title, items.Options{
				Priority:   nonEmpty(args.Priority, cfg.Defaults.Priority),
				Estimate:   nonEmpty(args.Estimate, cfg.Defaults.Estimate),
				Risk:       nonEmpty(args.Risk, cfg.Defaults.Risk),
				Area:       nonEmpty(args.Area, cfg.Defaults.Area),
				Ready:      args.Ready,
				CapturedBy: agentID,
			})
			if err != nil {
				return nil, fmt.Errorf("new item: %w", err)
			}
			parsed, err := items.Parse(path)
			if err != nil {
				return nil, fmt.Errorf("parse: %w", err)
			}
			if err := items.Persist(ctx, db, repoID, parsed, false); err != nil {
				return nil, fmt.Errorf("persist: %w", err)
			}
			return map[string]any{
				"id":     parsed.ID,
				"status": parsed.Status,
				"path":   path,
			}, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_accept",
		Description: "Promote captured items to open after Definition of Ready passes.",
		InputSchema: json.RawMessage(schemaAccept),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				IDs []string `json:"ids"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			agentID, _ := identity.AgentID()
			type rejection struct {
				ID         string               `json:"id"`
				Violations []items.DoRViolation `json:"violations,omitempty"`
				Error      string               `json:"error,omitempty"`
			}
			accepted := []string{}
			rejected := []rejection{}
			for _, id := range args.IDs {
				err := items.Promote(ctx, db, repoID, id, agentID)
				if err == nil {
					accepted = append(accepted, id)
					continue
				}
				var dorErr *items.DoRError
				if errors.As(err, &dorErr) {
					rejected = append(rejected, rejection{ID: id, Violations: dorErr.Violations})
					continue
				}
				rejected = append(rejected, rejection{ID: id, Error: err.Error()})
			}
			return map[string]any{"accepted": accepted, "rejected": rejected}, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_ready",
		Description: "Lint Definition of Ready for captured items; auto-promote those that pass when promote=true (default).",
		InputSchema: json.RawMessage(schemaReady),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			args := struct {
				IDs     []string `json:"ids"`
				Promote *bool    `json:"promote"`
				AgentID string   `json:"agent_id"`
			}{}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			promote := true
			if args.Promote != nil {
				promote = *args.Promote
			}
			acceptedBy := args.AgentID
			if acceptedBy == "" {
				acceptedBy, _ = identity.AgentID()
			}
			return Ready(ctx, ReadyArgs{
				DB:         db,
				RepoID:     repoID,
				AcceptedBy: acceptedBy,
				IDs:        args.IDs,
				Promote:    promote,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_inbox",
		Description: "List captured (inbox) items, optionally filtered by mine/ready_only/parent_spec.",
		InputSchema: json.RawMessage(schemaInbox),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				Mine       bool   `json:"mine"`
				ReadyOnly  bool   `json:"ready_only"`
				ParentSpec string `json:"parent_spec"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			rows, err := queryCapturedItems(ctx, db, repoID)
			if err != nil {
				return nil, fmt.Errorf("query inbox: %w", err)
			}
			var me string
			if args.Mine {
				me, _ = identity.AgentID()
			}
			type entry struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				CapturedBy string `json:"captured_by,omitempty"`
				CapturedAt int64  `json:"captured_at,omitempty"`
				ParentSpec string `json:"parent_spec,omitempty"`
				DoRPass    bool   `json:"dor_pass"`
			}
			out := []entry{}
			for _, r := range rows {
				if args.Mine && r.CapturedBy != me {
					continue
				}
				it, err := items.Parse(r.Path)
				if err != nil {
					continue
				}
				if args.ParentSpec != "" && it.ParentSpec != args.ParentSpec {
					continue
				}
				dorPass := len(items.DoRCheck(it)) == 0
				if args.ReadyOnly && !dorPass {
					continue
				}
				out = append(out, entry{
					ID:         r.ID,
					Title:      it.Title,
					CapturedBy: r.CapturedBy,
					CapturedAt: r.CapturedAt,
					ParentSpec: it.ParentSpec,
					DoRPass:    dorPass,
				})
			}
			return map[string]any{"items": out}, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_refine",
		Description: "Send an accepted item back to refinement: write reviewer comments under ## Reviewer feedback and flip status to needs-refinement.",
		InputSchema: json.RawMessage(schemaRefine),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				ItemID   string `json:"item_id"`
				Comments string `json:"comments"`
				AgentID  string `json:"agent_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			return Refine(ctx, RefineArgs{
				DB: db, RepoID: repoID,
				ItemID: args.ItemID, Comments: args.Comments,
			})
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_reject",
		Description: "Reject captured items (delete file + write to .squad/rejected.log).",
		InputSchema: json.RawMessage(schemaReject),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				IDs    []string `json:"ids"`
				Reason string   `json:"reason"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			if strings.TrimSpace(args.Reason) == "" {
				return nil, fmt.Errorf("reason required")
			}
			agentID, _ := identity.AgentID()
			squadDir := filepath.Join(repoRoot, ".squad")
			type refusal struct {
				ID    string `json:"id"`
				Error string `json:"error"`
			}
			deleted := []string{}
			refused := []refusal{}
			for _, id := range args.IDs {
				err := items.Reject(ctx, db, repoID, id, args.Reason, agentID, squadDir)
				if err == nil {
					deleted = append(deleted, id)
					continue
				}
				refused = append(refused, refusal{ID: id, Error: err.Error()})
			}
			return map[string]any{"deleted": deleted, "refused": refused}, nil
		},
	})

	srv.Register(mcp.Tool{
		Name:        "squad_decompose",
		Description: "Return the structured prompt for decomposing a spec into draft items. The calling agent runs the decomposition; squad just authors the prompt.",
		InputSchema: json.RawMessage(schemaDecompose),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				SpecName string `json:"spec_name"`
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return nil, err
				}
			}
			if err := requireRepo(repoRoot, repoID); err != nil {
				return nil, err
			}
			if strings.TrimSpace(args.SpecName) == "" {
				return nil, fmt.Errorf("spec_name required")
			}
			specPath := filepath.Join(repoRoot, ".squad", "specs", args.SpecName+".md")
			if _, err := os.Stat(specPath); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("spec %q not found at %s", args.SpecName, specPath)
				}
				return nil, fmt.Errorf("stat spec: %w", err)
			}
			var buf bytes.Buffer
			if err := decomposeTmpl.Execute(&buf, map[string]string{
				"SpecPath": specPath,
				"SpecName": args.SpecName,
			}); err != nil {
				return nil, fmt.Errorf("render prompt: %w", err)
			}
			return map[string]any{"prompt": buf.String()}, nil
		},
	})
}

func nonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
