package claims

import (
	"context"
	"database/sql"
	"encoding/json"
)

func postSystemMessage(ctx context.Context, tx *sql.Tx, repoID string, ts int64, agentID, thread, kind, body string, mentions []string, priority string) error {
	if priority == "" {
		priority = "normal"
	}
	var mentionsJSON any
	if len(mentions) > 0 {
		b, err := json.Marshal(mentions)
		if err != nil {
			return err
		}
		mentionsJSON = string(b)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, repoID, ts, agentID, thread, kind, body, mentionsJSON, priority)
	return err
}
