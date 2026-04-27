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

type inboxFixture struct {
	srv          *httptest.Server
	postURL      atomic.Value
	postBody     atomic.Value
	acceptStatus int
	acceptBody   string
	rejectStatus int
}

func newInboxFixture(t *testing.T, entries []client.InboxEntry) *inboxFixture {
	t.Helper()
	f := &inboxFixture{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/inbox" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(entries)
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
			if strings.HasSuffix(r.URL.Path, "/accept") {
				if f.acceptStatus != 0 {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(f.acceptStatus)
					_, _ = w.Write([]byte(f.acceptBody))
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/reject") {
				if f.rejectStatus != 0 {
					w.WriteHeader(f.rejectStatus)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *inboxFixture) PostURL() string {
	v := f.postURL.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func (f *inboxFixture) PostBody() map[string]any {
	v := f.postBody.Load()
	if v == nil {
		return nil
	}
	return v.(map[string]any)
}

func TestInbox_InitFetchesAndPopulates(t *testing.T) {
	entries := []client.InboxEntry{
		{ID: "FEAT-001", Title: "first", CapturedBy: "alice", DoRPass: true, Path: "/p/FEAT-001.md"},
		{ID: "BUG-002", Title: "second", CapturedBy: "bob", DoRPass: false, Path: "/p/BUG-002.md"},
		{ID: "FEAT-003", Title: "third", CapturedBy: "carol", DoRPass: true, Path: "/p/FEAT-003.md"},
	}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(InboxModel)
	out := mm.View()
	for _, want := range []string{"FEAT-001", "BUG-002", "FEAT-003"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestInbox_AcceptKeyHitsAcceptEndpoint(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from accept key")
	}
	_ = cmd()
	if got := f.PostURL(); got != "/api/items/FEAT-001/accept" {
		t.Fatalf("accept url=%q", got)
	}
}

func TestInbox_AcceptShowsDoRViolations(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: false, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	f.acceptStatus = http.StatusUnprocessableEntity
	f.acceptBody = `{"violations":[{"rule":"area-set","field":"area","message":"area is unset"},{"rule":"acceptance-criterion","field":"body","message":"no AC checkbox"}]}`
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected accept cmd")
	}
	resultMsg := cmd()
	updated, _ = m.Update(resultMsg)
	mm := updated.(InboxModel)
	out := mm.View()
	for _, want := range []string{"area-set", "area is unset", "acceptance-criterion", "no AC checkbox"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing violation %q: %q", want, out)
		}
	}
}

func TestInbox_RejectKeyEntersRejectMode(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	mm := updated.(InboxModel)
	if !mm.InReject() {
		t.Fatal("expected model to be in reject-prompt state after 'r'")
	}
	if mm.RejectingID() != "FEAT-001" {
		t.Fatalf("rejecting id=%q want FEAT-001", mm.RejectingID())
	}
}

func TestInbox_RejectModeEnterSubmits(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(InboxModel)
	for _, r := range "dup" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(InboxModel)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(InboxModel)
	if cmd == nil {
		t.Fatal("expected reject cmd from Enter")
	}
	_ = cmd()
	if got := f.PostURL(); got != "/api/items/FEAT-001/reject" {
		t.Fatalf("reject url=%q", got)
	}
	if body := f.PostBody(); body == nil || body["reason"] != "dup" {
		t.Fatalf("body=%v", body)
	}
	if m.InReject() {
		t.Fatal("expected reject mode to clear after submit")
	}
}

func TestInbox_RejectModeEmptyEnterShowsErrorAndKeepsOpen(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(InboxModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(InboxModel)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(inboxRejectResultMsg); ok {
				t.Fatal("empty Enter should not fire reject")
			}
		}
	}
	if !m.InReject() {
		t.Fatal("modal should stay open after empty submit")
	}
	out := m.View()
	if !strings.Contains(out, "required") {
		t.Fatalf("expected inline error containing 'required', got %q", out)
	}
}

func TestInbox_RejectModalRendersInView(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(InboxModel)
	out := m.View()
	if !strings.Contains(out, "Reject reason") {
		t.Fatalf("View missing modal header 'Reject reason': %q", out)
	}
}

func TestInbox_RejectModeEscCancels(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(InboxModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(InboxModel)
	if mm.InReject() {
		t.Fatal("expected reject mode to clear on Esc")
	}
}

func TestInbox_RefreshKeyRefetches(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Fatal("expected refetch cmd from R")
	}
	msg := cmd()
	if _, ok := msg.(inboxLoadedMsg); !ok {
		t.Fatalf("expected inboxLoadedMsg, got %T", msg)
	}
}

func TestInbox_SSEInboxChangedRefetches(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	ev := client.Event{Kind: "inbox_changed", Payload: []byte(`{"item_id":"FEAT-001","action":"captured"}`)}
	_, cmd := m.Update(ev)
	if cmd == nil {
		t.Fatal("expected refetch cmd on inbox_changed")
	}
	msg := cmd()
	if _, ok := msg.(inboxLoadedMsg); !ok {
		t.Fatalf("expected inboxLoadedMsg, got %T", msg)
	}
}

func TestInbox_RefreshMsgRefetches(t *testing.T) {
	entries := []client.InboxEntry{{ID: "FEAT-001", Title: "x", DoRPass: true, Path: "/p/FEAT-001.md"}}
	f := newInboxFixture(t, entries)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}

func TestInbox_AcceptOnEmptyNoOp(t *testing.T) {
	f := newInboxFixture(t, nil)
	c := client.New(f.srv.URL)
	m := NewInbox(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(InboxModel)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		got := cmd()
		if _, ok := got.(inboxAcceptResultMsg); ok {
			t.Fatal("accept should not fire when there are no rows")
		}
	}
}
