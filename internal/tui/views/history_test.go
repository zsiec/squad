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

func historyFixture(t *testing.T, msgs []client.Message) *httptest.Server {
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

func TestHistory_InitFetchesAndPopulates(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, TS: 1700000000, AgentID: "alice", Thread: "global", Kind: "say", Body: "hello"},
		{ID: 2, TS: 1700000060, AgentID: "bob", Thread: "BUG-1", Kind: "handoff", Body: "passing this off"},
	}
	srv := historyFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewHistory(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(HistoryModel)
	out := mm.View()
	for _, want := range []string{"alice", "bob", "say", "handoff"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestHistory_EnterOnItemThreadEmitsJumpMsg(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "a", Thread: "BUG-7", Kind: "say", Body: "x"},
	}
	srv := historyFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewHistory(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(HistoryModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	if cmd == nil {
		t.Fatal("expected cmd from Enter on item thread")
	}
	got := cmd()
	jm, ok := got.(HistoryJumpToItemMsg)
	if !ok {
		t.Fatalf("expected HistoryJumpToItemMsg, got %T", got)
	}
	if jm.ItemID != "BUG-7" {
		t.Fatalf("itemID=%q", jm.ItemID)
	}
}

func TestHistory_EnterOnGlobalThread_NoOp(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "a", Thread: "global", Kind: "say", Body: "x"},
	}
	srv := historyFixture(t, msgs)
	c := client.New(srv.URL, "")
	m := NewHistory(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(HistoryModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	if cmd != nil {
		got := cmd()
		if _, ok := got.(HistoryJumpToItemMsg); ok {
			t.Fatal("unexpected jump on global thread")
		}
	}
}

func TestHistory_SSEMessageRefetches(t *testing.T) {
	srv := historyFixture(t, []client.Message{{ID: 1, AgentID: "a", Thread: "global", Kind: "say", Body: "x"}})
	c := client.New(srv.URL, "")
	m := NewHistory(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(HistoryModel)
	ev := client.Event{Kind: "message", Payload: []byte(`{}`)}
	_, cmd := m.Update(ev)
	if cmd == nil {
		t.Fatal("expected refetch on message SSE")
	}
	msg := cmd()
	if _, ok := msg.(historyLoadedMsg); !ok {
		t.Fatalf("expected historyLoadedMsg, got %T", msg)
	}
}

func TestHistory_SSEItemChangedRefetches(t *testing.T) {
	srv := historyFixture(t, []client.Message{{ID: 1, AgentID: "a", Thread: "global", Kind: "say", Body: "x"}})
	c := client.New(srv.URL, "")
	m := NewHistory(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(HistoryModel)
	ev := client.Event{Kind: "item_changed", Payload: []byte(`{}`)}
	_, cmd := m.Update(ev)
	if cmd == nil {
		t.Fatal("expected refetch on item_changed")
	}
}

func TestHistory_RefreshMsgRefetches(t *testing.T) {
	srv := historyFixture(t, nil)
	c := client.New(srv.URL, "")
	m := NewHistory(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(HistoryModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}
