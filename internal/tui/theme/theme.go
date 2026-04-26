// Package theme is the single visual theme for the squad TUI v1.
// All view modules import their styles from here so a future skin/theme
// system can swap palettes without touching view code.
package theme

import "github.com/charmbracelet/lipgloss"

// Color tokens. Hex codes chosen to read well on both dark and light
// terminals; lipgloss adapts to truecolor / 256-color / 16-color terminals
// automatically.
var (
	Primary = lipgloss.Color("#5FAFFF")
	Accent  = lipgloss.Color("#FFAF5F")
	Success = lipgloss.Color("#5FD787")
	Warn    = lipgloss.Color("#FFD75F")
	Danger  = lipgloss.Color("#FF5F87")
	Dim     = lipgloss.Color("#5F5F5F")
)

// Semantic styles. View modules compose these rather than redeclaring
// foreground/background/padding inline.
var (
	Title       = lipgloss.NewStyle().Bold(true).Foreground(Primary)
	StatusBar   = lipgloss.NewStyle().Foreground(Dim).Padding(0, 1)
	TableHeader = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	TableRow    = lipgloss.NewStyle()
	TableSel    = lipgloss.NewStyle().Background(Primary).Foreground(lipgloss.Color("#000000"))
	Toast       = lipgloss.NewStyle().Foreground(Warn).Background(lipgloss.Color("#1F1F1F")).Padding(0, 2)

	// Status indicators
	ConnUp           = lipgloss.NewStyle().Foreground(Success)
	ConnReconnecting = lipgloss.NewStyle().Foreground(Warn)
	ConnDown         = lipgloss.NewStyle().Foreground(Danger)

	// Chat kind accents (for the session pane and chat tail views)
	KindSay       = lipgloss.NewStyle()
	KindQuestion  = lipgloss.NewStyle().Foreground(Warn)
	KindAnswer    = lipgloss.NewStyle().Foreground(Success)
	KindKnock     = lipgloss.NewStyle().Foreground(Danger)
	KindHandoff   = lipgloss.NewStyle().Foreground(lipgloss.Color("#AF87FF"))
	KindProgress  = lipgloss.NewStyle().Foreground(Primary)
	KindMilestone = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF87FF"))
	KindFYI       = lipgloss.NewStyle().Foreground(Dim)
)
