package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/notify"
	"github.com/zsiec/squad/internal/store"
)

func TestRealtimeChat_SenderWakesTwoListeners(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "global.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	const repoID = "repo-rt"
	clock := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	for _, id := range []string{"agent-a", "agent-b", "agent-sender"} {
		if _, err := db.Exec(`
			INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
			VALUES (?, ?, ?, '/tmp/wt', 1, ?, ?, 'active')
		`, id, repoID, id, clock.Unix(), clock.Unix()); err != nil {
			t.Fatal(err)
		}
	}

	registry := notify.NewRegistry(db)
	chatA := chat.New(db, repoID)
	chatB := chat.New(db, repoID)
	chatSender := chat.New(db, repoID)
	chatSender.SetNotifier(func(ctx context.Context, repoID string) {
		_ = notify.Wake(ctx, registry, repoID, 100*time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stdoutA, stdoutB bytes.Buffer
	var wg sync.WaitGroup
	results := make(chan int, 2)

	args := func(inst string) listenArgs {
		return listenArgs{Instance: inst, FallbackInt: 100 * time.Millisecond, MaxLifetime: 4 * time.Second}
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		results <- runListen(ctx, chatA, db, "agent-a", repoID, registry, args("inst-a"), &stdoutA)
	}()
	go func() {
		defer wg.Done()
		results <- runListen(ctx, chatB, db, "agent-b", repoID, registry, args("inst-b"), &stdoutB)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		eps, _ := registry.LookupRepo(ctx, repoID)
		if len(eps) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := chatSender.Post(ctx, chat.PostRequest{
		AgentID: "agent-sender", Thread: chat.ThreadGlobal, Kind: chat.KindSay,
		Body: "@agent-a @agent-b live wake test",
	}); err != nil {
		t.Fatalf("post: %v", err)
	}

	wg.Wait()
	close(results)
	codes := []int{}
	for c := range results {
		codes = append(codes, c)
	}
	if len(codes) != 2 || codes[0] != 2 || codes[1] != 2 {
		t.Fatalf("expected both listeners to exit 2 (decision-block), got %v", codes)
	}

	for _, out := range []string{stdoutA.String(), stdoutB.String()} {
		if !strings.Contains(out, `"decision":"block"`) {
			t.Errorf("listener stdout missing decision-block: %q", out)
		}
		if !strings.Contains(out, "live wake test") {
			t.Errorf("listener stdout missing message body: %q", out)
		}
	}

	eps, _ := registry.LookupRepo(ctx, repoID)
	if len(eps) != 0 {
		t.Fatalf("listeners must unregister on exit, got %+v", eps)
	}
}

func TestRealtimeChat_TickStillWorksAsFallback(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	if _, err := f.db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES ('peer', ?, 'P', '/tmp/wt', 1, 1, 1, 'active')
	`, f.repoID); err != nil {
		t.Fatal(err)
	}
	if err := f.chat.Post(ctx, chat.PostRequest{AgentID: "peer", Thread: chat.ThreadGlobal, Kind: chat.KindSay, Body: "@" + f.agentID + " hi"}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if code := runTickBody(ctx, f.chat, f.agentID, false, &stdout); code != 0 {
		t.Fatalf("tick exit=%d", code)
	}
	if !strings.Contains(stdout.String(), "hi") {
		t.Fatalf("tick fallback did not surface message: %q", stdout.String())
	}
}
