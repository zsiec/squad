package scaffold

import (
	"context"

	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

type Registration struct {
	RepoID string
}

// BootstrapAndRegister opens (or creates) the global DB, derives a stable
// repo_id, and upserts the repo row. Idempotent on re-run.
func BootstrapAndRegister(repoRoot, remoteURL, projectName string) (Registration, error) {
	db, err := store.OpenDefault()
	if err != nil {
		return Registration{}, err
	}
	defer db.Close()

	id, err := repo.RegisterRepo(context.Background(), db, repoRoot, remoteURL, projectName)
	if err != nil {
		return Registration{}, err
	}
	return Registration{RepoID: id}, nil
}
