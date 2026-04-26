package tui_test

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	_ "modernc.org/sqlite"

	"github.com/zsiec/squad/internal/server"
	"github.com/zsiec/squad/internal/store"
	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/views"
)

const e2eRepoID = "test-repo"

// e2eFixture stands up a real server with a temp DB and the BUG-100 items
// fixture from internal/server/testdata/items.
func e2eFixture(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	srv := server.New(db, e2eRepoID, server.Config{
		SquadDir: "../server/testdata",
	})
	t.Cleanup(srv.Close)

	// Provide a default acting agent so claim/messages handlers don't
	// reject for missing X-Squad-Agent. The TUI client doesn't set the
	// header today; the server falls back to callerAgent.
	srv = srv.WithCallerAgent("agent-tui")

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, db
}

func TestE2E_ClaimFlow(t *testing.T) {
	ts, db := e2eFixture(t)
	c := client.New(ts.URL, "")

	m := views.NewItems(c)
	updated, _ := m.Update(runE2ECmd(t, m.Init()))
	mm := updated.(views.ItemsModel)

	out := mm.View()
	if !strings.Contains(out, "BUG-100") {
		t.Fatalf("expected BUG-100 in items view, got %q", out)
	}

	_, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from c key")
	}
	msg := cmd()

	if _, ok := msg.(views.ItemsClaimedMsg); !ok {
		t.Fatalf("expected ItemsClaimedMsg, got %T (msg=%+v)", msg, msg)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM claims WHERE item_id = 'BUG-100'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 claim row, got %d", count)
	}
}

// runE2ECmd executes a tea.Cmd and returns its emitted Msg.
func runE2ECmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestE2E_SessionMessage(t *testing.T) {
	ts, db := e2eFixture(t)
	c := client.New(ts.URL, "")

	// Pre-seed an agent row so the whoami response is non-empty.
	now := time.Now().Unix()
	if _, err := db.Exec(`INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status) VALUES ('agent-tui', ?, 'me', '/tmp/wt', 1, ?, ?, 'active')`,
		e2eRepoID, now, now); err != nil {
		t.Fatal(err)
	}

	// Thread must be 'global' or PREFIX-NUMBER (server-side regex). Use
	// BUG-100 as a stand-in target — the session view doesn't care that
	// the target is an item rather than an agent for messaging purposes.
	const target = "BUG-100"
	m := views.NewSession(c, target)
	updated, _ := m.Update(runE2ECmd(t, m.Init()))
	mm := updated.(views.SessionModel)

	for _, r := range "ping" {
		updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		mm = updated.(views.SessionModel)
	}
	_, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	// Run the send. Result msg is unexported (sessionSendOKMsg); the
	// canonical evidence the POST landed is the messages-table row.
	_ = cmd()

	var body, thread, kind, mentions string
	if err := db.QueryRow(`SELECT body, thread, kind, mentions FROM messages ORDER BY id DESC LIMIT 1`).Scan(&body, &thread, &kind, &mentions); err != nil {
		t.Fatal(err)
	}
	if body != "ping" || thread != "global" || kind != "say" {
		t.Errorf("got body=%q thread=%q kind=%q want ping/global/say", body, thread, kind)
	}
	if !strings.Contains(mentions, target) {
		t.Errorf("expected mentions to include %q, got %q", target, mentions)
	}
}

func TestE2E_SSEInvalidation(t *testing.T) {
	ts, db := e2eFixture(t)
	c := client.New(ts.URL, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	eventCh := c.SubscribeEvents(ctx)

	m := views.NewItems(c)
	updated, _ := m.Update(runE2ECmd(t, m.Init()))
	mm := updated.(views.ItemsModel)

	// Out-of-band claim insert simulates a CLI-side `squad claim`. The
	// server's claimsPump observes it on its 500ms tick and emits
	// item_changed onto the bus, which the SSE handler forwards.
	now := time.Now().Unix()
	if _, err := db.Exec(`INSERT INTO claims (item_id, repo_id, agent_id, claimed_at, last_touch, intent, long) VALUES ('BUG-100', ?, 'agent-x', ?, ?, '', 0)`,
		e2eRepoID, now, now); err != nil {
		t.Fatal(err)
	}

	var ev client.Event
	select {
	case ev = <-eventCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
	if ev.Kind != "item_changed" {
		t.Fatalf("expected item_changed, got %q", ev.Kind)
	}

	_, refetchCmd := mm.Update(ev)
	if refetchCmd == nil {
		t.Fatal("expected refetch cmd from item_changed")
	}
	refMsg := refetchCmd()
	updated, _ = mm.Update(refMsg)
	mm = updated.(views.ItemsModel)

	if mm.View() == "" {
		t.Fatal("View empty after refetch")
	}
}

// suppress unused imports in case future test variants drop them.
var _ = context.Background
var _ = time.Second
