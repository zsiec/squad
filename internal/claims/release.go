package claims

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) Release(ctx context.Context, itemID, agentID, outcome string) error {
	if outcome == "" {
		outcome = "released"
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.releaseInTx(ctx, tx, itemID, agentID, outcome)
	})
}

func (s *Store) releaseInTx(ctx context.Context, tx *sql.Tx, itemID, agentID, outcome string) error {
	now := s.nowUnix()

	var holder string
	var claimedAt int64
	row := tx.QueryRowContext(ctx, `SELECT agent_id, claimed_at FROM claims WHERE item_id=?`, itemID)
	if err := row.Scan(&holder, &claimedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotClaimed
		}
		return fmt.Errorf("lookup claim: %w", err)
	}
	if holder != agentID {
		return ErrNotYours
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		VALUES (?, ?, ?, ?, ?, ?)
	`, s.repoID, itemID, agentID, claimedAt, now, outcome); err != nil {
		return fmt.Errorf("history insert: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM claims WHERE item_id=?`, itemID); err != nil {
		return fmt.Errorf("delete claim: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE touches SET released_at = ?
		WHERE item_id = ? AND agent_id = ? AND released_at IS NULL
	`, now, itemID, agentID); err != nil {
		return fmt.Errorf("release touches: %w", err)
	}

	body := fmt.Sprintf("released %s (%s)", itemID, outcome)
	if err := postSystemMessage(ctx, tx, s.repoID, now, agentID, "global", "release", body, nil, "normal"); err != nil {
		return err
	}
	return postSystemMessage(ctx, tx, s.repoID, now, agentID, itemID, "release", body, nil, "normal")
}

// ReleaseAll releases every active claim held by agentID with the given outcome.
// No-op when the agent holds no claims. Used by `squad handoff`.
func (s *Store) ReleaseAll(ctx context.Context, agentID, outcome string) error {
	_, err := s.ReleaseAllCount(ctx, agentID, outcome)
	return err
}

// ReleaseAllCount is like ReleaseAll but returns how many claims it
// released. Used by handoff's MCP/CLI surface to report the count back.
func (s *Store) ReleaseAllCount(ctx context.Context, agentID, outcome string) (int, error) {
	if outcome == "" {
		outcome = "released"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT item_id FROM claims WHERE agent_id = ?`, agentID)
	if err != nil {
		return 0, fmt.Errorf("list claims: %w", err)
	}
	var items []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, id)
	}
	rows.Close()
	for _, id := range items {
		if err := s.Release(ctx, id, agentID, outcome); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}
