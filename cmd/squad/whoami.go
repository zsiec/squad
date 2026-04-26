package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

type WhoamiArgs struct {
	DB *sql.DB `json:"-"`
	// RepoID scopes the claim lookup. When empty, no claim is reported —
	// preventing the cross-repo leak where an MCP whoami call from outside a
	// squad repo would echo a claim held in some other repo. Pass the
	// caller's discovered repo id when it exists; pass "" when no .squad/
	// is reachable so the response only carries the agent identity.
	RepoID string `json:"-"`
}

type WhoamiResult struct {
	AgentID    string `json:"id"`
	LastTickAt int64  `json:"last_tick_at,omitempty"`
	ItemID     string `json:"item_id,omitempty"`
	Intent     string `json:"intent,omitempty"`
	LastTouch  int64  `json:"last_touch,omitempty"`
}

func Whoami(_ context.Context, args WhoamiArgs) (*WhoamiResult, error) {
	if err := store.EnsureHome(); err != nil {
		return nil, err
	}
	id, err := identity.AgentID()
	if err != nil {
		return nil, err
	}
	res := &WhoamiResult{AgentID: id}
	db := args.DB
	if db == nil {
		opened, oerr := store.OpenDefault()
		if oerr != nil {
			return res, nil
		}
		defer opened.Close()
		db = opened
	}
	if err := annotateWhoamiFromDB(db, res, id, args.RepoID); err != nil && !errors.Is(err, sql.ErrNoRows) {
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
			res, err := Whoami(cmd.Context(), WhoamiArgs{RepoID: discoverRepoIDForWhoami()})
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

// discoverRepoIDForWhoami best-effort resolves the repo id for the caller's
// current directory. Returns "" when no .squad/ is reachable, which suppresses
// the claim fields in the response — exactly what we want when whoami is run
// outside any squad workspace.
func discoverRepoIDForWhoami() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	root, err := repo.Discover(wd)
	if err != nil {
		return ""
	}
	id, err := repo.IDFor(root)
	if err != nil {
		return ""
	}
	return id
}

func annotateWhoamiFromDB(db *sql.DB, res *WhoamiResult, agentID, repoID string) error {
	var lastTick sql.NullInt64
	if err := db.QueryRow(`SELECT last_tick_at FROM agents WHERE id = ? LIMIT 1`, agentID).Scan(&lastTick); err == nil && lastTick.Valid {
		res.LastTickAt = lastTick.Int64
	}

	// A claim is meaningful only inside the repo it was made against.
	// Without repoID we cannot tell whether the row in `claims` belongs to
	// the caller's current workspace, so we leave the claim fields empty.
	if repoID == "" {
		return nil
	}

	var itemID sql.NullString
	var lastTouch sql.NullInt64
	var intent sql.NullString
	if err := db.QueryRow(`SELECT item_id, last_touch, COALESCE(intent,'') FROM claims WHERE agent_id = ? AND repo_id = ? LIMIT 1`, agentID, repoID).Scan(&itemID, &lastTouch, &intent); err == nil {
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
