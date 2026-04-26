package views

import (
	"context"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

type AgentsOpenSessionMsg struct{ AgentID string }

type agentsLoadedMsg struct {
	agents []client.Agent
	err    error
}

type AgentsModel struct {
	client *client.Client
	table  components.Table
	agents []client.Agent
	err    error
}

func NewAgents(c *client.Client) AgentsModel {
	cols := []components.Column{
		{Title: "ID", Width: 16},
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Last Tick", Width: 12},
	}
	return AgentsModel{client: c, table: components.NewTable(cols, nil)}
}

func (m AgentsModel) Init() tea.Cmd { return m.fetch() }

func (m AgentsModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ag, err := c.Agents(ctx)
		return agentsLoadedMsg{agents: ag, err: err}
	}
}

func (m AgentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case agentsLoadedMsg:
		m.agents = msg.agents
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toAgentRows(msg.agents))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			id := m.selectedAgentID()
			if id == "" {
				return m, nil
			}
			return m, func() tea.Msg { return AgentsOpenSessionMsg{AgentID: id} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case client.Event:
		if msg.Kind == "agent_status" {
			return m, m.fetch()
		}
		return m, nil
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m AgentsModel) View() string {
	if m.err != nil {
		return "agents: error: " + m.err.Error()
	}
	if m.agents == nil {
		return "loading..."
	}
	return m.table.View()
}

func (m AgentsModel) selectedAgentID() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func toAgentRows(agents []client.Agent) [][]string {
	rows := make([][]string, len(agents))
	for i, a := range agents {
		last := ""
		if a.LastTickAt > 0 {
			last = strconv.FormatInt(a.LastTickAt, 10)
		}
		rows[i] = []string{a.AgentID, a.DisplayName, a.Status, last}
	}
	return rows
}
