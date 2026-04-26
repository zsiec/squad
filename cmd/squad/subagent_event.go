package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
	"github.com/zsiec/squad/internal/subagent"
)

type subagentHookPayload struct {
	HookEventName string `json:"hook_event_name"`
	AgentID       string `json:"agent_id"`
	AgentType     string `json:"agent_type"`
	SessionID     string `json:"session_id"`
}

func mapSubagentEventName(hookEventName string) string {
	switch hookEventName {
	case "SubagentStart":
		return "subagent_start"
	case "SubagentStop":
		return "subagent_stop"
	case "TaskCreated":
		return "task_created"
	case "TaskCompleted":
		return "task_completed"
	default:
		return ""
	}
}

func newSubagentEventCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "subagent-event",
		Hidden: true,
		Short:  "Hook entry: read SubagentStart/Stop or TaskCreated/Completed JSON from stdin and record it.",
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in := cmd.InOrStdin()
			if in == nil {
				in = os.Stdin
			}
			data, err := io.ReadAll(in)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad: subagent-event: read stdin: %v\n", err)
				return nil
			}
			if len(data) == 0 {
				return nil
			}
			var payload subagentHookPayload
			if err := json.Unmarshal(data, &payload); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad: subagent-event: parse: %v\n", err)
				return nil
			}
			eventName := mapSubagentEventName(payload.HookEventName)
			if eventName == "" {
				return nil
			}

			parentID, err := identity.AgentID()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad: subagent-event: agent id: %v\n", err)
				return nil
			}
			wd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad: subagent-event: cwd: %v\n", err)
				return nil
			}
			root, err := repo.Discover(wd)
			if err != nil {
				return nil
			}
			repoID, err := repo.IDFor(root)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad: subagent-event: repo id: %v\n", err)
				return nil
			}
			db, err := store.OpenDefault()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad: subagent-event: open store: %v\n", err)
				return nil
			}
			defer db.Close()

			rec := subagent.New(db, repoID, nil)
			if err := rec.Record(cmd.Context(), subagent.Event{
				AgentID:    parentID,
				SubagentID: payload.AgentID,
				Type:       payload.AgentType,
				EventName:  eventName,
			}); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad: subagent-event: record: %v\n", err)
			}
			return nil
		},
	}
}
