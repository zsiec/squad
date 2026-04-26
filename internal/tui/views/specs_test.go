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

func specsFixture(t *testing.T, specs []client.Spec) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/specs" {
			_ = json.NewEncoder(w).Encode(specs)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSpecs_InitFetchesAndPopulates(t *testing.T) {
	specs := []client.Spec{
		{Name: "auth", Title: "Auth subsystem", Path: "/p/auth.md"},
		{Name: "billing", Title: "Billing flow", Path: "/p/billing.md"},
	}
	srv := specsFixture(t, specs)
	c := client.New(srv.URL, "")
	m := NewSpecs(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(SpecsModel)
	out := mm.View()
	for _, want := range []string{"auth", "billing", "Auth subsystem"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestSpecs_EnterEmitsDrillInMsg(t *testing.T) {
	srv := specsFixture(t, []client.Spec{{Name: "auth", Title: "x"}})
	c := client.New(srv.URL, "")
	m := NewSpecs(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SpecsModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	got := cmd()
	dm, ok := got.(SpecsDrillInMsg)
	if !ok {
		t.Fatalf("expected SpecsDrillInMsg, got %T", got)
	}
	if dm.SpecName != "auth" {
		t.Fatalf("specName=%q", dm.SpecName)
	}
}

func TestSpecs_RefreshMsgRefetches(t *testing.T) {
	srv := specsFixture(t, nil)
	c := client.New(srv.URL, "")
	m := NewSpecs(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(SpecsModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}
