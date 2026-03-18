package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// actionGroup represents a labeled group of actions.
type actionGroup struct {
	label string
	items []string
}

var actionGroups = []actionGroup{
	{label: "Investigate", items: []string{"Test APIs", "View Hints"}},
	{label: "Observe", items: []string{"Open Grafana", "Open Jaeger", "Open Prometheus"}},
	{label: "Finish", items: []string{"Submit RCA", "Stop & Quit"}},
}

// flatActions returns all action items in order, for indexing.
func flatActions() []string {
	var all []string
	for _, g := range actionGroups {
		all = append(all, g.items...)
	}
	return all
}

// ActionsPanel renders a navigable list of actions.
type ActionsPanel struct {
	items  []string
	cursor int
	width  int
}

// NewActionsPanel creates an ActionsPanel with the default action list.
func NewActionsPanel() ActionsPanel {
	return ActionsPanel{
		items:  flatActions(),
		cursor: 0,
		width:  28,
	}
}

// SetWidth returns a copy of the panel with the given width.
func (p ActionsPanel) SetWidth(w int) ActionsPanel {
	p.width = w
	return p
}

// Update handles keyboard input for cursor navigation and selection.
func (p *ActionsPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.items)-1 {
				p.cursor++
			}
		case "enter":
			return func() tea.Msg {
				return actionSelectedMsg{action: p.items[p.cursor]}
			}
		case "1", "2", "3", "4", "5", "6", "7":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(p.items) {
				p.cursor = idx
				return func() tea.Msg {
					return actionSelectedMsg{action: p.items[idx]}
				}
			}
		}
	}
	return nil
}

// actionSelectedMsg is an internal message for action selection.
type actionSelectedMsg struct {
	action string
}

// View renders the actions list with groups.
func (p ActionsPanel) View() string {
	var b strings.Builder

	headerStyle := TitleStyle.Copy().MarginBottom(0)
	b.WriteString(headerStyle.Render("ACTIONS"))
	b.WriteString("\n")

	groupLabelStyle := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)

	num := 1
	for gi, group := range actionGroups {
		b.WriteString(groupLabelStyle.Render("  " + group.label))
		b.WriteString("\n")

		for _, item := range group.items {
			globalIdx := num - 1
			line := fmt.Sprintf("%d  %s", num, item)
			if globalIdx == p.cursor {
				b.WriteString(SelectedActionStyle.Render("> " + line))
			} else {
				b.WriteString(ActionItemStyle.Render("  " + line))
			}
			b.WriteString("\n")
			num++
		}

		if gi < len(actionGroups)-1 {
			b.WriteString("\n")
		}
	}

	return PanelStyle.
		Width(p.width).
		Render(b.String())
}

// SelectedAction returns the currently highlighted action name.
func (p ActionsPanel) SelectedAction() string {
	if p.cursor >= 0 && p.cursor < len(p.items) {
		return p.items[p.cursor]
	}
	return ""
}
