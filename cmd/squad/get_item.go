package main

import (
	"context"

	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/items"
)

// ErrItemNotFound is returned by GetItem when no item file in ItemsDir matches
// the requested id. Re-exports claims.ErrItemNotFound so MCP callers and the
// CLI share a single sentinel.
var ErrItemNotFound = claims.ErrItemNotFound

type GetItemArgs struct {
	ItemsDir string `json:"items_dir"`
	ItemID   string `json:"item_id"`
}

func GetItem(_ context.Context, args GetItemArgs) (*items.Item, error) {
	path := findItemPath(args.ItemsDir, args.ItemID)
	if path == "" {
		return nil, ErrItemNotFound
	}
	it, err := items.Parse(path)
	if err != nil {
		return nil, err
	}
	return &it, nil
}
