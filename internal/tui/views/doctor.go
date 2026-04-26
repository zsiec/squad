package views

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

const (
	stuckClaimAge    = 24 * time.Hour
	vanishedAgentAge = 1 * time.Hour
)

type Finding struct {
	Severity string
	Kind     string
	Subject  string
	Detail   string
	Age      int64
}

type DoctorJumpMsg struct{ Subject string }

type doctorLoadedMsg struct {
	claims []client.Claim
	agents []client.Agent
	err    error
}

type DoctorModel struct {
	client   *client.Client
	table    components.Table
	findings []Finding
	err      error
	loaded   bool
	nowFn    func() time.Time
}

func NewDoctor(c *client.Client) DoctorModel {
	return NewDoctorWithClock(c, time.Now)
}

func NewDoctorWithClock(c *client.Client, nowFn func() time.Time) DoctorModel {
	cols := []components.Column{
		{Title: "Sev", Width: 6},
		{Title: "Kind", Width: 18},
		{Title: "Subject", Width: 18},
		{Title: "Age", Width: 10},
		{Title: "Detail", Width: 50},
	}
	return DoctorModel{
		client: c,
		table:  components.NewTable(cols, nil),
		nowFn:  nowFn,
	}
}

func (m DoctorModel) Init() tea.Cmd { return m.fetch() }

func (m DoctorModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		claims, err := c.Claims(ctx)
		if err != nil {
			return doctorLoadedMsg{err: err}
		}
		agents, err := c.Agents(ctx)
		return doctorLoadedMsg{claims: claims, agents: agents, err: err}
	}
}

func (m DoctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case doctorLoadedMsg:
		m.err = msg.err
		m.loaded = true
		if msg.err == nil {
			m.findings = m.compute(msg.claims, msg.agents)
			m.table = m.table.SetRows(toDoctorRows(m.findings))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			subj := m.selectedSubject()
			if subj == "" {
				return m, nil
			}
			return m, func() tea.Msg { return DoctorJumpMsg{Subject: subj} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case client.Event:
		if msg.Kind == "item_changed" || msg.Kind == "claim_changed" || msg.Kind == "agent_status" {
			return m, m.fetch()
		}
		return m, nil
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m DoctorModel) View() string {
	if m.err != nil {
		return "doctor: error: " + m.err.Error()
	}
	if !m.loaded {
		return "loading..."
	}
	if len(m.findings) == 0 {
		return "(no findings)"
	}
	return m.table.View()
}

func (m DoctorModel) compute(claims []client.Claim, agents []client.Agent) []Finding {
	now := m.nowFn().Unix()
	out := []Finding{}
	for _, c := range claims {
		if c.ClaimedAt == 0 {
			continue
		}
		age := now - c.ClaimedAt
		if age > int64(stuckClaimAge.Seconds()) {
			subject := c.ItemID
			if subject == "" {
				subject = c.AgentID
			}
			out = append(out, Finding{
				Severity: "warn",
				Kind:     "stuck_claim",
				Subject:  subject,
				Detail:   fmt.Sprintf("claim by %s held for %s", c.AgentID, time.Duration(age)*time.Second),
				Age:      age,
			})
		}
	}
	activeAgents := map[string]struct{}{}
	for _, c := range claims {
		activeAgents[c.AgentID] = struct{}{}
	}
	for _, a := range agents {
		if _, ok := activeAgents[a.AgentID]; !ok {
			continue
		}
		if a.LastTickAt == 0 {
			continue
		}
		age := now - a.LastTickAt
		if age > int64(vanishedAgentAge.Seconds()) {
			out = append(out, Finding{
				Severity: "error",
				Kind:     "vanished_agent",
				Subject:  a.AgentID,
				Detail:   fmt.Sprintf("agent has open claim but hasn't ticked in %s", time.Duration(age)*time.Second),
				Age:      age,
			})
		}
	}
	return out
}

func (m DoctorModel) selectedSubject() string {
	row := m.table.SelectedRow()
	if len(row) < 3 {
		return ""
	}
	return row[2]
}

func toDoctorRows(findings []Finding) [][]string {
	rows := make([][]string, len(findings))
	for i, f := range findings {
		rows[i] = []string{
			f.Severity,
			f.Kind,
			f.Subject,
			(time.Duration(f.Age) * time.Second).String(),
			f.Detail,
		}
	}
	return rows
}
