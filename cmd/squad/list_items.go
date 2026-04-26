package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zsiec/squad/internal/items"
)

// Agent filtering joins against the claims table; DB+RepoID must be set when
// Agent is non-empty. The error path remains as a fallback when callers can
// only provide a filesystem-rooted view.
type ListItemsArgs struct {
	ItemsDir string `json:"items_dir"`
	DoneDir  string `json:"done_dir"`

	DB     *sql.DB `json:"-"`
	RepoID string  `json:"repo_id,omitempty"`

	Status   string `json:"status,omitempty"`
	Type     string `json:"type,omitempty"`
	Priority string `json:"priority,omitempty"`
	Agent    string `json:"agent,omitempty"`

	Limit int `json:"limit,omitempty"`
}

type ListItemsRow struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
	Agent    string `json:"agent,omitempty"`
}

type ListItemsResult struct {
	Items []ListItemsRow `json:"items"`
	Count int            `json:"count"`
}

const (
	listItemsDefaultLimit = 50
	listItemsMaxLimit     = 200
)

func ListItems(ctx context.Context, args ListItemsArgs) (*ListItemsResult, error) {
	if args.Agent != "" && (args.DB == nil || args.RepoID == "") {
		return nil, fmt.Errorf("list_items: agent filter requires DB and RepoID")
	}

	holders, err := claimHolders(ctx, args.DB, args.RepoID)
	if err != nil {
		return nil, err
	}

	includeDone := args.Status == "" || args.Status == "done"
	includeActive := args.Status != "done"

	var collected []items.Item
	if includeActive && args.ItemsDir != "" {
		got, err := readItemsDir(args.ItemsDir)
		if err != nil {
			return nil, err
		}
		collected = append(collected, got...)
	}
	if includeDone && args.DoneDir != "" {
		got, err := readItemsDir(args.DoneDir)
		if err != nil {
			return nil, err
		}
		for _, it := range got {
			if it.Status == "" {
				it.Status = "done"
			}
			collected = append(collected, it)
		}
	}

	filtered := collected[:0]
	for _, it := range collected {
		if args.Status != "" && it.Status != args.Status {
			continue
		}
		if args.Type != "" && it.Type != args.Type {
			continue
		}
		if args.Priority != "" && it.Priority != args.Priority {
			continue
		}
		if args.Agent != "" && holders[it.ID] != args.Agent {
			continue
		}
		filtered = append(filtered, it)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		pi, pj := priorityRank(filtered[i].Priority), priorityRank(filtered[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return filtered[i].Created < filtered[j].Created
	})

	limit := args.Limit
	if limit <= 0 {
		limit = listItemsDefaultLimit
	}
	if limit > listItemsMaxLimit {
		limit = listItemsMaxLimit
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	rows := make([]ListItemsRow, 0, len(filtered))
	for _, it := range filtered {
		rows = append(rows, ListItemsRow{
			ID:       it.ID,
			Title:    it.Title,
			Status:   it.Status,
			Type:     it.Type,
			Priority: it.Priority,
			Agent:    holders[it.ID],
		})
	}
	return &ListItemsResult{Items: rows, Count: len(rows)}, nil
}

func claimHolders(ctx context.Context, db *sql.DB, repoID string) (map[string]string, error) {
	out := map[string]string{}
	if db == nil || repoID == "" {
		return out, nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT item_id, agent_id FROM claims WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, agent string
		if err := rows.Scan(&id, &agent); err != nil {
			return nil, err
		}
		out[id] = agent
	}
	return out, rows.Err()
}

func readItemsDir(dir string) ([]items.Item, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []items.Item
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		it, perr := items.Parse(filepath.Join(dir, e.Name()))
		if perr != nil {
			continue
		}
		out = append(out, it)
	}
	return out, nil
}

func priorityRank(p string) int {
	switch p {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 99
	}
}
