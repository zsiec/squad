package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/touch"
)

// TouchArgs is the input for Touch.
type TouchArgs struct {
	Tracker *touch.Tracker
	AgentID string
	ItemID  string
	Paths   []string
}

// TouchedFile records each declared file plus any peer agents already
// touching it.
type TouchedFile struct {
	Path      string   `json:"path"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// TouchResult lists every path touched in this call.
type TouchResult struct {
	ItemID string        `json:"item_id"`
	Files  []TouchedFile `json:"files"`
}

// Touch declares the agent is editing the given paths under itemID, and
// reports any peer agents already touching the same files.
func Touch(ctx context.Context, args TouchArgs) (*TouchResult, error) {
	if args.ItemID == "" {
		return nil, fmt.Errorf("touch: item id required")
	}
	if len(args.Paths) == 0 {
		return nil, fmt.Errorf("touch: at least one path required")
	}
	out := make([]TouchedFile, 0, len(args.Paths))
	for _, p := range args.Paths {
		conflicts, err := args.Tracker.Add(ctx, args.AgentID, args.ItemID, p)
		if err != nil {
			return nil, err
		}
		out = append(out, TouchedFile{Path: p, Conflicts: conflicts})
	}
	return &TouchResult{ItemID: args.ItemID, Files: out}, nil
}

// UntouchArgs is the input for Untouch. Empty Paths releases everything
// the agent is touching.
type UntouchArgs struct {
	Tracker *touch.Tracker
	AgentID string
	Paths   []string
}

// UntouchResult reports how many touches were released.
type UntouchResult struct {
	Released int      `json:"released"`
	Paths    []string `json:"paths,omitempty"`
}

// Untouch releases the named paths (or all of the agent's touches when
// Paths is empty).
func Untouch(ctx context.Context, args UntouchArgs) (*UntouchResult, error) {
	if len(args.Paths) == 0 {
		n, err := args.Tracker.ReleaseAll(ctx, args.AgentID)
		if err != nil {
			return nil, err
		}
		return &UntouchResult{Released: n}, nil
	}
	for _, p := range args.Paths {
		if err := args.Tracker.Release(ctx, args.AgentID, p); err != nil {
			return nil, err
		}
	}
	return &UntouchResult{Released: len(args.Paths), Paths: args.Paths}, nil
}

// TouchesListOthersArgs is the input for TouchesListOthers.
type TouchesListOthersArgs struct {
	Tracker *touch.Tracker
	AgentID string
}

// TouchesListOthersResult lists active touches by peer agents.
type TouchesListOthersResult struct {
	Touches []touch.ActiveTouch `json:"touches"`
}

// TouchesListOthers returns active file touches held by agents other than
// the caller — used by the pre-edit-touch-check hook.
func TouchesListOthers(ctx context.Context, args TouchesListOthersArgs) (*TouchesListOthersResult, error) {
	rows, err := args.Tracker.ListOthers(ctx, args.AgentID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []touch.ActiveTouch{}
	}
	return &TouchesListOthersResult{Touches: rows}, nil
}

func newTouchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "touches",
		Short: "Query active file touches across the repo",
	}
	cmd.AddCommand(newTouchesListOthersCmd())
	return cmd
}

func newTouchesListOthersCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list-others",
		Short: "List active file touches held by agents OTHER than the current one",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			tr := touch.New(bc.db, bc.repoID)
			rows, err := tr.ListOthers(ctx, bc.agentID)
			if err != nil {
				return err
			}
			if asJSON {
				if rows == nil {
					rows = []touch.ActiveTouch{}
				}
				b, err := json.Marshal(rows)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-22s  %s\n", r.AgentID, r.ItemID, r.Path)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit as JSON array")
	return cmd
}

func newTouchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "touch <ITEM-ID> <path>...",
		Short: "Declare files you are editing so peers see the overlap",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			itemID, paths := args[0], args[1:]
			tr := touch.New(bc.db, bc.repoID)
			for _, p := range paths {
				conflicts, err := tr.Add(ctx, bc.agentID, itemID, p)
				if err != nil {
					return err
				}
				if len(conflicts) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"WARNING: %s also touched by: %s\n", p, strings.Join(conflicts, ", "))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "touched %s\n", p)
			}
			return nil
		},
	}
}

func newUntouchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untouch [path]...",
		Short: "Release file touches; no paths releases all touches for this agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			tr := touch.New(bc.db, bc.repoID)
			if len(args) == 0 {
				n, err := tr.ReleaseAll(ctx, bc.agentID)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "released %d touch(es)\n", n)
				return nil
			}
			for _, p := range args {
				if err := tr.Release(ctx, bc.agentID, p); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "released %s\n", p)
			}
			return nil
		},
	}
}
