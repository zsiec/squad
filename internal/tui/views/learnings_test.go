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

func learningsFixture(t *testing.T, learnings []client.Learning) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/learnings" {
			_ = json.NewEncoder(w).Encode(learnings)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLearnings_InitFetchesAndPopulates(t *testing.T) {
	learnings := []client.Learning{
		{Slug: "wal-corruption", Kind: "gotcha", State: "approved", Area: "store", Title: "WAL must be unlocked"},
		{Slug: "table-pattern", Kind: "pattern", State: "proposed", Area: "tui", Title: "Use shared table"},
	}
	srv := learningsFixture(t, learnings)
	c := client.New(srv.URL, "")
	m := NewLearnings(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(LearningsModel)
	out := mm.View()
	for _, want := range []string{"wal-corruption", "table-pattern", "gotcha", "pattern"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestLearnings_EnterEmitsJumpToFirstRelatedItem(t *testing.T) {
	learnings := []client.Learning{
		{Slug: "x", Kind: "gotcha", State: "approved", Related: []string{"BUG-42", "BUG-43"}},
	}
	srv := learningsFixture(t, learnings)
	c := client.New(srv.URL, "")
	m := NewLearnings(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(LearningsModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	got := cmd()
	jm, ok := got.(LearningsJumpToItemMsg)
	if !ok {
		t.Fatalf("expected LearningsJumpToItemMsg, got %T", got)
	}
	if jm.ItemID != "BUG-42" {
		t.Fatalf("itemID=%q want BUG-42", jm.ItemID)
	}
}

func TestLearnings_EnterNoOpWhenNoRelatedItems(t *testing.T) {
	learnings := []client.Learning{
		{Slug: "x", Kind: "gotcha", State: "approved", Related: nil},
	}
	srv := learningsFixture(t, learnings)
	c := client.New(srv.URL, "")
	m := NewLearnings(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(LearningsModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	if cmd != nil {
		got := cmd()
		if _, ok := got.(LearningsJumpToItemMsg); ok {
			t.Fatal("unexpected LearningsJumpToItemMsg when Related is empty")
		}
	}
}

func TestLearnings_SSELearningStateChangedRefetches(t *testing.T) {
	srv := learningsFixture(t, []client.Learning{{Slug: "x", Kind: "gotcha", State: "proposed"}})
	c := client.New(srv.URL, "")
	m := NewLearnings(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(LearningsModel)
	ev := client.Event{Kind: "learning_state_changed", Payload: []byte(`{"slug":"x","to_state":"approved"}`)}
	_, cmd := m.Update(ev)
	if cmd == nil {
		t.Fatal("expected refetch on learning_state_changed")
	}
	msg := cmd()
	if _, ok := msg.(learningsLoadedMsg); !ok {
		t.Fatalf("expected learningsLoadedMsg, got %T", msg)
	}
}

func TestLearnings_RefreshMsgRefetches(t *testing.T) {
	srv := learningsFixture(t, nil)
	c := client.New(srv.URL, "")
	m := NewLearnings(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(LearningsModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}
