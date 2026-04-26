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

// helper: server that returns the given items list for GET /api/items
// and records any POST URL hit (for asserting on action keys).
type testFixture struct {
	srv      *httptest.Server
	postURL  atomic.Value // string
	postBody atomic.Value // map[string]any
	t        *testing.T
}

func newFixture(t *testing.T, items []client.Item) *testFixture {
	t.Helper()
	f := &testFixture{t: t}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/items" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(items)
			return
		}
		if r.Method == http.MethodPost {
			f.postURL.Store(r.URL.Path)
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body == nil {
				body = map[string]any{}
			}
			f.postBody.Store(body)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *testFixture) PostURL() string {
	v := f.postURL.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestItems_InitFetchesAndPopulates(t *testing.T) {
	items := []client.Item{
		{ID: "BUG-1", Title: "first", Status: "open"},
		{ID: "BUG-2", Title: "second", Status: "claimed"},
	}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	cmd := m.Init()
	msg := runCmd(t, cmd)
	updated, _ := m.Update(msg)
	mm := updated.(ItemsModel)
	out := mm.View()
	if !strings.Contains(out, "BUG-1") || !strings.Contains(out, "BUG-2") {
		t.Fatalf("View missing items: %q", out)
	}
}

func TestItems_RendersR3R4Columns(t *testing.T) {
	items := []client.Item{
		{ID: "BUG-1", Title: "x", Status: "claimed", Epic: "auth", Parallel: true,
			DependsOn: []string{"BUG-99"}, EvidenceRequired: []string{"test"},
			ClaimedBy: "alice", LastTouch: 1700000000},
	}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(ItemsModel)
	out := mm.View()
	for _, want := range []string{"auth", "alice"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestItems_ClaimKeyHitsClaimEndpoint(t *testing.T) {
	items := []client.Item{{ID: "BUG-1", Title: "x", Status: "open"}}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	// Load
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ItemsModel)
	// Press 'c' on first row
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	// Run the cmd to fire the POST
	if cmd == nil {
		t.Fatal("expected non-nil cmd from claim key")
	}
	_ = cmd()
	if got := f.PostURL(); got != "/api/items/BUG-1/claim" {
		t.Fatalf("claim url=%q", got)
	}
}

func TestItems_ReleaseKeyHitsReleaseEndpoint(t *testing.T) {
	items := []client.Item{{ID: "BUG-1", Title: "x", Status: "claimed"}}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ItemsModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from release key")
	}
	_ = cmd()
	if got := f.PostURL(); got != "/api/items/BUG-1/release" {
		t.Fatalf("release url=%q", got)
	}
}

func TestItems_EnterEmitsDrillInMsg(t *testing.T) {
	items := []client.Item{{ID: "BUG-1", Title: "x", Status: "open"}}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ItemsModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from enter")
	}
	got := cmd()
	dm, ok := got.(ItemsDrillInMsg)
	if !ok {
		t.Fatalf("expected ItemsDrillInMsg, got %T", got)
	}
	if dm.ItemID != "BUG-1" {
		t.Fatalf("drillIn id=%q", dm.ItemID)
	}
}

func TestItems_SSEItemChangedRefetches(t *testing.T) {
	items := []client.Item{{ID: "BUG-1", Title: "before", Status: "open"}}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ItemsModel)
	// SSE delivers item_changed
	ev := client.Event{Kind: "item_changed", Payload: []byte(`{"item_id":"BUG-1","kind":"claimed"}`)}
	_, cmd := m.Update(ev)
	if cmd == nil {
		t.Fatal("expected refetch cmd on item_changed")
	}
	// Ensure the refetch fires by running the cmd; should produce another itemsLoadedMsg
	msg := cmd()
	if msg == nil {
		t.Fatal("refetch cmd produced nil msg")
	}
	if _, ok := msg.(itemsLoadedMsg); !ok {
		t.Fatalf("expected itemsLoadedMsg, got %T", msg)
	}
}

func TestItems_RefreshMsgRefetches(t *testing.T) {
	items := []client.Item{{ID: "BUG-1", Title: "x", Status: "open"}}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ItemsModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected refetch cmd on RefreshMsg")
	}
	_ = cmd
}
