package views

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
)

func doctorFixture(t *testing.T, claims []client.Claim, agents []client.Agent) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/claims":
			if claims == nil {
				_ = json.NewEncoder(w).Encode([]client.Claim{})
				return
			}
			_ = json.NewEncoder(w).Encode(claims)
			return
		case "/api/agents":
			if agents == nil {
				_ = json.NewEncoder(w).Encode([]client.Agent{})
				return
			}
			_ = json.NewEncoder(w).Encode(agents)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDoctor_StuckClaimFlagged(t *testing.T) {
	now := int64(1700000000)
	claims := []client.Claim{
		{ItemID: "BUG-1", AgentID: "alice", ClaimedAt: now - int64(30*time.Hour.Seconds())},
	}
	srv := doctorFixture(t, claims, nil)
	c := client.New(srv.URL)
	m := NewDoctorWithClock(c, func() time.Time { return time.Unix(now, 0) })
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(DoctorModel)
	out := mm.View()
	if !strings.Contains(out, "stuck_claim") {
		t.Errorf("expected stuck_claim in view: %q", out)
	}
}

func TestDoctor_FreshClaimNotFlagged(t *testing.T) {
	now := int64(1700000000)
	claims := []client.Claim{
		{ItemID: "BUG-1", AgentID: "alice", ClaimedAt: now - int64(1*time.Hour.Seconds())},
	}
	srv := doctorFixture(t, claims, nil)
	c := client.New(srv.URL)
	m := NewDoctorWithClock(c, func() time.Time { return time.Unix(now, 0) })
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(DoctorModel)
	out := mm.View()
	if strings.Contains(out, "stuck_claim") {
		t.Errorf("fresh claim should not be flagged: %q", out)
	}
}

func TestDoctor_VanishedAgentFlagged(t *testing.T) {
	now := int64(1700000000)
	claims := []client.Claim{
		{ItemID: "BUG-1", AgentID: "alice", ClaimedAt: now - 60},
	}
	agents := []client.Agent{
		{AgentID: "alice", LastTickAt: now - int64(2*time.Hour.Seconds())},
	}
	srv := doctorFixture(t, claims, agents)
	c := client.New(srv.URL)
	m := NewDoctorWithClock(c, func() time.Time { return time.Unix(now, 0) })
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(DoctorModel)
	out := mm.View()
	if !strings.Contains(out, "vanished_agent") {
		t.Errorf("expected vanished_agent: %q", out)
	}
}

func TestDoctor_AgentWithoutClaimNotVanished(t *testing.T) {
	now := int64(1700000000)
	agents := []client.Agent{
		{AgentID: "alice", LastTickAt: now - int64(2*time.Hour.Seconds())},
	}
	srv := doctorFixture(t, nil, agents)
	c := client.New(srv.URL)
	m := NewDoctorWithClock(c, func() time.Time { return time.Unix(now, 0) })
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(DoctorModel)
	out := mm.View()
	if strings.Contains(out, "vanished_agent") {
		t.Errorf("idle agent without active claim should not be flagged: %q", out)
	}
}

func TestDoctor_EnterEmitsJumpMsg(t *testing.T) {
	now := int64(1700000000)
	claims := []client.Claim{
		{ItemID: "BUG-1", AgentID: "alice", ClaimedAt: now - int64(30*time.Hour.Seconds())},
	}
	srv := doctorFixture(t, claims, nil)
	c := client.New(srv.URL)
	m := NewDoctorWithClock(c, func() time.Time { return time.Unix(now, 0) })
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(DoctorModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	got := cmd()
	jm, ok := got.(DoctorJumpMsg)
	if !ok {
		t.Fatalf("expected DoctorJumpMsg, got %T", got)
	}
	if jm.Subject == "" {
		t.Fatalf("subject empty: %+v", jm)
	}
	_ = updated
}

func TestDoctor_RefreshMsgRefetches(t *testing.T) {
	srv := doctorFixture(t, nil, nil)
	c := client.New(srv.URL)
	m := NewDoctor(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(DoctorModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}

func TestDoctor_SSEItemChangedRefetches(t *testing.T) {
	srv := doctorFixture(t, nil, nil)
	c := client.New(srv.URL)
	m := NewDoctor(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(DoctorModel)
	_, cmd := m.Update(client.Event{Kind: "item_changed", Payload: []byte(`{}`)})
	if cmd == nil {
		t.Fatal("expected refetch from item_changed")
	}
}
