package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/notify"
)

func TestRunListen_BlocksUntilWake(t *testing.T) {
	f := newChatFixture(t)
	registry := notify.NewRegistry(f.db)

	args := listenArgs{
		Instance:    "inst-test",
		FallbackInt: 50 * time.Millisecond,
		MaxLifetime: 2 * time.Second,
	}

	var stdout bytes.Buffer
	done := make(chan int, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		done <- runListen(ctx, f.chat, f.db, f.agentID, f.repoID, registry, args, &stdout)
	}()

	deadline := time.Now().Add(time.Second)
	var port int
	for time.Now().Before(deadline) {
		eps, _ := registry.LookupRepo(ctx, f.repoID)
		if len(eps) == 1 {
			port = eps[0].Port
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if port == 0 {
		t.Fatal("listen never registered an endpoint")
	}

	if err := chat.New(f.db, f.repoID).Post(ctx, chat.PostRequest{
		AgentID: "peer", Thread: chat.ThreadGlobal, Kind: chat.KindSay,
		Body: "@" + f.agentID + " ping",
	}); err != nil {
		t.Fatalf("post: %v", err)
	}
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 100*time.Millisecond)
	if err != nil {
		t.Fatalf("dial wake: %v", err)
	}
	_ = c.Close()

	select {
	case code := <-done:
		if code != 2 {
			t.Fatalf("expected exit code 2 (decision-block), got %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listen did not exit after wake")
	}

	out := stdout.String()
	var env struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, out)
	}
	if env.Decision != "block" {
		t.Fatalf("decision=%q want block", env.Decision)
	}
	if !strings.Contains(env.Reason, "ping") {
		t.Fatalf("reason missing message body: %q", env.Reason)
	}

	eps, _ := registry.LookupRepo(ctx, f.repoID)
	if len(eps) != 0 {
		t.Fatalf("listen must unregister on exit, found %+v", eps)
	}
}

// TestRunListen_FiresTimeBoxNudgeOnAsyncPoll covers FEAT-051: when the
// mailbox is empty but the held claim has crossed an unfired threshold,
// the listener returns exit 2 with the time-box body in the
// decision-block envelope, without waiting for a `squad tick` boundary.
func TestRunListen_FiresTimeBoxNudgeOnAsyncPoll(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	f := newChatFixture(t)
	registry := notify.NewRegistry(f.db)
	// 91m-old claim, no milestone — 90m threshold crossed, n90 NULL.
	claimAt := time.Now().Add(-91 * time.Minute)
	if _, err := f.db.Exec(
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, ?, ?, '', 0)`,
		f.repoID, "BUG-T1", f.agentID, claimAt.Unix(), claimAt.Unix(),
	); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	args := listenArgs{Instance: "inst-tb", FallbackInt: 30 * time.Millisecond, MaxLifetime: time.Second}
	var stdout bytes.Buffer
	code := runListen(context.Background(), f.chat, f.db, f.agentID, f.repoID, registry, args, &stdout)
	if code != 2 {
		t.Fatalf("expected exit 2 when threshold crossed, got %d (stdout=%q)", code, stdout.String())
	}
	var env struct{ Decision, Reason string }
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout.String())
	}
	if env.Decision != "block" {
		t.Errorf("decision=%q want block", env.Decision)
	}
	if !strings.Contains(env.Reason, "thinking") {
		t.Errorf("reason should carry the 90m time-box body: %q", env.Reason)
	}
	// Marker must be stamped so the tick path doesn't double-fire.
	var n90 sql.NullInt64
	_ = f.db.QueryRow(`SELECT nudged_90m_at FROM claims WHERE item_id=?`, "BUG-T1").Scan(&n90)
	if !n90.Valid {
		t.Errorf("nudged_90m_at should be stamped after listener emit")
	}
}

// TestRunListen_NoTimeBoxBelowThreshold pins the negative case: empty
// mailbox plus a young claim still hits the lifetime fallback with no
// body, exit 0. Guards against false positives from the new pollTimeBox
// path firing on irrelevant claims.
func TestRunListen_NoTimeBoxBelowThreshold(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	f := newChatFixture(t)
	registry := notify.NewRegistry(f.db)
	claimAt := time.Now().Add(-30 * time.Minute) // well below 90m
	if _, err := f.db.Exec(
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, ?, ?, '', 0)`,
		f.repoID, "BUG-T2", f.agentID, claimAt.Unix(), claimAt.Unix(),
	); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	args := listenArgs{Instance: "inst-tb-noop", FallbackInt: 30 * time.Millisecond, MaxLifetime: 100 * time.Millisecond}
	var stdout bytes.Buffer
	code := runListen(context.Background(), f.chat, f.db, f.agentID, f.repoID, registry, args, &stdout)
	if code != 0 {
		t.Errorf("expected 0 (idle exit) when no threshold crossed, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty when no time-box fires, got %q", stdout.String())
	}
}

// TestRunListen_TimeBoxMarkerSuppressesTickPath pins the dedupe contract:
// after the listener emits a time-box nudge, the tick-path
// maybePrintTimeBoxNudge against the same threshold is a no-op.
func TestRunListen_TimeBoxMarkerSuppressesTickPath(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	f := newChatFixture(t)
	registry := notify.NewRegistry(f.db)
	claimAt := time.Now().Add(-91 * time.Minute)
	if _, err := f.db.Exec(
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, ?, ?, '', 0)`,
		f.repoID, "BUG-T3", f.agentID, claimAt.Unix(), claimAt.Unix(),
	); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	args := listenArgs{Instance: "inst-tb-dedupe", FallbackInt: 30 * time.Millisecond, MaxLifetime: time.Second}
	var stdout bytes.Buffer
	if code := runListen(context.Background(), f.chat, f.db, f.agentID, f.repoID, registry, args, &stdout); code != 2 {
		t.Fatalf("listener should fire 90m nudge, got code %d", code)
	}
	// Subsequent tick-path call must be silent — marker is stamped.
	var tickBuf bytes.Buffer
	maybePrintTimeBoxNudge(context.Background(), f.db, f.repoID, f.agentID, time.Now(), &tickBuf)
	if tickBuf.String() != "" {
		t.Errorf("tick path must not double-fire after listener stamp; got %q", tickBuf.String())
	}
}

func TestRunListen_IdleExitOnLifetime(t *testing.T) {
	f := newChatFixture(t)
	registry := notify.NewRegistry(f.db)
	args := listenArgs{
		Instance:    "inst-idle",
		FallbackInt: 30 * time.Millisecond,
		MaxLifetime: 100 * time.Millisecond,
	}
	var stdout bytes.Buffer
	code := runListen(context.Background(), f.chat, f.db, f.agentID, f.repoID, registry, args, &stdout)
	if code != 0 {
		t.Fatalf("expected 0 on idle lifetime exit, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on idle exit, got %q", stdout.String())
	}
}
