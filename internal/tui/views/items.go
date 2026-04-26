// Package views hosts the bubbletea Models for each TUI screen.
// Each view is its own value-type Model; the root model in
// internal/tui composes them into a single program.
package views

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

// RefreshMsg is sent by the root model on Ctrl-R; views pattern-match.
// Defined here (not in tui package) to avoid views→tui import cycle.
type RefreshMsg struct{}

// ItemsDrillInMsg is emitted when the user presses Enter on an item.
type ItemsDrillInMsg struct{ ItemID string }

// ItemsOpenSessionMsg is emitted when the user presses 'm' on a claimed item.
type ItemsOpenSessionMsg struct{ AgentID string }

// ItemsClaimedMsg / ItemsReleasedMsg are success notifications view modules emit.
type ItemsClaimedMsg struct{ ItemID string }
type ItemsReleasedMsg struct{ ItemID string }

// itemsLoadedMsg is internal — carries fetch results back to Update.
type itemsLoadedMsg struct {
	items []client.Item
	err   error
}

type ItemsModel struct {
	client *client.Client
	table  components.Table
	items  []client.Item
	err    error
	toast  string
}

// NewItems constructs an ItemsModel with empty rows.
func NewItems(c *client.Client) ItemsModel {
	cols := []components.Column{
		{Title: "ID", Width: 12},
		{Title: "Title", Width: 32},
		{Title: "Status", Width: 9},
		{Title: "Epic", Width: 12},
		{Title: "Deps", Width: 5},
		{Title: "P", Width: 2},
		{Title: "Ev", Width: 4},
		{Title: "Claimed by", Width: 14},
	}
	return ItemsModel{
		client: c,
		table:  components.NewTable(cols, nil),
	}
}

// Init kicks off the initial fetch.
func (m ItemsModel) Init() tea.Cmd {
	return m.fetch()
}

func (m ItemsModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		items, err := c.Items(ctx, nil)
		return itemsLoadedMsg{items: items, err: err}
	}
}

// Update handles loaded data, key actions, SSE events, and refresh requests.
func (m ItemsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case itemsLoadedMsg:
		m.items = msg.items
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toItemsRows(msg.items))
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			id := m.selectedItemID()
			if id == "" {
				return m, nil
			}
			return m, m.claim(id)
		case "r":
			id := m.selectedItemID()
			if id == "" {
				return m, nil
			}
			return m, m.release(id)
		case "enter":
			id := m.selectedItemID()
			if id == "" {
				return m, nil
			}
			return m, func() tea.Msg { return ItemsDrillInMsg{ItemID: id} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case client.Event:
		if msg.Kind == "item_changed" || msg.Kind == "claim_changed" {
			return m, m.fetch()
		}
		return m, nil

	case RefreshMsg:
		return m, m.fetch()
	}

	return m, nil
}

func (m ItemsModel) View() string {
	if m.err != nil {
		return "items: error: " + m.err.Error()
	}
	if !m.loaded() {
		return "loading..."
	}
	out := m.table.View()
	if m.toast != "" {
		out += "\n" + m.toast
	}
	return out
}

func (m ItemsModel) loaded() bool {
	return m.items != nil || m.err != nil
}

func (m ItemsModel) selectedItemID() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func (m ItemsModel) claim(id string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.Claim(ctx, id, nil); err != nil {
			return itemsLoadedMsg{err: err}
		}
		return ItemsClaimedMsg{ItemID: id}
	}
}

func (m ItemsModel) release(id string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.Release(ctx, id, ""); err != nil {
			return itemsLoadedMsg{err: err}
		}
		return ItemsReleasedMsg{ItemID: id}
	}
}

func toItemsRows(items []client.Item) [][]string {
	rows := make([][]string, len(items))
	for i, it := range items {
		depsStr := ""
		if len(it.DependsOn) > 0 {
			depsStr = fmt.Sprintf("%d", len(it.DependsOn))
		}
		parStr := ""
		if it.Parallel {
			parStr = "✓"
		}
		evStr := ""
		if len(it.EvidenceRequired) > 0 {
			evStr = fmt.Sprintf("%d", len(it.EvidenceRequired))
		}
		rows[i] = []string{
			it.ID,
			it.Title,
			it.Status,
			it.Epic,
			depsStr,
			parStr,
			evStr,
			it.ClaimedBy,
		}
	}
	return rows
}
