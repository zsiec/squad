package hygiene

import (
	"context"
	"path/filepath"
	"testing"
)

func TestArchive_MovesOldMessages(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()

	// old message at ts=100, recent at ts=10000
	if _, err := db.Exec(`
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, priority)
		VALUES ('repo-test', 100, 'agent-a', 'global', 'say', 'old msg', 'normal')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, priority)
		VALUES ('repo-test', 10000, 'agent-a', 'global', 'say', 'recent', 'normal')
	`); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	moved, archivePath, err := Archive(ctx, db, "repo-test", dir, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved=%d want 1", moved)
	}
	if _, err := filepath.Abs(archivePath); err != nil {
		t.Fatal(err)
	}
	var c int
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE body='old msg'`).Scan(&c)
	if c != 0 {
		t.Fatalf("old messages still present: %d", c)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE body='recent'`).Scan(&c)
	if c != 1 {
		t.Fatalf("recent disappeared: %d", c)
	}
}

func TestArchive_IdempotentOnReRun(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	cutoff := int64(5000)
	if _, _, err := Archive(ctx, db, "repo-test", dir, cutoff); err != nil {
		t.Fatal(err)
	}
	moved, _, err := Archive(ctx, db, "repo-test", dir, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 0 {
		t.Fatalf("re-run moved=%d want 0", moved)
	}
}
