package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/facets-cloud/bugbuster/internal/scenario"
)

type apiEndpoint struct {
	Method string
	Path   string
	Label  string
}

// APITesterModel is the model for the API testing screen.
type APITesterModel struct {
	endpoints  []apiEndpoint
	cursor     int
	customURL  string
	editingURL bool
	response   *APIResultMsg
	loading    bool
	width      int
	height     int
	scenario   *scenario.Scenario
}

// NewAPITesterModel creates a new API tester model with predefined endpoints.
func NewAPITesterModel(sc *scenario.Scenario, width, height int) APITesterModel {
	return APITesterModel{
		scenario: sc,
		width:    width,
		height:   height,
		endpoints: []apiEndpoint{
			{Method: "GET", Path: "/api/catalog/products", Label: "List products"},
			{Method: "POST", Path: "/api/orders", Label: "Create order (random product)"},
			{Method: "GET", Path: "/api/orders", Label: "List orders"},
			{Method: "GET", Path: "", Label: "Custom URL"},
		},
	}
}

// Init satisfies the tea.Model interface (no-op for sub-models).
func (m APITesterModel) Init() tea.Cmd {
	return nil
}

// Update handles key presses and API responses for the API tester screen.
func (m APITesterModel) Update(msg tea.Msg) (APITesterModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		// If we are editing the custom URL, capture keystrokes.
		if m.editingURL {
			switch msg.Type {
			case tea.KeyEscape:
				m.editingURL = false
			case tea.KeyEnter:
				if m.customURL != "" {
					m.editingURL = false
					m.loading = true
					method := "GET"
					path := m.customURL
					if !strings.HasPrefix(path, "/") {
						path = "/" + path
					}
					return m, FireAPI("http://localhost:8888", method, path, nil)
				}
			case tea.KeyBackspace:
				if len(m.customURL) > 0 {
					m.customURL = m.customURL[:len(m.customURL)-1]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.customURL += string(msg.Runes)
				} else if msg.Type == tea.KeySpace {
					m.customURL += " "
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.endpoints)-1 {
				m.cursor++
			}
		case "enter":
			ep := m.endpoints[m.cursor]
			// Custom URL entry
			if ep.Label == "Custom URL" {
				m.editingURL = true
				return m, nil
			}
			m.loading = true
			var body *bytes.Reader
			if ep.Method == "POST" {
				payload := fmt.Sprintf(`{"product_id":%d,"quantity":1}`, rand.Intn(100)+1)
				body = bytes.NewReader([]byte(payload))
				return m, FireAPI("http://localhost:8888", ep.Method, ep.Path, body)
			}
			return m, FireAPI("http://localhost:8888", ep.Method, ep.Path, nil)
		case "esc":
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenRunning} }
		}

	case APIResultMsg:
		m.loading = false
		m.response = &msg
	}
	return m, nil
}

// View renders the API tester screen.
func (m APITesterModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("API Tester"))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Fire HTTP requests against the running services"))
	b.WriteString("\n\n")

	// Endpoint list
	for i, ep := range m.endpoints {
		methodStyle := lipgloss.NewStyle().Bold(true).Width(6)
		switch ep.Method {
		case "GET":
			methodStyle = methodStyle.Foreground(ColorSuccess)
		case "POST":
			methodStyle = methodStyle.Foreground(ColorWarning)
		default:
			methodStyle = methodStyle.Foreground(ColorAccent)
		}

		var line string
		if ep.Label == "Custom URL" {
			urlDisplay := m.customURL
			if urlDisplay == "" {
				urlDisplay = "/your/path/here"
			}
			if m.editingURL && m.cursor == i {
				line = fmt.Sprintf("%s %s%s",
					methodStyle.Render("GET"),
					urlDisplay,
					lipgloss.NewStyle().Foreground(ColorAccent).Render("_"),
				)
			} else {
				line = fmt.Sprintf("%s %s",
					methodStyle.Render("GET"),
					lipgloss.NewStyle().Foreground(ColorMuted).Render(urlDisplay),
				)
			}
		} else {
			line = fmt.Sprintf("%s %s  %s",
				methodStyle.Render(ep.Method),
				ep.Path,
				lipgloss.NewStyle().Foreground(ColorMuted).Render("("+ep.Label+")"),
			)
		}

		if i == m.cursor {
			b.WriteString(SelectedActionStyle.Render("> " + line))
		} else {
			b.WriteString(ActionItemStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Response area
	if m.loading {
		b.WriteString(PanelStyle.Render(
			lipgloss.NewStyle().Foreground(ColorWarning).Render("Sending request..."),
		))
	} else if m.response != nil {
		b.WriteString(m.renderResponse())
	} else {
		b.WriteString(PanelStyle.Render(
			lipgloss.NewStyle().Foreground(ColorMuted).Render("Select an endpoint and press [enter] to fire a request"),
		))
	}

	b.WriteString("\n\n")

	// Key help
	if m.editingURL {
		b.WriteString(KeyHelpStyle.Render("[enter] Fire request  [esc] Cancel editing"))
	} else {
		b.WriteString(KeyHelpStyle.Render("[enter] Fire request  [up/down] Navigate  [esc] Back"))
	}

	return b.String()
}

func (m APITesterModel) renderResponse() string {
	resp := m.response
	var b strings.Builder

	if resp.Err != nil {
		b.WriteString(StatusDownStyle.Render("ERROR"))
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDanger).Render(resp.Err.Error()))
		return PanelStyle.Render(b.String())
	}

	// Status line
	statusStyle := StatusUpStyle
	if resp.Status >= 400 {
		statusStyle = StatusDownStyle
	}
	b.WriteString(fmt.Sprintf("%s %s %s  %s\n\n",
		lipgloss.NewStyle().Bold(true).Render(resp.Method),
		resp.Path,
		statusStyle.Render(fmt.Sprintf("%d", resp.Status)),
		lipgloss.NewStyle().Foreground(ColorMuted).Render(resp.Duration.String()),
	))

	// Pretty-print JSON body if possible
	body := resp.Body
	if body != "" {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(body), "", "  "); err == nil {
			body = pretty.String()
		}
		// Truncate long bodies for display
		lines := strings.Split(body, "\n")
		maxLines := 30
		if m.height > 0 {
			maxLines = m.height/2 - 10
			if maxLines < 10 {
				maxLines = 10
			}
		}
		if len(lines) > maxLines {
			lines = append(lines[:maxLines], fmt.Sprintf("... (%d more lines)", len(lines)-maxLines))
		}
		body = strings.Join(lines, "\n")
		b.WriteString(body)
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render("(empty response body)"))
	}

	maxWidth := m.width - 8
	if maxWidth < 40 {
		maxWidth = 80
	}
	return PanelStyle.Width(maxWidth).Render(b.String())
}
