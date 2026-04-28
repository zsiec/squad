package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

// AC#5: seed two agents with closes across two areas; file a new item in
// one area; assert the right agent gets the fyi and only the right agent.
func TestRunNew_FyisAreaTopCloser(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Setenv("SQUAD_AGENT", "agent-me")
	t.Chdir(dir)

	canonical, err := repo.Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	repoID, err := repo.IDFor(canonical)
	if err != nil {
		t.Fatal(err)
	}

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	// agent-alpha: 3 closes in area "chat" (qualifies) + 1 in "stats".
	for i, id := range []string{"OLD-1", "OLD-2", "OLD-3"} {
		seedDoneInArea(t, db, repoID, id, "chat", "agent-alpha", now-int64(i*60))
	}
	seedDoneInArea(t, db, repoID, "OLD-4", "stats", "agent-alpha", now-200)
	// agent-beta: 1 close in "chat", 4 in "stats" — top closer of "stats".
	seedDoneInArea(t, db, repoID, "OLD-5", "chat", "agent-beta", now-300)
	for i, id := range []string{"OLD-6", "OLD-7", "OLD-8", "OLD-9"} {
		seedDoneInArea(t, db, repoID, id, "stats", "agent-beta", now-int64(400+i*60))
	}
	_ = db.Close()

	var stdout bytes.Buffer
	code := runNew([]string{"feat", "Area routing"}, &stdout, items.Options{Area: "chat"})
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout.String())
	}

	db, err = store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.QueryContext(context.Background(), `
		SELECT kind, mentions, body FROM messages WHERE repo_id=? ORDER BY id`, repoID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type msg struct{ kind, mentions, body string }
	var got []msg
	for rows.Next() {
		var m msg
		if err := rows.Scan(&m.kind, &m.mentions, &m.body); err != nil {
			t.Fatal(err)
		}
		got = append(got, m)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 fyi, got %d (msgs=%+v)", len(got), got)
	}
	if got[0].kind != "fyi" {
		t.Errorf("kind=%q want fyi", got[0].kind)
	}
	// area="chat" → top closer is agent-alpha (3 closes). agent-beta has
	// only 1 close in chat — must NOT be mentioned.
	if !strings.Contains(got[0].mentions, "agent-alpha") {
		t.Errorf("mentions=%q want to contain agent-alpha", got[0].mentions)
	}
	if strings.Contains(got[0].mentions, "agent-beta") {
		t.Errorf("mentions=%q should not contain agent-beta (top closer of stats, not chat)", got[0].mentions)
	}
}

func TestRunNew_NoFyiWhenSelfIsTopCloser(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Setenv("SQUAD_AGENT", "agent-me")
	t.Chdir(dir)

	canonical, _ := repo.Discover(dir)
	repoID, _ := repo.IDFor(canonical)
	db, _ := store.OpenDefault()
	now := time.Now().Unix()
	for i, id := range []string{"OLD-1", "OLD-2", "OLD-3"} {
		seedDoneInArea(t, db, repoID, id, "chat", "agent-me", now-int64(i*60))
	}
	_ = db.Close()

	var stdout bytes.Buffer
	if code := runNew([]string{"feat", "Self routing"}, &stdout, items.Options{Area: "chat"}); code != 0 {
		t.Fatalf("exit=%d", code)
	}

	db, _ = store.OpenDefault()
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE repo_id=?`, repoID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 messages (self is top closer), got %d", n)
	}
}
