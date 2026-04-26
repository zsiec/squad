//go:build dogfood

package stats

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Dogfood: open the user's actual ~/.squad/global.db (read-only) and
// confirm Compute returns no error and a sensible Snapshot. Excluded from the
// default build; run with `go test -tags dogfood ./internal/stats/`.
func TestComputeAgainstRealDB(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	path := filepath.Join(home, ".squad", "global.db")
	if _, err := os.Stat(path); err != nil {
		t.Skip("no real db")
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Skip(err)
	}
	defer db.Close()
	snap, err := Compute(context.Background(), db, ComputeOpts{
		RepoID: os.Getenv("SQUAD_DOGFOOD_REPO_ID"),
		Now:    time.Now(),
		Window: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if snap.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("schema_version: %d", snap.SchemaVersion)
	}
}
