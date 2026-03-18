package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/facets-cloud/bugbuster/internal/hints"
	"github.com/facets-cloud/bugbuster/internal/scenario"
	"github.com/facets-cloud/bugbuster/internal/scoring"
)

type revealedHint struct {
	index int
	text  string
}

// HintsModel is the model for the hints screen.
type HintsModel struct {
	scenario      *scenario.Scenario
	session       *scoring.Session
	revealedHints []revealedHint
	width         int
	height        int
	confirming    bool
	projectRoot   string
}

// NewHintsModel creates a new hints model, pre-populating revealed hints from the session.
func NewHintsModel(projectRoot string, sc *scenario.Scenario, sess *scoring.Session, width, height int) HintsModel {
	var revealed []revealedHint
	for _, idx := range sess.HintsUsed {
		if idx >= 0 && idx < len(sc.Hints) {
			revealed = append(revealed, revealedHint{
				index: idx,
				text:  sc.Hints[idx].Text,
			})
		}
	}
	return HintsModel{
		scenario:      sc,
		session:       sess,
		revealedHints: revealed,
		projectRoot:   projectRoot,
		width:         width,
		height:        height,
	}
}

// Init satisfies the tea.Model interface.
func (m HintsModel) Init() tea.Cmd {
	return nil
}

// Update handles key presses and hint reveal responses.
func (m HintsModel) Update(msg tea.Msg) (HintsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.confirming {
				m.confirming = false
				return m, RevealHint(m.projectRoot, m.scenario, m.session)
			}
			remaining := hints.RemainingHints(m.scenario.Hints, m.session.HintsUsed)
			if remaining > 0 {
				m.confirming = true
			}
		case "esc":
			if m.confirming {
				m.confirming = false
				return m, nil
			}
			return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenRunning} }
		}

	case HintRevealedMsg:
		if msg.Err != nil {
			m.confirming = false
			return m, nil
		}
		m.revealedHints = append(m.revealedHints, revealedHint{
			index: msg.Index,
			text:  msg.Text,
		})
	}
	return m, nil
}

// View renders the hints screen.
func (m HintsModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Hints"))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render(
		fmt.Sprintf("Hints used: %d/%d",
			len(m.revealedHints),
			len(m.scenario.Hints),
		),
	))
	b.WriteString("\n\n")

	hintBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 2).
		MarginBottom(1)

	maxWidth := m.width - 8
	if maxWidth < 40 {
		maxWidth = 80
	}

	// Revealed hints
	if len(m.revealedHints) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render("  No hints revealed yet."))
		b.WriteString("\n\n")
	} else {
		for i, h := range m.revealedHints {
			header := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).
				Render(fmt.Sprintf("Hint #%d", i+1))

			content := fmt.Sprintf("%s\n%s", header, h.text)
			b.WriteString(hintBoxStyle.Width(maxWidth).Render(content))
			b.WriteString("\n")
		}
	}

	// Next hint or all used
	remaining := hints.RemainingHints(m.scenario.Hints, m.session.HintsUsed)
	if remaining > 0 {
		if m.confirming {
			confirmBox := AlertBoxStyle.Width(maxWidth).Render(
				"Reveal next hint?\n\n[enter] Yes    [esc] Cancel",
			)
			b.WriteString(confirmBox)
		} else {
			availStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			b.WriteString(availStyle.Render(
				fmt.Sprintf("  %d hints remaining - press [enter] to reveal", remaining),
			))
		}
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).
			Render("  All hints have been revealed."))
	}

	b.WriteString("\n\n")
	b.WriteString(KeyHelpStyle.Render("[enter] Reveal hint  [esc] Back"))

	return b.String()
}
