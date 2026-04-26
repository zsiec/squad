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

// agentsFixture: minimal server returning a fixed agents list.
func agentsFixture(t *testing.T, agents []client.Agent) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents" {
			_ = json.NewEncoder(w).Encode(agents)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAgents_InitFetchesAndPopulates(t *testing.T) {
	agents := []client.Agent{
		{AgentID: "agent-1", DisplayName: "alice", Status: "active", LastTickAt: 1700000000},
		{AgentID: "agent-2", DisplayName: "bob", Status: "idle", LastTickAt: 1700000100},
	}
	srv := agentsFixture(t, agents)
	c := client.New(srv.URL, "")
	m := NewAgents(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(AgentsModel)
	out := mm.View()
	for _, want := range []string{"agent-1", "alice", "agent-2", "bob"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestAgents_EnterEmitsOpenSessionMsg(t *testing.T) {
	agents := []client.Agent{
		{AgentID: "agent-1", DisplayName: "alice", Status: "active"},
	}
	srv := agentsFixture(t, agents)
	c := client.New(srv.URL, "")
	m := NewAgents(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(AgentsModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter")
	}
	got := cmd()
	osm, ok := got.(AgentsOpenSessionMsg)
	if !ok {
		t.Fatalf("expected AgentsOpenSessionMsg, got %T", got)
	}
	if osm.AgentID != "agent-1" {
		t.Fatalf("agent_id=%q", osm.AgentID)
	}
}

func TestAgents_SSEAgentStatusRefetches(t *testing.T) {
	agents := []client.Agent{{AgentID: "agent-1", DisplayName: "alice", Status: "idle"}}
	srv := agentsFixture(t, agents)
	c := client.New(srv.URL, "")
	m := NewAgents(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(AgentsModel)
	ev := client.Event{Kind: "agent_status", Payload: []byte(`{"agent_id":"agent-1","kind":"updated"}`)}
	_, cmd := m.Update(ev)
	if cmd == nil {
		t.Fatal("expected refetch cmd on agent_status")
	}
	msg := cmd()
	if _, ok := msg.(agentsLoadedMsg); !ok {
		t.Fatalf("expected agentsLoadedMsg, got %T", msg)
	}
}

func TestAgents_RefreshMsgRefetches(t *testing.T) {
	srv := agentsFixture(t, nil)
	c := client.New(srv.URL, "")
	m := NewAgents(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(AgentsModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected refetch cmd on RefreshMsg")
	}
}
