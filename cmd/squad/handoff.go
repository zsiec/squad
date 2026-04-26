package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
)

func newHandoffCmd() *cobra.Command {
	var (
		shipped   []string
		inflight  []string
		surprised []string
		unblocks  []string
		note      string
		stdinIn   bool
	)
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: "Post a handoff brief and release any held claims",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			h := chat.HandoffBody{
				Shipped:     shipped,
				InFlight:    inflight,
				SurprisedBy: surprised,
				Unblocks:    unblocks,
				Note:        note,
			}
			if stdinIn {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				extra := strings.TrimSpace(string(data))
				switch {
				case h.Note != "" && extra != "":
					h.Note = h.Note + "\n\n" + extra
				case extra != "":
					h.Note = extra
				}
			}
			if code := runHandoffBody(ctx, bc.chat, bc.store, bc.agentID, h); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&shipped, "shipped", nil, "item shipped this session (repeatable)")
	cmd.Flags().StringSliceVar(&inflight, "in-flight", nil, "item still in flight (repeatable)")
	cmd.Flags().StringSliceVar(&surprised, "surprised-by", nil, "surprising finding (repeatable)")
	cmd.Flags().StringSliceVar(&unblocks, "unblocks", nil, "item unblocked by this work (repeatable)")
	cmd.Flags().StringVar(&note, "note", "", "free-form note")
	cmd.Flags().BoolVar(&stdinIn, "stdin", false, "append note body from stdin")
	return cmd
}

// HandoffArgs is the input for Handoff. At least one of Shipped, InFlight,
// SurprisedBy, Unblocks, or Note must be non-empty — an empty handoff has
// no body and is rejected.
type HandoffArgs struct {
	Chat        *chat.Chat
	ClaimStore  *claims.Store
	AgentID     string
	Shipped     []string
	InFlight    []string
	SurprisedBy []string
	Unblocks    []string
	Note        string
}

// HandoffResult reports the outcome of a successful handoff: the summary
// chat surfaced, plus the count of claims released.
type HandoffResult struct {
	AgentID        string `json:"agent_id"`
	Summary        string `json:"summary"`
	ClaimsReleased int    `json:"claims_released"`
}

// ErrHandoffEmpty is returned when no handoff fields are populated.
var ErrHandoffEmpty = errors.New("handoff: at least one of shipped, in-flight, surprised-by, unblocks, note required")

// Handoff posts a handoff brief to chat and releases every claim the agent
// currently holds. Pure of writers — callers print the result themselves.
func Handoff(ctx context.Context, args HandoffArgs) (*HandoffResult, error) {
	h := chat.HandoffBody{
		Shipped:     args.Shipped,
		InFlight:    args.InFlight,
		SurprisedBy: args.SurprisedBy,
		Unblocks:    args.Unblocks,
		Note:        args.Note,
	}
	if h.Empty() {
		return nil, ErrHandoffEmpty
	}
	if err := args.Chat.PostHandoff(ctx, args.AgentID, h); err != nil {
		return nil, err
	}
	released, err := args.ClaimStore.ReleaseAllCount(ctx, args.AgentID, "handoff")
	if err != nil {
		return nil, err
	}
	return &HandoffResult{
		AgentID:        args.AgentID,
		Summary:        h.Summary(),
		ClaimsReleased: released,
	}, nil
}

func runHandoffBody(ctx context.Context, c *chat.Chat, claimStore *claims.Store, agentID string, h chat.HandoffBody) int {
	res, err := Handoff(ctx, HandoffArgs{
		Chat:        c,
		ClaimStore:  claimStore,
		AgentID:     agentID,
		Shipped:     h.Shipped,
		InFlight:    h.InFlight,
		SurprisedBy: h.SurprisedBy,
		Unblocks:    h.Unblocks,
		Note:        h.Note,
	})
	if errors.Is(err, ErrHandoffEmpty) {
		fmt.Fprintln(os.Stderr, "handoff requires at least one --shipped / --in-flight / --surprised-by / --unblocks / --note (or --stdin)")
		return 4
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Printf("handoff posted by %s (%s)\n", res.AgentID, res.Summary)
	return 0
}
