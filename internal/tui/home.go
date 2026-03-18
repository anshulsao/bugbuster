package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/facets-cloud/bugbuster/internal/scenario"
)

// HomeModel represents the home/landing screen of BugBuster.
type HomeModel struct {
	scenarios   []*scenario.Scenario
	cursor      int
	width       int
	height      int
	loading     bool
	projectRoot string
}

// NewHomeModel creates a new HomeModel in the loading state.
func NewHomeModel(projectRoot string) HomeModel {
	return HomeModel{
		cursor:      0,
		loading:     true,
		projectRoot: projectRoot,
	}
}

// Init satisfies the tea.Model interface. Returns nil because the root model
// is responsible for dispatching LoadScenarios().
func (m HomeModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the home screen.
func (m HomeModel) Update(msg tea.Msg) (HomeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ScenariosLoadedMsg:
		m.scenarios = msg.Scenarios
		m.loading = false
		if msg.Err != nil {
			// Keep loading false so the "no scenarios" message shows
			m.scenarios = nil
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if len(m.scenarios) > 0 && m.cursor < len(m.scenarios)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.scenarios) > 0 && m.cursor < len(m.scenarios) {
				return m, func() tea.Msg {
					return ScenarioSelectedMsg{Scenario: m.scenarios[m.cursor]}
				}
			}
		case "q":
			return m, tea.Quit
		}
	}

	return m, nil
}

// levelBadge returns a styled level string with appropriate color.
func levelBadge(level int) string {
	var label string
	var style lipgloss.Style

	switch level {
	case 1:
		label = "EASY"
		style = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)
	case 2:
		label = "MEDIUM"
		style = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)
	case 3:
		label = "HARD"
		style = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)
	default:
		label = "EXTREME"
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DC2626")).
			Bold(true).
			Blink(true)
	}

	return style.Render(fmt.Sprintf("[%s]", label))
}

// View renders the home screen.
func (m HomeModel) View() string {
	var b strings.Builder

	// Banner
	banner := TitleStyle.Render("BugBuster -- Incident Response Training")
	subtitle := SubtitleStyle.Render("Master production debugging through realistic scenarios")
	b.WriteString(banner + "\n" + subtitle + "\n\n")

	// Loading state
	if m.loading {
		b.WriteString(KeyHelpStyle.Render("Loading scenarios..."))
		return b.String()
	}

	// No scenarios
	if len(m.scenarios) == 0 {
		b.WriteString(AlertBoxStyle.Render("No scenarios found. Place scenario directories in ./scenarios/"))
		return b.String()
	}

	// Build left column: scenario list
	var scenarioLines []string
	scenarioLines = append(scenarioLines, lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Render("SCENARIOS"))
	scenarioLines = append(scenarioLines, "")

	for i, sc := range m.scenarios {
		timeStr := fmt.Sprintf("~%d min", sc.EstimatedTimeMinutes)
		badge := levelBadge(sc.Level)
		name := sc.Name

		if i == m.cursor {
			line := SelectedActionStyle.Render(fmt.Sprintf("> %s  %s  %s", name, badge, KeyHelpStyle.Render(timeStr)))
			scenarioLines = append(scenarioLines, line)
		} else {
			line := ActionItemStyle.Render(fmt.Sprintf("  %s  %s  %s", name, badge, KeyHelpStyle.Render(timeStr)))
			scenarioLines = append(scenarioLines, line)
		}
	}

	b.WriteString(PanelStyle.Render(strings.Join(scenarioLines, "\n")))
	b.WriteString("\n\n")

	// Workspace path
	if m.projectRoot != "" {
		pathInfo := KeyHelpStyle.Render(fmt.Sprintf("Workspace: %s", m.projectRoot))
		b.WriteString(pathInfo)
		b.WriteString("\n")
		pathHint := KeyHelpStyle.Render("Service source code is here — edit files to debug and fix bugs")
		b.WriteString(pathHint)
		b.WriteString("\n\n")
	}

	// Key help
	help := KeyHelpStyle.Render("[enter] Start  [q] Quit")
	b.WriteString(help)

	return b.String()
}
