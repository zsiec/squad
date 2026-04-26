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

	updated, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	mm = updated.(views.ItemsModel)
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

// suppress unused imports in case future test variants drop them.
var _ = context.Background
var _ = time.Second
