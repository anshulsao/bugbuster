package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/facets-cloud/bugbuster/internal/hints"
	"github.com/facets-cloud/bugbuster/internal/scenario"
	"github.com/facets-cloud/bugbuster/internal/scoring"
)

// SidebarPanel shows timer, points, and hints status.
type SidebarPanel struct {
	session  *scoring.Session
	scenario *scenario.Scenario
	width    int
}

// NewSidebarPanel creates a SidebarPanel.
func NewSidebarPanel(sc *scenario.Scenario, sess *scoring.Session) SidebarPanel {
	return SidebarPanel{
		session:  sess,
		scenario: sc,
		width:    15,
	}
}

// SetWidth returns a copy of the panel with the given width.
func (p SidebarPanel) SetWidth(w int) SidebarPanel {
	p.width = w
	return p
}

// View renders the sidebar with timer, points, and hints.
func (p SidebarPanel) View() string {
	// Timer
	elapsed := time.Since(p.session.StartTime)
	mins := int(elapsed.Minutes())
	secs := int(elapsed.Seconds()) % 60
	timerStr := fmt.Sprintf("%02d:%02d", mins, secs)

	timerLabel := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render("TIMER")
	timerValue := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		Render(timerStr)

	// Hints
	used := len(p.session.HintsUsed)
	total := used + hints.RemainingHints(p.scenario.Hints, p.session.HintsUsed)
	hintsLabel := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render("HINTS")
	hintsValue := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning).
		Render(fmt.Sprintf("%d/%d", used, total))

	content := lipgloss.JoinVertical(lipgloss.Left,
		timerLabel,
		timerValue,
		"",
		hintsLabel,
		hintsValue,
	)

	return SidebarStyle.
		Width(p.width).
		Render(content)
}

// ViewCompact renders the sidebar as a single-line status bar for compact mode.
func (p SidebarPanel) ViewCompact(width int) string {
	elapsed := time.Since(p.session.StartTime)
	mins := int(elapsed.Minutes())
	secs := int(elapsed.Seconds()) % 60
	timerStr := fmt.Sprintf("%02d:%02d", mins, secs)

	used := len(p.session.HintsUsed)
	total := used + hints.RemainingHints(p.scenario.Hints, p.session.HintsUsed)

	bar := fmt.Sprintf("Timer: %s | Hints: %d/%d", timerStr, used, total)
	return KeyHelpStyle.Render(bar)
}
