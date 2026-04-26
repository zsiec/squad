package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/components"
	"github.com/zsiec/squad/internal/tui/theme"
	"github.com/zsiec/squad/internal/tui/views"
)

type View int

const (
	ViewItems View = iota
	ViewAgents
	ViewChat
	ViewRepos
	ViewHistory
	ViewMailbox
	ViewDoctor
	ViewSpecs
	ViewEpics
	ViewStats
	ViewLearnings
	ViewSession
)

func (v View) Name() string {
	switch v {
	case ViewItems:
		return "items"
	case ViewAgents:
		return "agents"
	case ViewChat:
		return "chat"
	case ViewRepos:
		return "repos"
	case ViewHistory:
		return "history"
	case ViewMailbox:
		return "mailbox"
	case ViewDoctor:
		return "doctor"
	case ViewSpecs:
		return "specs"
	case ViewEpics:
		return "epics"
	case ViewStats:
		return "stats"
	case ViewLearnings:
		return "learnings"
	case ViewSession:
		return "session"
	}
	return "?"
}

var numberKeyView = map[rune]View{
	'1': ViewItems, '2': ViewAgents, '3': ViewChat, '4': ViewRepos,
	'5': ViewHistory, '6': ViewMailbox, '7': ViewDoctor, '8': ViewSpecs,
	'9': ViewEpics, '0': ViewStats,
}

// StubView renders only its view name. Tasks 11-22 replace each entry
// with the real view's Model.
type StubView struct{ label string }

func (s StubView) Init() tea.Cmd                           { return nil }
func (s StubView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s StubView) View() string                            { return s.label }

// Model is the root bubbletea model.
type Model struct {
	client  *client.Client
	width   int
	height  int
	current View
	views   map[View]tea.Model
	palette components.Palette
	eventCh <-chan client.Event
	scope   string
}

// NewModel constructs a root model. eventCh may be nil for testing.
func NewModel(c *client.Client, eventCh <-chan client.Event, scope string) Model {
	views := map[View]tea.Model{}
	for v := ViewItems; v <= ViewSession; v++ {
		views[v] = StubView{label: v.Name() + " view (placeholder)"}
	}
	cmds := []components.Command{}
	for v := ViewItems; v <= ViewSession; v++ {
		cmds = append(cmds, components.Command{Name: v.Name(), Description: v.Name() + " view"})
	}
	return Model{
		client:  c,
		current: ViewItems,
		views:   views,
		palette: components.NewPalette(cmds, ""),
		eventCh: eventCh,
		scope:   scope,
	}
}

func (m Model) Current() View       { return m.current }
func (m Model) Width() int          { return m.width }
func (m Model) Height() int         { return m.height }
func (m Model) PaletteActive() bool { return m.palette.IsActive() }

// SetCurrent changes the active view. Returns a new Model.
func (m Model) SetCurrent(v View) Model {
	m.current = v
	return m
}

// WithView replaces the model registered for the given View. Used by
// app.go (real views) and tests (recording stubs). Returns a new Model.
func (m Model) WithView(v View, mod tea.Model) Model {
	views := make(map[View]tea.Model, len(m.views))
	for k, vv := range m.views {
		views[k] = vv
	}
	views[v] = mod
	m.views = views
	return m
}

// paletteSelMsgFor wraps components.PaletteSelectedMsg for in-package tests.
func paletteSelMsgFor(cmd string) tea.Msg {
	return components.PaletteSelectedMsg{Command: cmd}
}

func (m Model) Init() tea.Cmd {
	if m.eventCh == nil {
		return nil
	}
	return waitForEvent(m.eventCh)
}

// waitForEvent reads one event from the channel and emits it as a tea.Msg.
// The Update handler re-arms after delivering each event.
func waitForEvent(ch <-chan client.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return ev
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		var cmd tea.Cmd
		m.views[m.current], cmd = m.views[m.current].Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.palette.IsActive() {
			var cmd tea.Cmd
			m.palette, cmd = m.palette.Update(msg)
			return m, cmd
		}
		s := msg.String()
		if s == ":" {
			m.palette = m.palette.Open()
			return m, nil
		}
		if s == "esc" {
			var cmd tea.Cmd
			m.views[m.current], cmd = m.views[m.current].Update(msg)
			return m, cmd
		}
		if s == "q" || s == "ctrl+c" {
			return m, tea.Quit
		}
		if s == "ctrl+r" {
			var cmd tea.Cmd
			m.views[m.current], cmd = m.views[m.current].Update(views.RefreshMsg{})
			return m, cmd
		}
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			if v, ok := numberKeyView[msg.Runes[0]]; ok {
				m.current = v
				return m, m.views[v].Init()
			}
		}
		var cmd tea.Cmd
		m.views[m.current], cmd = m.views[m.current].Update(msg)
		return m, cmd

	case components.PaletteSelectedMsg:
		for v := ViewItems; v <= ViewSession; v++ {
			if v.Name() == msg.Command {
				m.current = v
				return m, m.views[v].Init()
			}
		}
		var cmd tea.Cmd
		m.views[m.current], cmd = m.views[m.current].Update(msg)
		return m, cmd

	case client.Event:
		var cmd tea.Cmd
		m.views[m.current], cmd = m.views[m.current].Update(msg)
		batch := []tea.Cmd{}
		if cmd != nil {
			batch = append(batch, cmd)
		}
		if m.eventCh != nil {
			batch = append(batch, waitForEvent(m.eventCh))
		}
		return m, tea.Batch(batch...)
	}

	var cmd tea.Cmd
	m.views[m.current], cmd = m.views[m.current].Update(msg)
	return m, cmd
}

func (m Model) View() string {
	body := m.views[m.current].View()
	state := components.StatusBarState{
		Scope: m.scope,
		View:  m.current.Name(),
		Conn:  components.ConnUp,
		Width: m.width,
	}
	bar := components.StatusBar(state)
	if m.palette.IsActive() {
		divider := theme.StatusBar.Render(strings.Repeat("─", maxInt(m.width, 1)))
		return m.palette.View() + "\n" + divider + "\n" + body + "\n" + bar
	}
	return body + "\n" + bar
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
