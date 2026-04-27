package views

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
)

func mailboxFixture(t *testing.T, me string, msgs []client.Message) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/whoami":
			_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": me, "display_name": me})
			return
		case "/api/messages":
			_ = json.NewEncoder(w).Encode(msgs)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMailbox_InitFiltersMentionsAndDirect(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "global", Body: "hello @agent-me please review", Kind: "say"},
		{ID: 2, AgentID: "bob", Thread: "agent-me", Body: "direct ping", Kind: "say"},
		{ID: 3, AgentID: "carol", Thread: "global", Body: "unrelated chatter", Kind: "say"},
		{ID: 4, AgentID: "dave", Thread: "BUG-7", Body: "noise", Kind: "say"},
	}
	srv := mailboxFixture(t, "agent-me", msgs)
	c := client.New(srv.URL)
	m := NewMailbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(MailboxModel)
	out := mm.View()
	if !strings.Contains(out, "alice") {
		t.Errorf("expected mention from alice in view: %q", out)
	}
	if !strings.Contains(out, "bob") {
		t.Errorf("expected direct from bob in view: %q", out)
	}
	if strings.Contains(out, "carol") {
		t.Errorf("unrelated message from carol should be filtered: %q", out)
	}
	if strings.Contains(out, "dave") {
		t.Errorf("unrelated message from dave should be filtered: %q", out)
	}
}

func TestMailbox_EnterEmitsOpenMsg(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "BUG-7", Body: "@agent-me look", Kind: "say"},
	}
	srv := mailboxFixture(t, "agent-me", msgs)
	c := client.New(srv.URL)
	m := NewMailbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(MailboxModel)
	updated, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	got := cmd()
	om, ok := got.(MailboxOpenMsg)
	if !ok {
		t.Fatalf("expected MailboxOpenMsg, got %T", got)
	}
	if om.Thread != "BUG-7" {
		t.Fatalf("thread=%q", om.Thread)
	}
}

func TestMailbox_RKeyEmitsReplyMsg(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "BUG-7", Body: "@agent-me look", Kind: "say"},
	}
	srv := mailboxFixture(t, "agent-me", msgs)
	c := client.New(srv.URL)
	m := NewMailbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(MailboxModel)
	updated, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_ = updated
	if cmd == nil {
		t.Fatal("expected cmd from r key")
	}
	got := cmd()
	rm, ok := got.(MailboxReplyMsg)
	if !ok {
		t.Fatalf("expected MailboxReplyMsg, got %T", got)
	}
	if rm.Thread != "BUG-7" || rm.To != "alice" {
		t.Fatalf("reply=%+v", rm)
	}
}

func TestMailbox_SSEMessageRefetches(t *testing.T) {
	srv := mailboxFixture(t, "agent-me", nil)
	c := client.New(srv.URL)
	m := NewMailbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(MailboxModel)
	_, cmd := mm.Update(client.Event{Kind: "message", Payload: []byte(`{}`)})
	if cmd == nil {
		t.Fatal("expected refetch on message SSE")
	}
}

func TestMailbox_RefreshMsgRefetches(t *testing.T) {
	srv := mailboxFixture(t, "agent-me", nil)
	c := client.New(srv.URL)
	m := NewMailbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(MailboxModel)
	_, cmd := mm.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected refetch on RefreshMsg")
	}
}

func TestMailbox_NoMatchesShowsEmpty(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "global", Body: "no mentions here", Kind: "say"},
	}
	srv := mailboxFixture(t, "agent-me", msgs)
	c := client.New(srv.URL)
	m := NewMailbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(MailboxModel)
	out := mm.View()
	if strings.Contains(out, "alice") {
		t.Errorf("unrelated message should be filtered: %q", out)
	}
}
