package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/store"
)

func newWhoamiCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Print the agent id this session resolves to",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.EnsureHome(); err != nil {
				return err
			}
			id, err := identity.AgentID()
			if err != nil {
				return err
			}
			if !asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), id)
				return nil
			}
			out := map[string]any{"id": id}
			if err := annotateWhoamiFromDB(out, id); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
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

func annotateWhoamiFromDB(out map[string]any, agentID string) error {
	db, err := store.OpenDefault()
	if err != nil {
		return nil
	}
	defer db.Close()

	var lastTick sql.NullInt64
	if err := db.QueryRow(`SELECT last_tick_at FROM agents WHERE id = ? LIMIT 1`, agentID).Scan(&lastTick); err == nil && lastTick.Valid {
		out["last_tick_at"] = lastTick.Int64
	}

	var itemID sql.NullString
	var lastTouch sql.NullInt64
	var intent sql.NullString
	if err := db.QueryRow(`SELECT item_id, last_touch, COALESCE(intent,'') FROM claims WHERE agent_id = ? LIMIT 1`, agentID).Scan(&itemID, &lastTouch, &intent); err == nil {
		if itemID.Valid {
			out["item_id"] = itemID.String
		}
		if lastTouch.Valid {
			out["last_touch"] = lastTouch.Int64
		}
		if intent.Valid && intent.String != "" {
			out["intent"] = intent.String
		}
	}
	return nil
}
