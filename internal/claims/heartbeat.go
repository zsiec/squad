package claims

import "context"

func (s *Store) TouchClaim(ctx context.Context, agentID string) error {
	now := s.nowUnix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE claims SET last_touch = ? WHERE agent_id = ?
	`, now, agentID)
	return err
}
