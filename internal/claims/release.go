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
