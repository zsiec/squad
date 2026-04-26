package views

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

// EpicsDrillInMsg is emitted when the user presses Enter on an epic.
type EpicsDrillInMsg struct{ EpicName string }

type epicsLoadedMsg struct {
	epics []client.Epic
	err   error
}

type EpicsModel struct {
	client     *client.Client
	table      components.Table
	epics      []client.Epic
	err        error
	specFilter string
}

func NewEpics(c *client.Client) EpicsModel {
	cols := []components.Column{
		{Title: "Name", Width: 24},
		{Title: "Spec", Width: 16},
		{Title: "Status", Width: 10},
		{Title: "Parallelism", Width: 12},
	}
	return EpicsModel{client: c, table: components.NewTable(cols, nil)}
}

// SetSpecFilter sets the spec to filter by; refetch on next Init/refresh.
func (m EpicsModel) SetSpecFilter(spec string) EpicsModel {
	m.specFilter = spec
	return m
}

func (m EpicsModel) Init() tea.Cmd { return m.fetch() }

func (m EpicsModel) fetch() tea.Cmd {
	c := m.client
	spec := m.specFilter
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var opts *client.EpicListOpts
		if spec != "" {
			opts = &client.EpicListOpts{Spec: spec}
		}
		epics, err := c.Epics(ctx, opts)
		return epicsLoadedMsg{epics: epics, err: err}
	}
}

func (m EpicsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case epicsLoadedMsg:
		m.epics = msg.epics
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toEpicRows(msg.epics))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			name := m.selectedEpicName()
			if name == "" {
				return m, nil
			}
			return m, func() tea.Msg { return EpicsDrillInMsg{EpicName: name} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m EpicsModel) View() string {
	if m.err != nil {
		return "epics: error: " + m.err.Error()
	}
	if m.epics == nil {
		return "loading..."
	}
	return m.table.View()
}

func (m EpicsModel) selectedEpicName() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func toEpicRows(epics []client.Epic) [][]string {
	rows := make([][]string, len(epics))
	for i, e := range epics {
		rows[i] = []string{e.Name, e.Spec, e.Status, e.Parallelism}
	}
	return rows
}
