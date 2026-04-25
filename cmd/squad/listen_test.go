package main

import (
	"bytes"
	"context"
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
		done <- runListen(ctx, f.chat, f.agentID, f.repoID, registry, args, &stdout)
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

func TestRunListen_IdleExitOnLifetime(t *testing.T) {
	f := newChatFixture(t)
	registry := notify.NewRegistry(f.db)
	args := listenArgs{
		Instance:    "inst-idle",
		FallbackInt: 30 * time.Millisecond,
		MaxLifetime: 100 * time.Millisecond,
	}
	var stdout bytes.Buffer
	code := runListen(context.Background(), f.chat, f.agentID, f.repoID, registry, args, &stdout)
	if code != 0 {
		t.Fatalf("expected 0 on idle lifetime exit, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on idle exit, got %q", stdout.String())
	}
}
