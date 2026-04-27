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

func epicsFixture(t *testing.T, epics []client.Epic) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/epics" {
			_ = json.NewEncoder(w).Encode(epics)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEpics_InitFetchesAndPopulates(t *testing.T) {
	epics := []client.Epic{
		{Name: "login-redesign", Spec: "auth", Status: "open", Parallelism: "low"},
	}
	srv := epicsFixture(t, epics)
	c := client.New(srv.URL)
	m := NewEpics(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(EpicsModel)
	out := mm.View()
	for _, want := range []string{"login-redesign", "auth", "open"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestEpics_EnterEmitsDrillInMsg(t *testing.T) {
	srv := epicsFixture(t, []client.Epic{{Name: "login-redesign", Spec: "auth"}})
	c := client.New(srv.URL)
	m := NewEpics(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(EpicsModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	got := cmd()
	dm, ok := got.(EpicsDrillInMsg)
	if !ok {
		t.Fatalf("expected EpicsDrillInMsg, got %T", got)
	}
	if dm.EpicName != "login-redesign" {
		t.Fatalf("epicName=%q", dm.EpicName)
	}
}

func TestEpics_SetSpecFilterAppendsQuery(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)
	c := client.New(srv.URL)
	m := NewEpics(c).SetSpecFilter("auth")
	_ = runCmd(t, m.Init())
	if !strings.Contains(gotURL, "spec=auth") {
		t.Fatalf("url=%q want spec=auth", gotURL)
	}
}

func TestEpics_RefreshMsgRefetches(t *testing.T) {
	srv := epicsFixture(t, nil)
	c := client.New(srv.URL)
	m := NewEpics(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(EpicsModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}
