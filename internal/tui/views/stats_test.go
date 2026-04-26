package views

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
)

const sampleStatsJSON = `{
  "schema_version": 1,
  "items": {
    "total": 42,
    "open": 10,
    "claimed": 5,
    "blocked": 2,
    "done": 25,
    "by_priority": {"high": 3, "normal": 28, "low": 11}
  },
  "claims": {
    "active": 5,
    "completed_in_window": 15,
    "duration_seconds": {"p50": 1800, "p90": 7200, "p99": 14400, "min": 60, "max": 18000, "count": 15}
  },
  "verification": {
    "rate": 0.85,
    "dones_with_full_evidence": 17,
    "dones_total": 20,
    "by_kind": {
      "test": {"attested": 18, "passed": 17},
      "lint": {"attested": 19, "passed": 19}
    }
  },
  "by_agent": [
    {"agent_id": "alice", "display_name": "Alice", "claims_completed": 10, "claim_p50_seconds": 1500},
    {"agent_id": "bob", "display_name": "Bob", "claims_completed": 5, "claim_p50_seconds": 2400}
  ],
  "by_epic": [
    {"epic": "auth", "items_total": 8, "items_done": 6, "verification_rate": 0.9}
  ]
}`

func statsFixture(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/stats") {
			_, _ = w.Write([]byte(sampleStatsJSON))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStats_InitFetchesAndRendersPanels(t *testing.T) {
	srv := statsFixture(t)
	c := client.New(srv.URL, "")
	m := NewStats(c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(StatsModel)
	updated, _ = m.Update(runCmd(t, m.Init()))
	mm := updated.(StatsModel)
	out := mm.View()

	// Items panel
	if !strings.Contains(out, "42") || !strings.Contains(out, "open") {
		t.Errorf("Items panel missing: %q", out)
	}
	// Claims panel — p50/p90/p99 visible
	if !strings.Contains(out, "p50") || !strings.Contains(out, "p99") {
		t.Errorf("Claims panel missing: %q", out)
	}
	// Verification panel — rate visible (e.g. "85%" or "0.85")
	if !strings.Contains(out, "85") {
		t.Errorf("Verification panel missing rate: %q", out)
	}
	// Per-agent panel — alice present
	if !strings.Contains(out, "alice") || !strings.Contains(out, "Alice") {
		t.Errorf("Per-agent panel missing: %q", out)
	}
}

func TestStats_TKeyTogglesBreakdown(t *testing.T) {
	srv := statsFixture(t)
	c := client.New(srv.URL, "")
	m := NewStats(c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(StatsModel)
	updated, _ = m.Update(runCmd(t, m.Init()))
	m = updated.(StatsModel)
	out := m.View()
	if !strings.Contains(out, "alice") {
		t.Fatalf("expected per-agent default: %q", out)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	mm := updated.(StatsModel)
	out = mm.View()
	if !strings.Contains(out, "auth") {
		t.Errorf("expected per-epic after t toggle: %q", out)
	}
	if strings.Contains(out, "alice") {
		t.Errorf("per-agent should be hidden after toggle: %q", out)
	}
}

func TestStats_WKeyCyclesWindow(t *testing.T) {
	gotURLs := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/stats") {
			gotURLs = append(gotURLs, r.URL.RequestURI())
			_, _ = w.Write([]byte(sampleStatsJSON))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := client.New(srv.URL, "")
	m := NewStats(c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(StatsModel)
	updated, _ = m.Update(runCmd(t, m.Init()))
	m = updated.(StatsModel)
	// Press w — should cycle to next window and refetch
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(StatsModel)
	if cmd == nil {
		t.Fatal("expected cmd from w key")
	}
	_ = cmd()
	// At least 2 fetches: Init + post-w
	if len(gotURLs) < 2 {
		t.Fatalf("got URLs=%v, expected >=2 fetches", gotURLs)
	}
	// Second URL should have a different window= than first
	if gotURLs[0] == gotURLs[1] {
		t.Fatalf("window did not change: %v", gotURLs)
	}
}

func TestStats_RefreshMsgRefetches(t *testing.T) {
	srv := statsFixture(t)
	c := client.New(srv.URL, "")
	m := NewStats(c)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(StatsModel)
	updated, _ = m.Update(runCmd(t, m.Init()))
	m = updated.(StatsModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}
