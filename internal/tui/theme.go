package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
const (
	ColorPrimary = lipgloss.Color("#7C3AED")
	ColorAccent  = lipgloss.Color("#06B6D4")
	ColorSuccess = lipgloss.Color("#10B981")
	ColorWarning = lipgloss.Color("#F59E0B")
	ColorDanger  = lipgloss.Color("#EF4444")
	ColorMuted   = lipgloss.Color("#6B7280")
	ColorSurface = lipgloss.Color("#1F2937")
	ColorBg      = lipgloss.Color("#111827")
)

// Styles used across all TUI screens.
var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	AlertBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDanger).
			Padding(1, 2).
			Foreground(ColorDanger).
			Bold(true)

	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(1, 2)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorAccent).
			Padding(1, 2)

	ActionItemStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingLeft(2)

	SelectedActionStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true).
				PaddingLeft(1)

	StatusUpStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StatusDownStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	KeyHelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary)
)
