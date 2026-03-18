package tui

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ServicesPanel displays docker compose service statuses.
type ServicesPanel struct {
	output string
	err    error
	width  int
}

// NewServicesPanel creates an empty ServicesPanel.
func NewServicesPanel() ServicesPanel {
	return ServicesPanel{
		width: 40,
	}
}

// SetWidth returns a copy of the panel with the given width.
func (p ServicesPanel) SetWidth(w int) ServicesPanel {
	p.width = w
	return p
}

// Update stores the latest service status output.
func (p *ServicesPanel) Update(msg ServiceStatusMsg) {
	p.output = msg.Output
	p.err = msg.Err
}

// View renders the services panel.
func (p ServicesPanel) View() string {
	var b strings.Builder

	headerStyle := TitleStyle.Copy().MarginBottom(0)
	b.WriteString(headerStyle.Render("SERVICES"))
	b.WriteString("\n\n")

	if p.err != nil {
		b.WriteString(StatusDownStyle.Render("Error: " + p.err.Error()))
		return PanelStyle.Width(p.width).Render(b.String())
	}

	if p.output == "" {
		b.WriteString(lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true).
			Render("Polling..."))
		return PanelStyle.Width(p.width).Render(b.String())
	}

	services := parseDockerComposePS(p.output)
	if len(services) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No services found"))
		return PanelStyle.Width(p.width).Render(b.String())
	}

	// Calculate name column width
	maxName := 0
	for _, svc := range services {
		if len(svc.name) > maxName {
			maxName = len(svc.name)
		}
	}
	if maxName > 22 {
		maxName = 22
	}

	for i, svc := range services {
		var indicator string
		if svc.up {
			if svc.health != "" {
				indicator = StatusUpStyle.Render("UP (" + svc.health + ")")
			} else {
				indicator = StatusUpStyle.Render("UP")
			}
		} else {
			indicator = StatusDownStyle.Render("DOWN")
		}
		nameStyle := lipgloss.NewStyle().Width(maxName + 2)
		b.WriteString(nameStyle.Render(svc.name) + indicator)
		if i < len(services)-1 {
			b.WriteString("\n")
		}
	}

	return PanelStyle.Width(p.width).Render(b.String())
}

type serviceEntry struct {
	name   string
	up     bool
	health string
}

// dockerPSEntry matches the JSON output of `docker compose ps --format json`.
type dockerPSEntry struct {
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
}

// parseDockerComposePS parses the JSON output of `docker compose ps --format json`.
func parseDockerComposePS(output string) []serviceEntry {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}

	// docker compose ps --format json returns a JSON array
	var raw []dockerPSEntry
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		// Fallback: try as newline-delimited JSON objects
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var entry dockerPSEntry
			if json.Unmarshal([]byte(line), &entry) == nil && entry.Service != "" {
				raw = append(raw, entry)
			}
		}
	}

	if len(raw) == 0 {
		return nil
	}

	// Infrastructure services to hide — freshers don't need to see these
	infra := map[string]bool{
		"prometheus":     true,
		"grafana":        true,
		"loki":           true,
		"promtail":       true,
		"otel-collector": true,
		"jaeger":         true,
		"tempo":          true,
		"mailhog":        true,
		"mock-api":       true,
		"scenario-traffic": true,
	}

	// Deduplicate by service name, filter out infra
	seen := map[string]bool{}
	var entries []serviceEntry
	for _, r := range raw {
		if r.Service == "" || seen[r.Service] || infra[r.Service] {
			continue
		}
		seen[r.Service] = true
		state := strings.ToLower(r.State)
		isUp := state == "running" || state == "up"
		entries = append(entries, serviceEntry{
			name:   r.Service,
			up:     isUp,
			health: r.Health,
		})
	}

	return entries
}
