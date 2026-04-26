package views

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

type inboxLoadedMsg struct {
	entries []client.InboxEntry
	err     error
}

type inboxAcceptResultMsg struct {
	ID  string
	Err error
}

type inboxRejectResultMsg struct {
	ID  string
	Err error
}

type InboxModel struct {
	client      *client.Client
	table       components.Table
	entries     []client.InboxEntry
	err         error
	showError   string
	rejectModal *components.ReasonModal
	rejectingID string
}

func NewInbox(c *client.Client) InboxModel {
	cols := []components.Column{
		{Title: "ID", Width: 12},
		{Title: "Title", Width: 36},
		{Title: "Captured by", Width: 14},
		{Title: "Spec", Width: 12},
		{Title: "DoR", Width: 4},
	}
	return InboxModel{client: c, table: components.NewTable(cols, nil)}
}

func (m InboxModel) Init() tea.Cmd { return m.fetch() }

func (m InboxModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		entries, err := c.Inbox(ctx, client.InboxOpts{})
		return inboxLoadedMsg{entries: entries, err: err}
	}
}

func (m InboxModel) InReject() bool      { return m.rejectModal != nil }
func (m InboxModel) RejectingID() string { return m.rejectingID }
func (m InboxModel) RejectModal() *components.ReasonModal {
	return m.rejectModal
}

func (m InboxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case inboxLoadedMsg:
		m.entries = msg.entries
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toInboxRows(msg.entries))
		}
		return m, nil

	case inboxAcceptResultMsg:
		if msg.Err != nil {
			var dorErr *client.DoRViolationsError
			if errors.As(msg.Err, &dorErr) {
				m.showError = formatViolations(dorErr.Violations)
			} else {
				m.showError = msg.Err.Error()
			}
			return m, nil
		}
		m.showError = ""
		return m, m.fetch()

	case inboxRejectResultMsg:
		if msg.Err != nil {
			m.showError = msg.Err.Error()
			return m, nil
		}
		m.showError = ""
		return m, m.fetch()

	case tea.KeyMsg:
		if m.rejectModal != nil {
			return m.updateReject(msg)
		}
		switch msg.String() {
		case "a":
			id := m.selectedID()
			if id == "" {
				return m, nil
			}
			m.showError = ""
			return m, m.acceptCmd(id)
		case "r":
			id := m.selectedID()
			if id == "" {
				return m, nil
			}
			modal := components.NewReasonModal()
			m.rejectModal = &modal
			m.rejectingID = id
			m.showError = ""
			return m, nil
		case "e":
			path := m.selectedPath()
			if path == "" {
				return m, nil
			}
			return m, m.editCmd(path)
		case "R":
			return m, m.fetch()
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case client.Event:
		if msg.Kind == "inbox_changed" {
			return m, m.fetch()
		}
		return m, nil

	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m InboxModel) updateReject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, _ := m.rejectModal.Update(msg)
	m.rejectModal = &updated
	if updated.Submitted() {
		id, reason := m.rejectingID, updated.Value()
		m.rejectModal = nil
		m.rejectingID = ""
		return m, m.rejectCmd(id, reason)
	}
	if updated.Cancelled() {
		m.rejectModal = nil
		m.rejectingID = ""
		return m, nil
	}
	return m, nil
}

func (m InboxModel) View() string {
	if m.err != nil {
		return "inbox: error: " + m.err.Error()
	}
	if m.entries == nil {
		return "loading..."
	}
	out := m.table.View()
	if m.rejectModal != nil {
		out += "\n" + m.rejectModal.View()
		out += "\n(Enter=submit  Esc=cancel)"
	}
	if m.showError != "" {
		out += "\n" + m.showError
	}
	return out
}

func (m InboxModel) selectedID() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func (m InboxModel) selectedPath() string {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.entries) {
		return ""
	}
	return m.entries[idx].Path
}

func (m InboxModel) acceptCmd(id string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := c.Accept(ctx, id)
		return inboxAcceptResultMsg{ID: id, Err: err}
	}
}

func (m InboxModel) rejectCmd(id, reason string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := c.Reject(ctx, id, reason)
		return inboxRejectResultMsg{ID: id, Err: err}
	}
}

func (m InboxModel) editCmd(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	refetch := m.fetch()
	return tea.ExecProcess(cmd, func(error) tea.Msg {
		return refetch()
	})
}

func formatViolations(vs []client.DoRViolation) string {
	if len(vs) == 0 {
		return "definition of ready failed"
	}
	lines := make([]string, 0, len(vs)+1)
	lines = append(lines, fmt.Sprintf("definition of ready failed (%d):", len(vs)))
	for _, v := range vs {
		field := v.Field
		if field != "" {
			field = " [" + field + "]"
		}
		lines = append(lines, "  - "+v.Rule+field+": "+v.Message)
	}
	return strings.Join(lines, "\n")
}

func toInboxRows(entries []client.InboxEntry) [][]string {
	rows := make([][]string, len(entries))
	for i, e := range entries {
		dor := "x"
		if e.DoRPass {
			dor = "ok"
		}
		rows[i] = []string{e.ID, e.Title, e.CapturedBy, e.ParentSpec, dor}
	}
	return rows
}
