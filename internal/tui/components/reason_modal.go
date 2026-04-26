package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type ReasonModal struct {
	value     string
	submitted bool
	cancelled bool
	err       string
}

func NewReasonModal() ReasonModal { return ReasonModal{} }

func (m ReasonModal) Init() tea.Cmd { return nil }

func (m ReasonModal) Update(msg tea.Msg) (ReasonModal, tea.Cmd) {
	if m.submitted || m.cancelled {
		return m, nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.Type {
	case tea.KeyEsc:
		m.cancelled = true
		return m, nil
	case tea.KeyEnter:
		if strings.TrimSpace(m.value) == "" {
			m.err = "reason required"
			return m, nil
		}
		m.submitted = true
		return m, nil
	case tea.KeyBackspace:
		if len(m.value) > 0 {
			m.value = m.value[:len(m.value)-1]
			m.err = ""
		}
		return m, nil
	case tea.KeyRunes:
		m.value += string(k.Runes)
		m.err = ""
		return m, nil
	case tea.KeySpace:
		m.value += " "
		m.err = ""
		return m, nil
	}
	return m, nil
}

func (m ReasonModal) View() string {
	var b strings.Builder
	b.WriteString("┌─ Reject reason ──┐\n")
	b.WriteString("│ " + m.value + "_\n")
	if m.err != "" {
		b.WriteString("│ ! " + m.err + "\n")
	}
	b.WriteString("└──────────────────┘")
	return b.String()
}

func (m ReasonModal) Value() string   { return m.value }
func (m ReasonModal) Submitted() bool { return m.submitted }
func (m ReasonModal) Cancelled() bool { return m.cancelled }
func (m ReasonModal) Error() string   { return m.err }
