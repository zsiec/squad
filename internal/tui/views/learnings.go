package views

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

type LearningsJumpToItemMsg struct{ ItemID string }

type learningsLoadedMsg struct {
	learnings []client.Learning
	err       error
}

type LearningsModel struct {
	client    *client.Client
	table     components.Table
	learnings []client.Learning
	err       error
}

func NewLearnings(c *client.Client) LearningsModel {
	cols := []components.Column{
		{Title: "Slug", Width: 24},
		{Title: "Kind", Width: 10},
		{Title: "State", Width: 10},
		{Title: "Area", Width: 12},
		{Title: "Title", Width: 36},
	}
	return LearningsModel{client: c, table: components.NewTable(cols, nil)}
}

func (m LearningsModel) Init() tea.Cmd { return m.fetch() }

func (m LearningsModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l, err := c.Learnings(ctx, nil)
		return learningsLoadedMsg{learnings: l, err: err}
	}
}

func (m LearningsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case learningsLoadedMsg:
		m.learnings = msg.learnings
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toLearningRows(msg.learnings))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			row := m.table.Cursor()
			if row < 0 || row >= len(m.learnings) {
				return m, nil
			}
			l := m.learnings[row]
			if len(l.Related) == 0 {
				return m, nil
			}
			itemID := l.Related[0]
			return m, func() tea.Msg { return LearningsJumpToItemMsg{ItemID: itemID} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case client.Event:
		if msg.Kind == "learning_state_changed" {
			return m, m.fetch()
		}
		return m, nil
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m LearningsModel) View() string {
	if m.err != nil {
		return "learnings: error: " + m.err.Error()
	}
	if m.learnings == nil {
		return "loading..."
	}
	return m.table.View()
}

func toLearningRows(ls []client.Learning) [][]string {
	rows := make([][]string, len(ls))
	for i, l := range ls {
		rows[i] = []string{l.Slug, l.Kind, l.State, l.Area, l.Title}
	}
	return rows
}
