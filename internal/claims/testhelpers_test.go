package claims

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func newTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	s := New(db, "repo-test", func() time.Time { return now })
	return s, db
}
