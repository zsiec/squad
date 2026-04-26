package components

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zsiec/squad/internal/tui/theme"
)

const maxHistory = 200

type Command struct {
	Name        string
	Description string
}

type PaletteSelectedMsg struct {
	Command string
}

type Palette struct {
	commands    []Command
	matches     []Command
	cursor      int
	input       string
	active      bool
	history     []string
	historyPath string
}

func NewPalette(commands []Command, historyPath string) Palette {
	p := Palette{
		commands:    commands,
		matches:     append([]Command(nil), commands...),
		historyPath: historyPath,
	}
	if historyPath != "" {
		if data, err := os.ReadFile(historyPath); err == nil {
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			if len(lines) > maxHistory {
				lines = lines[len(lines)-maxHistory:]
			}
			p.history = lines
		}
	}
	return p
}

func (p Palette) Open() Palette {
	p.active = true
	p.input = ""
	p.cursor = 0
	p.matches = append([]Command(nil), p.commands...)
	return p
}

func (p Palette) Close() Palette {
	p.active = false
	return p
}

func (p Palette) IsActive() bool             { return p.active }
func (p Palette) Cursor() int                { return p.cursor }
func (p Palette) VisibleMatches() []Command  { return p.matches }
func (p Palette) HistorySize() int           { return len(p.history) }

func (p Palette) HasHistory(name string) bool {
	for _, h := range p.history {
		if h == name {
			return true
		}
	}
	return false
}

func (p Palette) Update(msg tea.Msg) (Palette, tea.Cmd) {
	if !p.active {
		return p, nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch k.Type {
	case tea.KeyEsc:
		p = p.Close()
		return p, nil
	case tea.KeyEnter:
		if len(p.matches) == 0 || p.cursor >= len(p.matches) {
			p = p.Close()
			return p, nil
		}
		chosen := p.matches[p.cursor].Name
		p.history = append(p.history, chosen)
		if len(p.history) > maxHistory {
			p.history = p.history[len(p.history)-maxHistory:]
		}
		if p.historyPath != "" {
			data := strings.Join(p.history, "\n") + "\n"
			tmp := p.historyPath + ".tmp"
			if err := os.WriteFile(tmp, []byte(data), 0o600); err == nil {
				_ = os.Rename(tmp, p.historyPath)
			}
		}
		p = p.Close()
		return p, func() tea.Msg { return PaletteSelectedMsg{Command: chosen} }
	case tea.KeyDown:
		if p.cursor < len(p.matches)-1 {
			p.cursor++
		}
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
	case tea.KeyBackspace:
		if len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
			p.refilter()
		}
	case tea.KeyRunes:
		p.input += string(k.Runes)
		p.refilter()
	}
	return p, nil
}

func (p *Palette) refilter() {
	if p.input == "" {
		p.matches = append([]Command(nil), p.commands...)
	} else {
		p.matches = p.matches[:0]
		for _, c := range p.commands {
			if fuzzyMatch(p.input, c.Name) || fuzzyMatch(p.input, c.Description) {
				p.matches = append(p.matches, c)
			}
		}
	}
	if p.cursor >= len(p.matches) {
		p.cursor = 0
	}
}

func fuzzyMatch(query, candidate string) bool {
	q := strings.ToLower(query)
	c := strings.ToLower(candidate)
	qi := 0
	for ci := 0; ci < len(c) && qi < len(q); ci++ {
		if c[ci] == q[qi] {
			qi++
		}
	}
	return qi == len(q)
}

func (p Palette) View() string {
	if !p.active {
		return ""
	}
	header := theme.Title.Render("> " + p.input)
	rows := make([]string, 0, len(p.matches))
	for i, m := range p.matches {
		row := m.Name
		if m.Description != "" {
			row += "  " + m.Description
		}
		if i == p.cursor {
			row = theme.TableSel.Render(row)
		}
		rows = append(rows, row)
	}
	body := strings.Join(rows, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}
