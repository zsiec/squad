package main

import (
	"context"

	"github.com/zsiec/squad/internal/items"
)

// AutoRefineApplyArgs is the input for AutoRefineApply.
type AutoRefineApplyArgs struct {
	SquadDir string `json:"-"`
	ItemID   string `json:"item_id"`
	NewBody  string `json:"new_body"`
	// Area is optional. When non-empty, the frontmatter `area` field is
	// rewritten to this value alongside the body — lets the auto-refine
	// flow heal items captured with the `<fill-in>` placeholder, which
	// the DoR area-set rule would otherwise reject.
	Area string `json:"area"`
}

// AutoRefineApplyResult is the success shape returned to the LLM caller.
// Body and file path are intentionally omitted: bodies are large and the
// dashboard re-fetches via the existing item-detail endpoint.
type AutoRefineApplyResult struct {
	OK            bool   `json:"ok"`
	ItemID        string `json:"item_id"`
	AutoRefinedAt int64  `json:"auto_refined_at"`
}

// AutoRefineApply is the CLI/MCP-shaped wrapper around items.AutoRefineApply.
// All bookkeeping (status guard, DoR check, atomic body rewrite, audit-field
// stamping) lives in the items package; this layer only adapts to the
// JSON-RPC arguments and re-parses the file once to surface the freshly-
// stamped auto_refined_at to the caller.
func AutoRefineApply(ctx context.Context, args AutoRefineApplyArgs) (*AutoRefineApplyResult, error) {
	if err := items.AutoRefineApply(args.SquadDir, args.ItemID, args.NewBody, args.Area, "claude"); err != nil {
		return nil, err
	}
	path, _, err := items.FindByID(args.SquadDir, args.ItemID)
	if err != nil {
		return nil, err
	}
	it, err := items.Parse(path)
	if err != nil {
		return nil, err
	}
	return &AutoRefineApplyResult{
		OK:            true,
		ItemID:        args.ItemID,
		AutoRefinedAt: it.AutoRefinedAt,
	}, nil
}
