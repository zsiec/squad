package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
}

func nonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
