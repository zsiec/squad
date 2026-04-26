package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/zsiec/squad/internal/tui/theme"
)

// ConnState is the connection-state indicator shown in the status bar.
type ConnState int

const (
	ConnUp ConnState = iota
	ConnReconnecting
	ConnDown
)

// StatusBarState carries the data the bar needs each render. View modules
// (specifically the root model) populate it before invoking StatusBar.
type StatusBarState struct {
	Scope  string
	View   string
	Conn   ConnState
	Notify int
	Lag    int // milliseconds; 0 means "no lag"
	Hints  []string
	Width  int
}

// StatusBar renders the status bar as a styled string sized to s.Width.
// Pure function — no internal state.
func StatusBar(s StatusBarState) string {
	dotStyle := theme.ConnUp
	switch s.Conn {
	case ConnReconnecting:
		dotStyle = theme.ConnReconnecting
	case ConnDown:
		dotStyle = theme.ConnDown
	}
	dot := dotStyle.Render("●")

	left := fmt.Sprintf("%s scope:%s view:%s", dot, s.Scope, s.View)

	right := ""
	if s.Notify > 0 {
		right += fmt.Sprintf("📬 %d ", s.Notify)
	}
	if s.Lag > 0 {
		right += theme.ConnReconnecting.Render(fmt.Sprintf("⚠ lag %dms ", s.Lag))
	}
	for _, h := range s.Hints {
		right += " " + h
	}

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	mid := s.Width - leftWidth - rightWidth
	if mid < 1 {
		mid = 1
	}
	spacer := lipgloss.NewStyle().Width(mid).Render("")

	return theme.StatusBar.Render(left + spacer + right)
}
