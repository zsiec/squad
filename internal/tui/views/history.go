package views

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

type HistoryJumpToItemMsg struct{ ItemID string }

type historyLoadedMsg struct {
	msgs []client.Message
	err  error
}

type HistoryModel struct {
	client *client.Client
	table  components.Table
	msgs   []client.Message
	err    error
}

func NewHistory(c *client.Client) HistoryModel {
	cols := []components.Column{
		{Title: "Time", Width: 10},
		{Title: "Agent", Width: 16},
		{Title: "Kind", Width: 10},
		{Title: "Thread", Width: 16},
		{Title: "Preview", Width: 40},
	}
	return HistoryModel{client: c, table: components.NewTable(cols, nil)}
}

func (m HistoryModel) Init() tea.Cmd { return m.fetch() }

func (m HistoryModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msgs, err := c.Messages(ctx, &client.MessagesOpts{Limit: 500})
		return historyLoadedMsg{msgs: msgs, err: err}
	}
}

func (m HistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		m.msgs = msg.msgs
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toHistoryRows(msg.msgs))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			thread := m.selectedThread()
			if thread == "" || !itemThreadRe.MatchString(thread) {
				return m, nil
			}
			return m, func() tea.Msg { return HistoryJumpToItemMsg{ItemID: thread} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case client.Event:
		if msg.Kind == "message" || msg.Kind == "item_changed" {
			return m, m.fetch()
		}
		return m, nil
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m HistoryModel) View() string {
	if m.err != nil {
		return "history: error: " + m.err.Error()
	}
	if m.msgs == nil {
		return "loading..."
	}
	return m.table.View()
}

func (m HistoryModel) selectedThread() string {
	row := m.table.SelectedRow()
	if len(row) < 4 {
		return ""
	}
	return row[3]
}

func toHistoryRows(msgs []client.Message) [][]string {
	rows := make([][]string, len(msgs))
	for i, msg := range msgs {
		ts := time.Unix(msg.TS, 0).Format("15:04:05")
		preview := msg.Body
		if len(preview) > 60 {
			preview = preview[:60]
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		rows[i] = []string{ts, msg.AgentID, msg.Kind, msg.Thread, preview}
	}
	return rows
}
