package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StartupModel manages the Docker environment startup screen.
type StartupModel struct {
	scenarioName string
	logs         []string
	maxLogs      int
	totalLines   int
	done         bool
	err          error
	width        int
	height       int
	progress     float64
	lines        <-chan string
	errs         <-chan error
}

// NewStartupModel creates a StartupModel for the given scenario.
func NewStartupModel(scenarioName string) StartupModel {
	return StartupModel{
		scenarioName: scenarioName,
		maxLogs:      20,
	}
}

// Update handles messages for the startup screen.
func (m StartupModel) Update(msg tea.Msg) (StartupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case dockerStreamMsg:
		// Docker started — store channels and begin reading lines
		m.lines = msg.lines
		m.errs = msg.errs
		return m, WaitForDockerLine(m.lines, m.errs)

	case DockerLogMsg:
		m.totalLines++
		m.logs = append(m.logs, msg.Line)
		if len(m.logs) > m.maxLogs {
			m.logs = m.logs[len(m.logs)-m.maxLogs:]
		}
		// Heuristic progress: assume ~30 log lines = 100%
		m.progress = float64(m.totalLines) / 30.0
		if m.progress > 1.0 {
			m.progress = 0.99
		}
		// Read next line
		return m, WaitForDockerLine(m.lines, m.errs)

	case DockerDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.done = true
		m.progress = 1.0
		// After 1 second, transition to the running screen
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return ScreenChangeMsg{Screen: ScreenRunning}
		})

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the Docker startup screen.
func (m StartupModel) View() string {
	var b strings.Builder

	// Title
	title := TitleStyle.Render(fmt.Sprintf("Starting: %s", m.scenarioName))
	b.WriteString(title)
	b.WriteString("\n\n")

	// Progress bar
	barWidth := 50
	if m.width > 0 && m.width-10 < barWidth {
		barWidth = m.width - 10
		if barWidth < 10 {
			barWidth = 10
		}
	}
	filled := int(m.progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	var bar string
	if filled > 0 {
		bar = strings.Repeat("=", filled-1) + ">"
	}
	bar += strings.Repeat(" ", empty)
	pct := int(m.progress * 100)

	progressStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	progressBar := progressStyle.Render(fmt.Sprintf("[%s] %d%%", bar, pct))
	b.WriteString(progressBar)
	b.WriteString("\n\n")

	// Error state
	if m.err != nil {
		errMsg := AlertBoxStyle.Render(fmt.Sprintf("Error: %s", m.err.Error()))
		b.WriteString(errMsg)
		b.WriteString("\n\n")
		b.WriteString(KeyHelpStyle.Render("[ctrl+c] Quit"))
		return b.String()
	}

	// Done state
	if m.done {
		doneStyle := StatusUpStyle
		b.WriteString(doneStyle.Render("Environment ready! Launching dashboard..."))
		b.WriteString("\n")
		return b.String()
	}

	// Docker logs viewport
	logHeader := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Bold(true).
		Render("Docker Compose Output")
	b.WriteString(logHeader)
	b.WriteString("\n")

	logStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted).
		Padding(0, 1).
		Foreground(ColorMuted)

	var logContent string
	if len(m.logs) == 0 {
		logContent = "Waiting for output..."
	} else {
		logContent = strings.Join(m.logs, "\n")
	}

	b.WriteString(logStyle.Render(logContent))
	b.WriteString("\n\n")
	b.WriteString(KeyHelpStyle.Render("[ctrl+c] Cancel"))

	return b.String()
}
