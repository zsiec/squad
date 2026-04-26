package views

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

type MailboxOpenMsg struct{ Thread string }
type MailboxReplyMsg struct {
	Thread string
	To     string
}

type mailboxLoadedMsg struct {
	msgs []client.Message
	me   string
	err  error
}

type MailboxModel struct {
	client   *client.Client
	table    components.Table
	err      error
	me       string
	msgs     []client.Message
	filtered []client.Message
	loaded   bool
}

func NewMailbox(c *client.Client) MailboxModel {
	cols := []components.Column{
		{Title: "Time", Width: 10},
		{Title: "From", Width: 16},
		{Title: "Thread", Width: 16},
		{Title: "Preview", Width: 50},
	}
	return MailboxModel{client: c, table: components.NewTable(cols, nil)}
}

func (m MailboxModel) Init() tea.Cmd { return m.fetch() }

func (m MailboxModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		who, whoErr := c.Whoami(ctx)
		if whoErr != nil {
			return mailboxLoadedMsg{err: whoErr}
		}
		msgs, err := c.Messages(ctx, &client.MessagesOpts{Limit: 500})
		return mailboxLoadedMsg{msgs: msgs, me: who.AgentID, err: err}
	}
}

func (m MailboxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case mailboxLoadedMsg:
		m.msgs = msg.msgs
		m.me = msg.me
		m.err = msg.err
		m.loaded = true
		if msg.err == nil {
			m.filtered = filterForMe(msg.msgs, msg.me)
			m.table = m.table.SetRows(toMailboxRows(m.filtered))
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			cur := m.selected()
			if cur == nil {
				return m, nil
			}
			thread := cur.Thread
			return m, func() tea.Msg { return MailboxOpenMsg{Thread: thread} }
		case "r":
			cur := m.selected()
			if cur == nil {
				return m, nil
			}
			thread, to := cur.Thread, cur.AgentID
			return m, func() tea.Msg { return MailboxReplyMsg{Thread: thread, To: to} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case client.Event:
		if msg.Kind == "message" {
			return m, m.fetch()
		}
		return m, nil
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m MailboxModel) View() string {
	if m.err != nil {
		return "mailbox: error: " + m.err.Error()
	}
	if !m.loaded {
		return "loading..."
	}
	if len(m.filtered) == 0 {
		return "(no messages addressed to you)"
	}
	return m.table.View()
}

func (m MailboxModel) selected() *client.Message {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.filtered) {
		return nil
	}
	c := m.filtered[idx]
	return &c
}

func filterForMe(msgs []client.Message, me string) []client.Message {
	if me == "" {
		return msgs
	}
	needle := "@" + me
	out := make([]client.Message, 0, len(msgs))
	for _, m := range msgs {
		if strings.Contains(m.Body, needle) || m.Thread == me {
			out = append(out, m)
		}
	}
	return out
}

func toMailboxRows(msgs []client.Message) [][]string {
	rows := make([][]string, len(msgs))
	for i, msg := range msgs {
		ts := time.Unix(msg.TS, 0).Format("15:04:05")
		preview := msg.Body
		if len(preview) > 60 {
			preview = preview[:60]
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		rows[i] = []string{ts, msg.AgentID, msg.Thread, preview}
	}
	return rows
}
