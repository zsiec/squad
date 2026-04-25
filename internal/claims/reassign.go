package claims

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *Store) Reassign(ctx context.Context, itemID, byAgent, toAgent string) error {
	toAgent = strings.TrimPrefix(toAgent, "@")
	if toAgent == "" {
		return fmt.Errorf("reassign: --to is required")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.releaseInTx(ctx, tx, itemID, byAgent, "released"); err != nil {
			return err
		}
		now := s.nowUnix()
		body := fmt.Sprintf("@%s reassigning %s to you", toAgent, itemID)
		return postSystemMessage(ctx, tx, s.repoID, now, byAgent, "global", "say", body, []string{toAgent}, "normal")
	})
}
