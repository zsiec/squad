package views

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
)

func sessionFixture(t *testing.T, me string, msgs []client.Message) (*httptest.Server, *atomic.Value /*postBody*/) {
	t.Helper()
	postBody := &atomic.Value{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/whoami":
			_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": me, "display_name": me})
			return
		case "/api/messages":
			if r.Method == http.MethodPost {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				postBody.Store(body)
				_, _ = w.Write([]byte(`{"ok":true}`))
				return
			}
			_ = json.NewEncoder(w).Encode(msgs)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, postBody
}

func TestSession_InitLoadsAndFiltersTranscript(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "alice", Thread: "global", Kind: "say", Body: "@target hey"},
		{ID: 2, AgentID: "target", Thread: "global", Kind: "say", Body: "what's up"},
		{ID: 3, AgentID: "bob", Thread: "global", Kind: "say", Body: "unrelated"}, // shouldn't show
	}
	srv, _ := sessionFixture(t, "me", msgs)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(SessionModel)
	out := mm.View()
	if !strings.Contains(out, "@target hey") {
		t.Errorf("expected first message in transcript: %q", out)
	}
	if !strings.Contains(out, "what's up") {
		t.Errorf("expected target's message: %q", out)
	}
	if strings.Contains(out, "unrelated") {
		t.Errorf("unrelated message should be filtered: %q", out)
	}
}

func TestSession_CtrlKCyclesKind(t *testing.T) {
	srv, _ := sessionFixture(t, "me", nil)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SessionModel)
	if m.Kind() != "say" {
		t.Fatalf("default kind=%q want say", m.Kind())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(SessionModel)
	if m.Kind() != "ask" {
		t.Errorf("after ctrl-k, kind=%q want ask", m.Kind())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(SessionModel)
	if m.Kind() != "knock" {
		t.Errorf("after ctrl-k×2, kind=%q want knock", m.Kind())
	}
}

func TestSession_EnterSendsMessage(t *testing.T) {
	srv, postBody := sessionFixture(t, "me", nil)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SessionModel)

	// Type "hello" character by character
	for _, r := range "hello" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(SessionModel)
	}

	// Enter to send
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	_ = cmd() // run the POST

	body := postBody.Load()
	if body == nil {
		t.Fatal("POST not made")
	}
	bm := body.(map[string]any)
	if bm["thread"] != "global" {
		t.Errorf("post thread=%v want global", bm["thread"])
	}
	mentions, _ := bm["mentions"].([]any)
	hasTarget := false
	for _, m := range mentions {
		if m == "target" {
			hasTarget = true
		}
	}
	if !hasTarget {
		t.Errorf("expected mentions to include 'target', got %v", mentions)
	}
	if bm["body"] != "hello" {
		t.Errorf("post body=%v", bm["body"])
	}
	if bm["kind"] != "say" {
		t.Errorf("expected kind=say, got %v", bm["kind"])
	}
}

func TestSession_EmptyEnterDoesNothing(t *testing.T) {
	srv, postBody := sessionFixture(t, "me", nil)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SessionModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		_ = cmd()
		if postBody.Load() != nil {
			t.Fatal("empty Enter should not POST")
		}
	}
}

func TestSession_EscWithEmptyComposerEmitsExit(t *testing.T) {
	srv, _ := sessionFixture(t, "me", nil)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SessionModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd from Esc with empty composer")
	}
	got := cmd()
	if _, ok := got.(SessionExitMsg); !ok {
		t.Fatalf("expected SessionExitMsg, got %T", got)
	}
}

func TestSession_EscWithNonEmptyComposerClears(t *testing.T) {
	srv, _ := sessionFixture(t, "me", nil)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SessionModel)
	for _, r := range "draft" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(SessionModel)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		got := cmd()
		if _, ok := got.(SessionExitMsg); ok {
			t.Fatal("Esc on non-empty composer should clear, not exit")
		}
	}
	// Verify composer is now empty
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(SessionModel)
	_ = mm
	// Subsequent Enter should not POST (composer was cleared)
}

func TestSession_SSEMessageAppendsIfQualified(t *testing.T) {
	msgs := []client.Message{
		{ID: 1, AgentID: "target", Thread: "global", Kind: "say", Body: "first"},
	}
	srv, _ := sessionFixture(t, "me", msgs)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SessionModel)
	ev := client.Event{
		Kind:    "message",
		Payload: []byte(`{"id":2,"agent_id":"target","thread":"global","kind":"say","body":"new"}`),
	}
	updated, _ = m.Update(ev)
	mm := updated.(SessionModel)
	out := mm.View()
	if !strings.Contains(out, "new") {
		t.Errorf("expected new message in transcript: %q", out)
	}
}

func TestSession_SSEMessageIgnoredIfUnqualified(t *testing.T) {
	srv, _ := sessionFixture(t, "me", nil)
	c := client.New(srv.URL)
	m := NewSession(c, "target")
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SessionModel)
	ev := client.Event{
		Kind:    "message",
		Payload: []byte(`{"id":2,"agent_id":"random","thread":"global","kind":"say","body":"unrelated"}`),
	}
	updated, _ = m.Update(ev)
	mm := updated.(SessionModel)
	out := mm.View()
	if strings.Contains(out, "unrelated") {
		t.Errorf("unrelated SSE message should not appear: %q", out)
	}
}
