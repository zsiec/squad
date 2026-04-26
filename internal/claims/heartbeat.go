package claims

import "context"

func (s *Store) TouchClaim(ctx context.Context, agentID string) error {
	now := s.nowUnix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE claims SET last_touch = ? WHERE repo_id = ? AND agent_id = ?
	`, now, s.repoID, agentID)
	return err
}
