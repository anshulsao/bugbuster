package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/facets-cloud/bugbuster/internal/scenario"
	"github.com/facets-cloud/bugbuster/internal/scoring"
)

// RunningModel is the main dashboard compositor for an active debugging session.
type RunningModel struct {
	alert      AlertPanel
	sidebar    SidebarPanel
	actions    ActionsPanel
	services   ServicesPanel
	apiResults APIResultsPanel

	scenario     *scenario.Scenario
	session      *scoring.Session
	projectRoot  string
	composeFiles []string
	width        int
	height       int
}

// NewRunningModel creates a RunningModel with all sub-panels initialized.
func NewRunningModel(projectRoot string, sc *scenario.Scenario, sess *scoring.Session, composeFiles []string) RunningModel {
	return RunningModel{
		alert:        NewAlertPanel(sc),
		sidebar:      NewSidebarPanel(sc, sess),
		actions:      NewActionsPanel(),
		services:     NewServicesPanel(),
		apiResults:   NewAPIResultsPanel(),
		scenario:     sc,
		session:      sess,
		projectRoot:  projectRoot,
		composeFiles: composeFiles,
		width:        120,
		height:       40,
	}
}

// Init starts the timer tick and service polling.
func (m RunningModel) Init() tea.Cmd {
	return tea.Batch(TimerTick(), ServicePollTick())
}

// Update handles all messages for the running dashboard.
func (m RunningModel) Update(msg tea.Msg) (RunningModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TimerTickMsg:
		return m, TimerTick()

	case ServiceStatusMsg:
		if msg.Output == "" && msg.Err == nil {
			return m, PollServices(m.projectRoot, m.composeFiles)
		}
		m.services.Update(msg)
		return m, ServicePollTick()

	case APIResultMsg:
		result := APIResult{
			Method:   msg.Method,
			Path:     msg.Path,
			Status:   msg.Status,
			Duration: msg.Duration,
			Body:     msg.Body,
		}
		m.apiResults.Add(result)
		return m, nil

	case actionSelectedMsg:
		return m.handleAction(msg.action)

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "down", "j", "k", "enter", "1", "2", "3", "4", "5", "6", "7":
			cmd := m.actions.Update(msg)
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		default:
			return m, nil
		}
	}

	return m, nil
}

// handleAction processes the selected action and returns appropriate commands.
func (m RunningModel) handleAction(action string) (RunningModel, tea.Cmd) {
	switch {
	case strings.HasPrefix(action, "Test APIs"):
		return m, func() tea.Msg {
			return ScreenChangeMsg{Screen: ScreenAPITester}
		}
	case strings.HasPrefix(action, "View Hints"):
		return m, func() tea.Msg {
			return ScreenChangeMsg{Screen: ScreenHints}
		}
	case strings.HasPrefix(action, "Open Grafana"):
		return m, OpenBrowser("http://localhost:3000")
	case strings.HasPrefix(action, "Open Jaeger"):
		return m, OpenBrowser("http://localhost:16686")
	case strings.HasPrefix(action, "Open Prometheus"):
		return m, OpenBrowser("http://localhost:9091")
	case strings.HasPrefix(action, "Submit RCA"):
		return m, func() tea.Msg {
			return ScreenChangeMsg{Screen: ScreenSubmit}
		}
	case strings.HasPrefix(action, "Stop & Quit"):
		return m, tea.Quit
	}
	return m, nil
}

// View renders the complete dashboard layout, clamped to terminal height.
func (m RunningModel) View() string {
	var content string
	if m.width < 100 {
		content = m.viewCompact()
	} else {
		content = m.viewWide()
	}
	return clampHeight(content, m.height)
}

// clampHeight truncates rendered content to fit within maxHeight lines.
func clampHeight(content string, maxHeight int) string {
	if maxHeight <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxHeight {
		return content
	}
	return strings.Join(lines[:maxHeight], "\n")
}

// viewWide renders the wide (>=100 columns) layout.
func (m RunningModel) viewWide() string {
	sidebarWidth := 20
	actionsWidth := 30
	availableWidth := m.width

	// Row 1: Alert (compact — just 3 lines) + Sidebar
	alertWidth := availableWidth - sidebarWidth - 2
	if alertWidth < 30 {
		alertWidth = 30
	}
	alertView := m.alert.SetWidth(alertWidth).View()
	sidebarView := m.sidebar.SetWidth(sidebarWidth).View()

	// Match heights for row 1
	alertHeight := lipgloss.Height(alertView)
	sidebarHeight := lipgloss.Height(sidebarView)
	if alertHeight > sidebarHeight {
		sidebarView = lipgloss.NewStyle().Height(alertHeight).Render(sidebarView)
	} else if sidebarHeight > alertHeight {
		alertView = lipgloss.NewStyle().Height(sidebarHeight).Render(alertView)
	}

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, alertView, " ", sidebarView)

	// Row 2: Actions + Services
	servicesWidth := availableWidth - actionsWidth - 2
	if servicesWidth < 20 {
		servicesWidth = 20
	}
	actionsView := m.actions.SetWidth(actionsWidth).View()
	servicesView := m.services.SetWidth(servicesWidth).View()

	// Match heights for row 2
	actionsHeight := lipgloss.Height(actionsView)
	servicesHeight := lipgloss.Height(servicesView)
	if actionsHeight > servicesHeight {
		servicesView = lipgloss.NewStyle().Height(actionsHeight).Render(servicesView)
	} else if servicesHeight > actionsHeight {
		actionsView = lipgloss.NewStyle().Height(servicesHeight).Render(actionsView)
	}

	row2 := lipgloss.JoinHorizontal(lipgloss.Top, actionsView, " ", servicesView)

	// Calculate remaining height for API results
	row1Height := lipgloss.Height(row1)
	row2Height := lipgloss.Height(row2)
	usedHeight := row1Height + 1 + row2Height + 1 + 1 + 1 // rows + gaps + help + bottom
	remainingHeight := m.height - usedHeight
	if remainingHeight < 3 {
		remainingHeight = 3
	}

	// Row 3: API Results (height-capped)
	row3 := m.apiResults.SetWidth(availableWidth).View()
	row3Lines := strings.Split(row3, "\n")
	if len(row3Lines) > remainingHeight {
		row3 = strings.Join(row3Lines[:remainingHeight], "\n")
	}

	// Workspace path + key help
	wsPath := KeyHelpStyle.Render(fmt.Sprintf("Source: %s/services/", m.projectRoot))
	help := KeyHelpStyle.Render("1-7: action  j/k: navigate  enter: select  ctrl+c: quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		row1,
		row2,
		row3,
		wsPath,
		help,
	)
}

// viewCompact renders a stacked vertical layout for narrow terminals.
// Collapses alert to a 2-line summary and sidebar to a single status bar
// to free vertical space for actions + services at 80x24.
func (m RunningModel) viewCompact() string {
	w := m.width
	if w < 20 {
		w = 20
	}

	// Collapsed alert: just the incident title in 2 lines
	alertLine := lipgloss.NewStyle().Bold(true).Foreground(ColorDanger).
		Render("INCIDENT")
	alertTitle := lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).
		Render(wordWrap(m.alert.alert, w-4))
	compactAlert := alertLine + "  " + alertTitle

	// Inline sidebar as single status bar
	sidebarBar := m.sidebar.ViewCompact(w)

	actionsView := m.actions.SetWidth(w).View()
	servicesView := m.services.SetWidth(w).View()

	wsPath := KeyHelpStyle.Render(fmt.Sprintf("Source: %s/services/", m.projectRoot))
	help := KeyHelpStyle.Render("1-7: action  j/k: navigate  enter: select  ctrl+c: quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		compactAlert,
		sidebarBar,
		actionsView,
		servicesView,
		wsPath,
		help,
	)
}
