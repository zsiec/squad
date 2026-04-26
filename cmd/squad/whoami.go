package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/store"
)

type WhoamiArgs struct{}

type WhoamiResult struct {
	AgentID    string `json:"id"`
	LastTickAt int64  `json:"last_tick_at,omitempty"`
	ItemID     string `json:"item_id,omitempty"`
	Intent     string `json:"intent,omitempty"`
	LastTouch  int64  `json:"last_touch,omitempty"`
}

func Whoami(_ context.Context, _ WhoamiArgs) (*WhoamiResult, error) {
	if err := store.EnsureHome(); err != nil {
		return nil, err
	}
	id, err := identity.AgentID()
	if err != nil {
		return nil, err
	}
	res := &WhoamiResult{AgentID: id}
	if err := annotateWhoamiFromDB(res, id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return res, nil
}

func newWhoamiCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Print the agent id this session resolves to",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := Whoami(cmd.Context(), WhoamiArgs{})
			if err != nil {
				return err
			}
			if !asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), res.AgentID)
				return nil
			}
			out := map[string]any{"id": res.AgentID}
			if res.LastTickAt != 0 {
				out["last_tick_at"] = res.LastTickAt
			}
			if res.ItemID != "" {
				out["item_id"] = res.ItemID
			}
			if res.Intent != "" {
				out["intent"] = res.Intent
			}
			if res.LastTouch != 0 {
				out["last_touch"] = res.LastTouch
			}
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit a JSON object with id, last_tick_at, item_id, intent, last_touch")
	return cmd
}

func annotateWhoamiFromDB(res *WhoamiResult, agentID string) error {
	db, err := store.OpenDefault()
	if err != nil {
		return nil
	}
	defer db.Close()

	var lastTick sql.NullInt64
	if err := db.QueryRow(`SELECT last_tick_at FROM agents WHERE id = ? LIMIT 1`, agentID).Scan(&lastTick); err == nil && lastTick.Valid {
		res.LastTickAt = lastTick.Int64
	}

	var itemID sql.NullString
	var lastTouch sql.NullInt64
	var intent sql.NullString
	if err := db.QueryRow(`SELECT item_id, last_touch, COALESCE(intent,'') FROM claims WHERE agent_id = ? LIMIT 1`, agentID).Scan(&itemID, &lastTouch, &intent); err == nil {
		if itemID.Valid {
			res.ItemID = itemID.String
		}
		if lastTouch.Valid {
			res.LastTouch = lastTouch.Int64
		}
		if intent.Valid && intent.String != "" {
			res.Intent = intent.String
		}
	}
	return nil
}
