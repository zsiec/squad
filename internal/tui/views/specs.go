package views

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

// SpecsDrillInMsg is emitted when the user presses Enter on a spec.
type SpecsDrillInMsg struct{ SpecName string }

type specsLoadedMsg struct {
	specs []client.Spec
	err   error
}

type SpecsModel struct {
	client *client.Client
	table  components.Table
	specs  []client.Spec
	err    error
}

func NewSpecs(c *client.Client) SpecsModel {
	cols := []components.Column{
		{Title: "Name", Width: 20},
		{Title: "Title", Width: 40},
		{Title: "Path", Width: 32},
	}
	return SpecsModel{client: c, table: components.NewTable(cols, nil)}
}

func (m SpecsModel) Init() tea.Cmd { return m.fetch() }

func (m SpecsModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		specs, err := c.Specs(ctx)
		return specsLoadedMsg{specs: specs, err: err}
	}
}

func (m SpecsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case specsLoadedMsg:
		m.specs = msg.specs
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toSpecRows(msg.specs))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			name := m.selectedSpecName()
			if name == "" {
				return m, nil
			}
			return m, func() tea.Msg { return SpecsDrillInMsg{SpecName: name} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m SpecsModel) View() string {
	if m.err != nil {
		return "specs: error: " + m.err.Error()
	}
	if m.specs == nil {
		return "loading..."
	}
	return m.table.View()
}

func (m SpecsModel) selectedSpecName() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func toSpecRows(specs []client.Spec) [][]string {
	rows := make([][]string, len(specs))
	for i, s := range specs {
		rows[i] = []string{s.Name, s.Title, s.Path}
	}
	return rows
}
