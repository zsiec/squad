package workspace

import (
	"context"
	"database/sql"
)

type ItemRef struct {
	RepoID, ID, Priority, Title, Estimate, Status string
}

type ItemSource interface {
	For(ctx context.Context, repoID string) ([]ItemRef, error)
}

type dbItemSource struct{ db *sql.DB }

func (s dbItemSource) For(ctx context.Context, repoID string) ([]ItemRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id, COALESCE(priority,''), COALESCE(title,''), COALESCE(estimate,''), COALESCE(status,'') FROM items WHERE repo_id = ?`,
		repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ItemRef
	for rows.Next() {
		r := ItemRef{RepoID: repoID}
		if err := rows.Scan(&r.ID, &r.Priority, &r.Title, &r.Estimate, &r.Status); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

type staticItemSource []ItemRef

func (s staticItemSource) For(_ context.Context, repoID string) ([]ItemRef, error) {
	var out []ItemRef
	for _, r := range s {
		if r.RepoID == repoID {
			out = append(out, r)
		}
	}
	return out, nil
}
