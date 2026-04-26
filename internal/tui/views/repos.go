package views

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
)

type ReposScopeMsg struct{ RepoID string }

type reposLoadedMsg struct {
	repos []client.Repo
	err   error
}

type ReposModel struct {
	client *client.Client
	table  components.Table
	repos  []client.Repo
	err    error
}

func NewRepos(c *client.Client) ReposModel {
	cols := []components.Column{
		{Title: "Repo", Width: 20},
		{Title: "Path", Width: 32},
		{Title: "Remote", Width: 32},
	}
	return ReposModel{client: c, table: components.NewTable(cols, nil)}
}

func (m ReposModel) Init() tea.Cmd { return m.fetch() }

func (m ReposModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		repos, err := c.Repos(ctx)
		return reposLoadedMsg{repos: repos, err: err}
	}
}

func (m ReposModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reposLoadedMsg:
		m.repos = msg.repos
		m.err = msg.err
		if msg.err == nil {
			m.table = m.table.SetRows(toRepoRows(msg.repos))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			id := m.selectedRepoID()
			if id == "" {
				return m, nil
			}
			return m, func() tea.Msg { return ReposScopeMsg{RepoID: id} }
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	case client.Event:
		if msg.Kind == "item_changed" {
			return m, m.fetch()
		}
		return m, nil
	case RefreshMsg:
		return m, m.fetch()
	}
	return m, nil
}

func (m ReposModel) View() string {
	if m.err != nil {
		return "repos: error: " + m.err.Error()
	}
	if m.repos == nil {
		return "loading..."
	}
	return m.table.View()
}

func (m ReposModel) selectedRepoID() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func toRepoRows(repos []client.Repo) [][]string {
	rows := make([][]string, len(repos))
	for i, r := range repos {
		rows[i] = []string{r.RepoID, r.Path, r.Remote}
	}
	return rows
}
