package views

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zsiec/squad/internal/tui/client"
)

var statsWindows = []int64{3600, 86400, 604800, 2592000}

type statsLoadedMsg struct {
	snapshot client.Stats
	err      error
}

type StatsModel struct {
	client    *client.Client
	snapshot  client.Stats
	loaded    bool
	windowSec int64
	breakdown string
	err       error
	width     int
	height    int
}

func NewStats(c *client.Client) StatsModel {
	return StatsModel{
		client:    c,
		windowSec: 86400,
		breakdown: "agent",
		width:     80,
		height:    24,
	}
}

func (m StatsModel) Init() tea.Cmd { return m.fetch() }

func (m StatsModel) fetch() tea.Cmd {
	c := m.client
	w := m.windowSec
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s, err := c.Stats(ctx, w)
		return statsLoadedMsg{snapshot: s, err: err}
	}
}

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsLoadedMsg:
		m.snapshot = msg.snapshot
		m.loaded = true
		m.err = msg.err
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			if m.breakdown == "agent" {
				m.breakdown = "epic"
			} else {
				m.breakdown = "agent"
			}
			return m, nil
		case "w":
			cur := m.windowSec
			next := statsWindows[0]
			for i, w := range statsWindows {
				if w == cur {
					next = statsWindows[(i+1)%len(statsWindows)]
					break
				}
			}
			m.windowSec = next
			return m, m.fetch()
		}
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m StatsModel) View() string {
	if m.err != nil {
		return "stats: error: " + m.err.Error()
	}
	if !m.loaded {
		return "loading stats..."
	}

	panelWidth := (m.width - 4) / 2
	if panelWidth < 30 {
		panelWidth = 30
	}
	panelHeight := (m.height - 6) / 2
	if panelHeight < 6 {
		panelHeight = 6
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Width(panelWidth).
		Height(panelHeight).
		Padding(0, 1)

	tl := border.Render(m.renderItemsPanel())
	tr := border.Render(m.renderClaimsPanel())
	bl := border.Render(m.renderVerificationPanel())
	br := border.Render(m.renderBreakdownPanel())

	top := lipgloss.JoinHorizontal(lipgloss.Top, tl, tr)
	bot := lipgloss.JoinHorizontal(lipgloss.Top, bl, br)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		top,
		bot,
		fmt.Sprintf("window: %s   t toggle breakdown   w cycle window", windowLabel(m.windowSec)),
	)
}

func windowLabel(sec int64) string {
	switch sec {
	case 3600:
		return "1h"
	case 86400:
		return "24h"
	case 604800:
		return "7d"
	case 2592000:
		return "30d"
	}
	return fmt.Sprintf("%ds", sec)
}

func (m StatsModel) renderItemsPanel() string {
	var p struct {
		Total      int            `json:"total"`
		Open       int            `json:"open"`
		Claimed    int            `json:"claimed"`
		Blocked    int            `json:"blocked"`
		Done       int            `json:"done"`
		ByPriority map[string]int `json:"by_priority"`
	}
	_ = json.Unmarshal(m.snapshot.Items, &p)
	var b strings.Builder
	b.WriteString("Items\n")
	fmt.Fprintf(&b, "  total: %d\n", p.Total)
	fmt.Fprintf(&b, "  open: %d  claimed: %d  blocked: %d  done: %d", p.Open, p.Claimed, p.Blocked, p.Done)
	return b.String()
}

func (m StatsModel) renderClaimsPanel() string {
	var p struct {
		Active            int `json:"active"`
		CompletedInWindow int `json:"completed_in_window"`
		DurationSeconds   struct {
			P50   int64 `json:"p50"`
			P90   int64 `json:"p90"`
			P99   int64 `json:"p99"`
			Min   int64 `json:"min"`
			Max   int64 `json:"max"`
			Count int64 `json:"count"`
		} `json:"duration_seconds"`
	}
	_ = json.Unmarshal(m.snapshot.Claims, &p)
	var b strings.Builder
	b.WriteString("Claims\n")
	fmt.Fprintf(&b, "  active: %d   completed: %d\n", p.Active, p.CompletedInWindow)
	fmt.Fprintf(&b, "  p50: %s\n", time.Duration(p.DurationSeconds.P50)*time.Second)
	fmt.Fprintf(&b, "  p90: %s\n", time.Duration(p.DurationSeconds.P90)*time.Second)
	fmt.Fprintf(&b, "  p99: %s", time.Duration(p.DurationSeconds.P99)*time.Second)
	return b.String()
}

func (m StatsModel) renderVerificationPanel() string {
	var p struct {
		Rate                  *float64 `json:"rate"`
		DonesWithFullEvidence int      `json:"dones_with_full_evidence"`
		DonesTotal            int      `json:"dones_total"`
		ByKind                map[string]struct {
			Attested int `json:"attested"`
			Passed   int `json:"passed"`
		} `json:"by_kind"`
	}
	_ = json.Unmarshal(m.snapshot.Verification, &p)
	var b strings.Builder
	b.WriteString("Verification\n")
	if p.Rate != nil {
		fmt.Fprintf(&b, "  rate: %.0f%% [%s]\n", *p.Rate*100, asciiBar(*p.Rate, 16))
	} else {
		b.WriteString("  rate: n/a\n")
	}
	fmt.Fprintf(&b, "  %d/%d dones with full evidence", p.DonesWithFullEvidence, p.DonesTotal)
	return b.String()
}

func (m StatsModel) renderBreakdownPanel() string {
	if m.breakdown == "epic" {
		var rows []struct {
			Epic             string  `json:"epic"`
			ItemsTotal       int     `json:"items_total"`
			ItemsDone        int     `json:"items_done"`
			VerificationRate float64 `json:"verification_rate"`
		}
		_ = json.Unmarshal(m.snapshot.ByEpic, &rows)
		var b strings.Builder
		b.WriteString("By epic\n")
		for i, r := range rows {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "  %s: %d/%d done\n", r.Epic, r.ItemsDone, r.ItemsTotal)
		}
		return b.String()
	}
	var rows []struct {
		AgentID         string `json:"agent_id"`
		DisplayName     string `json:"display_name"`
		ClaimsCompleted int    `json:"claims_completed"`
		ClaimP50Seconds int64  `json:"claim_p50_seconds"`
	}
	_ = json.Unmarshal(m.snapshot.ByAgent, &rows)
	var b strings.Builder
	b.WriteString("By agent\n")
	for i, r := range rows {
		if i >= 5 {
			break
		}
		fmt.Fprintf(&b, "  %s (%s): %d completed\n", r.DisplayName, r.AgentID, r.ClaimsCompleted)
	}
	return b.String()
}

func asciiBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	full := int(pct * float64(width))
	out := strings.Repeat("█", full)
	if full < width {
		partial := pct*float64(width) - float64(full)
		switch {
		case partial > 0.875:
			out += "▉"
		case partial > 0.75:
			out += "▊"
		case partial > 0.625:
			out += "▋"
		case partial > 0.5:
			out += "▌"
		case partial > 0.375:
			out += "▍"
		case partial > 0.25:
			out += "▎"
		case partial > 0.125:
			out += "▏"
		}
	}
	pad := width - lipgloss.Width(out)
	if pad < 0 {
		pad = 0
	}
	return out + strings.Repeat(" ", pad)
}
