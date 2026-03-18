package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/facets-cloud/bugbuster/internal/scenario"
)

// AlertPanel renders the incident alert and observation in a bordered box.
type AlertPanel struct {
	alert       string
	observation string
	width       int
}

// NewAlertPanel creates an AlertPanel from a scenario.
func NewAlertPanel(sc *scenario.Scenario) AlertPanel {
	return AlertPanel{
		alert:       sc.Incident.Alert,
		observation: sc.Incident.Observation,
		width:       60,
	}
}

// SetWidth returns a copy of the panel with the given width.
func (p AlertPanel) SetWidth(w int) AlertPanel {
	p.width = w
	return p
}

// View renders the alert panel.
func (p AlertPanel) View() string {
	innerWidth := p.width - 6 // account for border + padding
	if innerWidth < 20 {
		innerWidth = 20
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorDanger).
		Render("INCIDENT")

	alertStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning).
		Width(innerWidth)

	observationStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(innerWidth)

	alertText := alertStyle.Render(wordWrap(p.alert, innerWidth))
	obsText := observationStyle.Render(wordWrap(p.observation, innerWidth))

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		alertText,
		"",
		obsText,
	)

	box := AlertBoxStyle.
		Width(p.width).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorDanger)

	return box.Render(content)
}

// wordWrap breaks text into lines that fit within maxWidth characters.
func wordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) > maxWidth {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}
	lines = append(lines, currentLine)

	return strings.Join(lines, "\n")
}
