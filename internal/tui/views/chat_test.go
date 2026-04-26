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

func chatFixture(t *testing.T, msgs []client.Message) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/messages" {
			_ = json.NewEncoder(w).Encode(msgs)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestChat_InitFetchesAndRenders(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, TS: 1700000000, AgentID: "alice", Thread: "global", Kind: "say", Body: "hello world"},
		{ID: 2, TS: 1700000060, AgentID: "bob", Thread: "BUG-1", Kind: "say", Body: "looking at this"},
	}
	srv := chatFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewChat(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(ChatModel)
	out := mm.View()
	for _, want := range []string{"alice", "bob", "hello world", "looking at this"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestChat_SSEMessageAppends(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, TS: 1700000000, AgentID: "alice", Thread: "global", Kind: "say", Body: "first"},
	}
	srv := chatFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewChat(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ChatModel)
	// SSE delivers a new message
	ev := client.Event{
		Kind:    "message",
		Payload: []byte(`{"id":2,"ts":1700000100,"agent_id":"bob","thread":"global","kind":"say","body":"second"}`),
	}
	updated, _ = m.Update(ev)
	m = updated.(ChatModel)
	out := m.View()
	if !strings.Contains(out, "second") {
		t.Errorf("View missing newly-appended SSE message: %q", out)
	}
}

func TestChat_ReplyKeyEmitsReplyMsg(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "BUG-1", Kind: "say", Body: "hi"},
	}
	srv := chatFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewChat(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ChatModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(ChatModel)
	if cmd == nil {
		t.Fatal("expected cmd from r key")
	}
	got := cmd()
	rmsg, ok := got.(ChatReplyMsg)
	if !ok {
		t.Fatalf("expected ChatReplyMsg, got %T", got)
	}
	if rmsg.To != "alice" || rmsg.Thread != "BUG-1" {
		t.Fatalf("reply msg=%+v", rmsg)
	}
}

func TestChat_JumpKeyEmitsJumpToItemMsg(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "BUG-42", Kind: "say", Body: "x"},
	}
	srv := chatFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewChat(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ChatModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = updated.(ChatModel)
	if cmd == nil {
		t.Fatal("expected cmd from i key")
	}
	got := cmd()
	jm, ok := got.(ChatJumpToItemMsg)
	if !ok {
		t.Fatalf("expected ChatJumpToItemMsg, got %T", got)
	}
	if jm.ItemID != "BUG-42" {
		t.Fatalf("jump itemID=%q", jm.ItemID)
	}
}

func TestChat_JumpKey_NoOpOnGlobalThread(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "global", Kind: "say", Body: "x"},
	}
	srv := chatFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewChat(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ChatModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_ = updated
	// The 'i' key on a global-thread message should NOT emit ChatJumpToItemMsg.
	if cmd != nil {
		got := cmd()
		if _, ok := got.(ChatJumpToItemMsg); ok {
			t.Fatal("unexpected ChatJumpToItemMsg on global-thread message")
		}
	}
}

func TestChat_RefreshMsgRefetches(t *testing.T) {
	srv := chatFixture(t, nil)
	c := client.New(srv.URL, "")
	m := NewChat(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ChatModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}
