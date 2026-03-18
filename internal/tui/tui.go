package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/facets-cloud/bugbuster/internal/docker"
	"github.com/facets-cloud/bugbuster/internal/scenario"
	"github.com/facets-cloud/bugbuster/internal/scoring"
)

// Model is the top-level Bubble Tea model that manages screen routing.
type Model struct {
	screen      ScreenType
	width       int
	height      int
	projectRoot string

	// State
	scenario     *scenario.Scenario
	session      *scoring.Session
	composeFiles []string

	// Sub-models (screens)
	home        HomeModel
	startup     StartupModel
	running     RunningModel
	apiTester   APITesterModel
	hints       HintsModel
	submit      SubmitModel

	// Prerequisite check
	prereqErrors []string
}

// NewModel creates a Model starting at the home screen.
func NewModel(projectRoot string) Model {
	return Model{
		screen:      ScreenHome,
		projectRoot: projectRoot,
		home:        NewHomeModel(projectRoot),
	}
}

// Init loads scenarios and leaderboard on startup.
func (m Model) Init() tea.Cmd {
	return LoadScenarios(m.projectRoot)
}

// Update handles global messages and delegates to the active screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Sub-models that handle WindowSizeMsg will get it via delegation below.

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.scenario != nil && len(m.composeFiles) > 0 {
				_ = docker.Down(m.projectRoot, m.composeFiles)
			}
			return m, tea.Quit
		}
		// Dismiss prereq error overlay
		if len(m.prereqErrors) > 0 && (msg.String() == "esc" || msg.String() == "q") {
			m.prereqErrors = nil
			return m, nil
		}

	case ScreenChangeMsg:
		m.screen = msg.Screen
		switch msg.Screen {
		case ScreenHome:
			m.home = NewHomeModel(m.projectRoot)
			cmds = append(cmds, LoadScenarios(m.projectRoot))
		case ScreenStartup:
			if m.scenario != nil {
				m.composeFiles = docker.ComposeFiles(m.projectRoot, m.scenario.Dir)
				m.startup = NewStartupModel(m.scenario.Name)
				cmds = append(cmds, StartDocker(m.projectRoot, m.composeFiles))
			}
		case ScreenRunning:
			if m.scenario != nil && m.session != nil {
				m.running = NewRunningModel(m.projectRoot, m.scenario, m.session, m.composeFiles)
				cmds = append(cmds, m.running.Init())
			}
		case ScreenAPITester:
			if m.scenario != nil {
				m.apiTester = NewAPITesterModel(m.scenario, m.width, m.height)
			}
		case ScreenHints:
			if m.scenario != nil && m.session != nil {
				m.hints = NewHintsModel(m.projectRoot, m.scenario, m.session, m.width, m.height)
			}
		case ScreenSubmit:
			if m.scenario != nil && m.session != nil {
				m.submit = NewSubmitModel(m.projectRoot, m.scenario, m.session, m.width, m.height)
			}
		}
		return m, tea.Batch(cmds...)

	case ScenarioSelectedMsg:
		m.scenario = msg.Scenario
		m.prereqErrors = nil

		// Check prerequisites before starting Docker
		if issues := docker.CheckPrerequisites(); len(issues) > 0 {
			m.prereqErrors = issues
			return m, nil
		}

		// Create a new scoring session.
		if err := scoring.NewSession(m.projectRoot, msg.Scenario.Dir); err != nil {
			m.session = &scoring.Session{
				Scenario: msg.Scenario.Dir,
				Points:   scoring.StartingPoints,
				Active:   true,
			}
		} else {
			sess, _ := scoring.LoadSession(m.projectRoot)
			m.session = sess
		}
		// Transition to startup screen.
		m.screen = ScreenStartup
		m.composeFiles = docker.ComposeFiles(m.projectRoot, m.scenario.Dir)
		m.startup = NewStartupModel(m.scenario.Name)
		cmds = append(cmds, StartDocker(m.projectRoot, m.composeFiles))
		return m, tea.Batch(cmds...)
	}

	// Delegate to active screen.
	var cmd tea.Cmd
	switch m.screen {
	case ScreenHome:
		m.home, cmd = m.home.Update(msg)
	case ScreenStartup:
		m.startup, cmd = m.startup.Update(msg)
	case ScreenRunning:
		m.running, cmd = m.running.Update(msg)
	case ScreenAPITester:
		m.apiTester, cmd = m.apiTester.Update(msg)
	case ScreenHints:
		m.hints, cmd = m.hints.Update(msg)
	case ScreenSubmit:
		m.submit, cmd = m.submit.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the active screen.
func (m Model) View() string {
	if len(m.prereqErrors) > 0 {
		return m.viewPrereqErrors()
	}

	switch m.screen {
	case ScreenHome:
		return m.home.View()
	case ScreenStartup:
		return m.startup.View()
	case ScreenRunning:
		return m.running.View()
	case ScreenAPITester:
		return m.apiTester.View()
	case ScreenHints:
		return m.hints.View()
	case ScreenSubmit:
		return m.submit.View()
	default:
		return "Unknown screen"
	}
}

func (m Model) viewPrereqErrors() string {
	var b strings.Builder

	header := lipgloss.NewStyle().Bold(true).Foreground(ColorDanger).
		Render("PREREQUISITES CHECK FAILED")
	b.WriteString(header)
	b.WriteString("\n\n")

	for _, issue := range m.prereqErrors {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			StatusDownStyle.Render("[X]"),
			issue))
	}

	b.WriteString("\n")
	b.WriteString(KeyHelpStyle.Render("Install the prerequisites above and try again."))
	b.WriteString("\n\n")
	b.WriteString(KeyHelpStyle.Render("[esc] Back  [q] Quit"))

	return b.String()
}
