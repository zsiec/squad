package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newAnswerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "answer <msg-id> <message>",
		Short: "Answer a previous message by id",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runAnswerBody(ctx, bc.chat, bc.agentID, args); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	return cmd
}

// AnswerArgs is the input for Answer.
type AnswerArgs struct {
	Chat    *chat.Chat
	AgentID string
	Ref     int64
	Body    string
}

// AnswerResult reports a successful answer delivery.
type AnswerResult struct {
	AgentID string `json:"agent_id"`
	Ref     int64  `json:"ref"`
}

// Answer replies to a previous message by id.
func Answer(ctx context.Context, args AnswerArgs) (*AnswerResult, error) {
	if args.Ref <= 0 {
		return nil, fmt.Errorf("answer: ref must be a positive message id")
	}
	if args.Body == "" {
		return nil, fmt.Errorf("answer: body required")
	}
	if err := args.Chat.Answer(ctx, args.AgentID, args.Ref, args.Body); err != nil {
		return nil, err
	}
	return &AnswerResult{AgentID: args.AgentID, Ref: args.Ref}, nil
}

func runAnswerBody(ctx context.Context, c *chat.Chat, agentID string, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: squad answer <msg-id> <message>")
		return 4
	}
	ref, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || ref <= 0 {
		fmt.Fprintf(os.Stderr, "invalid msg-id: %q\n", args[0])
		return 4
	}
	if _, err := Answer(ctx, AnswerArgs{
		Chat:    c,
		AgentID: agentID,
		Ref:     ref,
		Body:    strings.Join(args[1:], " "),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}
