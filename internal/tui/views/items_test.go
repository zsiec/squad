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
	// Press space on first row to claim ('c' is now the filter-cycle key)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	// Run the cmd to fire the POST
	if cmd == nil {
		t.Fatal("expected non-nil cmd from claim key")
	}
	_ = cmd()
	if got := f.PostURL(); got != "/api/items/BUG-1/claim" {
		t.Fatalf("claim url=%q", got)
	}
}

func TestItems_DefaultFilterExcludesCaptured(t *testing.T) {
	items := []client.Item{
		{ID: "CAP-1", Title: "captured-one", Status: "captured"},
		{ID: "OPN-1", Title: "open-one", Status: "open"},
		{ID: "BLK-1", Title: "blocked-one", Status: "blocked"},
	}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(ItemsModel)
	out := mm.View()
	if strings.Contains(out, "CAP-1") {
		t.Fatalf("default filter should hide captured, got %q", out)
	}
	if !strings.Contains(out, "OPN-1") {
		t.Fatalf("default filter should show open, got %q", out)
	}
	if !strings.Contains(out, "BLK-1") {
		t.Fatalf("default filter should show blocked (active work), got %q", out)
	}
	if !strings.Contains(out, "captured: 1") || !strings.Contains(out, "open: 1") || !strings.Contains(out, "blocked: 1") {
		t.Fatalf("count band missing expected counts, got %q", out)
	}
}

func TestItems_PressCCyclesFilter(t *testing.T) {
	items := []client.Item{
		{ID: "CAP-1", Title: "captured-one", Status: "captured"},
		{ID: "OPN-1", Title: "open-one", Status: "open"},
		{ID: "BLK-1", Title: "blocked-one", Status: "blocked"},
	}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(ItemsModel)

	// Press 'c': filter should advance from open → captured.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	mm = updated.(ItemsModel)
	out := mm.View()
	if !strings.Contains(out, "CAP-1") {
		t.Fatalf("after 1x c expected CAP-1 visible, got %q", out)
	}
	if strings.Contains(out, "OPN-1") || strings.Contains(out, "BLK-1") {
		t.Fatalf("after 1x c only captured should show, got %q", out)
	}

	// Press 'c' again: filter → blocked.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	mm = updated.(ItemsModel)
	out = mm.View()
	if !strings.Contains(out, "BLK-1") {
		t.Fatalf("after 2x c expected BLK-1 visible, got %q", out)
	}
	if strings.Contains(out, "OPN-1") || strings.Contains(out, "CAP-1") {
		t.Fatalf("after 2x c only blocked should show, got %q", out)
	}

	// Press 'c' again: filter → all.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	mm = updated.(ItemsModel)
	out = mm.View()
	if !strings.Contains(out, "CAP-1") || !strings.Contains(out, "OPN-1") || !strings.Contains(out, "BLK-1") {
		t.Fatalf("after 3x c all rows should show, got %q", out)
	}

	// Press 'c' again: back to open (active work, captured excluded).
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	mm = updated.(ItemsModel)
	out = mm.View()
	if !strings.Contains(out, "OPN-1") || !strings.Contains(out, "BLK-1") {
		t.Fatalf("after 4x c expected open + blocked visible, got %q", out)
	}
	if strings.Contains(out, "CAP-1") {
		t.Fatalf("after 4x c captured should be hidden, got %q", out)
	}
}

func TestItems_FilterRowFormatting(t *testing.T) {
	items := []client.Item{
		{ID: "CAP-1", Title: "c", Status: "captured"},
		{ID: "OPN-1", Title: "o", Status: "open"},
		{ID: "BLK-1", Title: "b", Status: "blocked"},
		{ID: "DON-1", Title: "d", Status: "done"},
	}
	f := newFixture(t, items)
	c := client.New(f.srv.URL, "")
	m := NewItems(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(ItemsModel)
	out := mm.View()

	// Order of statuses on the band: captured, open, blocked, done.
	cap := strings.Index(out, "captured:")
	op := strings.Index(out, "open:")
	bl := strings.Index(out, "blocked:")
	dn := strings.Index(out, "done:")
	if cap < 0 || op < 0 || bl < 0 || dn < 0 {
		t.Fatalf("count band missing entries, got %q", out)
	}
	if !(cap < op && op < bl && bl < dn) {
		t.Fatalf("count band order wrong: cap=%d open=%d blocked=%d done=%d (%q)", cap, op, bl, dn, out)
	}

	// Default filter is open; the active filter should be highlighted with [ ].
	if !strings.Contains(out, "[open: 1]") {
		t.Fatalf("expected active filter [open: 1], got %q", out)
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
