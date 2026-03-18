package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// APIResult holds the outcome of a single API test.
type APIResult struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
	Body     string
}

// APIResultsPanel shows a ring buffer of recent API test results.
type APIResultsPanel struct {
	results    []APIResult
	maxResults int
	width      int
}

// NewAPIResultsPanel creates an APIResultsPanel with a capacity of 10.
func NewAPIResultsPanel() APIResultsPanel {
	return APIResultsPanel{
		results:    make([]APIResult, 0, 10),
		maxResults: 10,
		width:      80,
	}
}

// SetWidth returns a copy of the panel with the given width.
func (p APIResultsPanel) SetWidth(w int) APIResultsPanel {
	p.width = w
	return p
}

// Add appends a result, evicting the oldest if at capacity.
func (p *APIResultsPanel) Add(r APIResult) {
	if len(p.results) >= p.maxResults {
		p.results = p.results[1:]
	}
	p.results = append(p.results, r)
}

// View renders the API results panel.
func (p APIResultsPanel) View() string {
	var b strings.Builder

	headerStyle := TitleStyle.Copy().MarginBottom(0)
	b.WriteString(headerStyle.Render("API RESULTS"))
	b.WriteString("\n\n")

	if len(p.results) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true).
			Render("No API calls yet. Select 'Test APIs' to begin."))
		return PanelStyle.Width(p.width).Render(b.String())
	}

	// Calculate available width for body preview
	// FORMAT: METHOD PATH STATUS DURATION BODY
	// Approximate: method(7) + path(~20) + status(5) + duration(10) + gaps(8) = 50
	innerWidth := p.width - 6 // borders + padding
	bodyWidth := innerWidth - 50
	if bodyWidth < 10 {
		bodyWidth = 10
	}

	for i, r := range p.results {
		method := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			Width(7).
			Render(r.Method)

		path := lipgloss.NewStyle().
			Width(20).
			Render(truncate(r.Path, 20))

		statusStyle := statusColorStyle(r.Status)
		status := statusStyle.Render(fmt.Sprintf("%d", r.Status))

		dur := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(10).
			Render(formatDuration(r.Duration))

		bodyPreview := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render(truncate(cleanBody(r.Body), bodyWidth))

		b.WriteString(fmt.Sprintf("%s %s %s %s %s", method, path, status, dur, bodyPreview))
		if i < len(p.results)-1 {
			b.WriteString("\n")
		}
	}

	return PanelStyle.Width(p.width).Render(b.String())
}

// statusColorStyle returns a style colored by HTTP status code range.
func statusColorStyle(code int) lipgloss.Style {
	switch {
	case code >= 200 && code < 300:
		return lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	case code >= 400 && code < 500:
		return lipgloss.NewStyle().Bold(true).Foreground(ColorWarning)
	case code >= 500:
		return lipgloss.NewStyle().Bold(true).Foreground(ColorDanger)
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(ColorMuted)
	}
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if maxLen <= 3 {
		if len(s) > maxLen {
			return s[:maxLen]
		}
		return s
	}
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// cleanBody removes newlines and excess whitespace from response body.
func cleanBody(body string) string {
	body = strings.ReplaceAll(body, "\n", " ")
	body = strings.ReplaceAll(body, "\r", "")
	body = strings.ReplaceAll(body, "\t", " ")
	// Collapse multiple spaces
	for strings.Contains(body, "  ") {
		body = strings.ReplaceAll(body, "  ", " ")
	}
	return strings.TrimSpace(body)
}

// formatDuration formats a duration in a human-friendly way.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dus", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
