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

// TestE2E_ListenAndMailbox_BinaryFlow exercises the full chain: listen
// registers a notify endpoint, a peer post + TCP wake triggers exit-2
// with decision-block JSON, and a follow-up mailbox call confirms the
// cursor advanced to empty.
func TestE2E_ListenAndMailbox_BinaryFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e")
	}
	f := newChatFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := f.db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES ('peer', ?, 'P', '/tmp/wt', 1, 1, 1, 'active')
	`, f.repoID); err != nil {
		t.Fatal(err)
	}

	registry := notify.NewRegistry(f.db)

	var stdout bytes.Buffer
	a := listenArgs{Instance: "e2e-1", FallbackInt: 50 * time.Millisecond, MaxLifetime: 3 * time.Second}

	doneCh := make(chan int, 1)
	go func() { doneCh <- runListen(ctx, f.chat, f.agentID, f.repoID, registry, a, &stdout) }()

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
		t.Fatal("listen did not register")
	}

	if err := f.chat.Post(ctx, chat.PostRequest{AgentID: "peer", Thread: chat.ThreadGlobal, Kind: chat.KindSay, Body: "@" + f.agentID + " e2e"}); err != nil {
		t.Fatal(err)
	}
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 200*time.Millisecond)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = c.Close()

	select {
	case code := <-doneCh:
		if code != 2 {
			t.Fatalf("expected exit 2, got %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listen never woke")
	}

	var env struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout.String())
	}
	if env.Decision != "block" || !strings.Contains(env.Reason, "e2e") {
		t.Fatalf("envelope wrong: %+v", env)
	}

	var mboxOut bytes.Buffer
	code := runMailbox(ctx, f.chat, f.agentID, mailboxArgs{Format: "additional-context", Event: "UserPromptSubmit"}, &mboxOut)
	if code != 0 || mboxOut.Len() != 0 {
		t.Fatalf("post-listen mailbox should be empty, got code=%d body=%q", code, mboxOut.String())
	}
}
