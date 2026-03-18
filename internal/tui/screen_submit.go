package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/facets-cloud/bugbuster/internal/scenario"
	"github.com/facets-cloud/bugbuster/internal/scoring"
)

var rcaCategories = []string{
	"resource-saturation",
	"misconfiguration",
	"network-partition",
	"dependency-failure",
	"memory-leak",
	"race-condition",
	"data-corruption",
	"security-breach",
	"capacity-limit",
	"other",
}

var serviceOptions = []string{
	"api-gateway",
	"order-service",
	"payment-service",
	"catalog-service",
	"redis",
	"rabbitmq",
}

// SubmitModel is the RCA submission wizard.
type SubmitModel struct {
	step       int // 0=category, 1=services, 2=description, 3=confirm
	categories []string
	catCursor  int

	services    []string
	svcSelected map[int]bool
	svcCursor   int

	description string
	fix         string
	editingDesc bool
	editingFix  bool

	result     *SubmitResultMsg
	submitting bool

	// AI evaluation
	aiFeedback    string
	aiEvalRunning bool
	aiEvalErr     error

	// Failed result state
	showSolution bool

	// Viewport for scrollable result
	resultVP      viewport.Model
	resultVPReady bool

	scenario    *scenario.Scenario
	session     *scoring.Session
	projectRoot string
	width       int
	height      int
}

// NewSubmitModel creates a new submit model.
func NewSubmitModel(projectRoot string, sc *scenario.Scenario, sess *scoring.Session, width, height int) SubmitModel {
	return SubmitModel{
		step:        0,
		categories:  rcaCategories,
		services:    serviceOptions,
		svcSelected: make(map[int]bool),
		editingDesc: true,
		scenario:    sc,
		session:     sess,
		projectRoot: projectRoot,
		width:       width,
		height:      height,
	}
}

// Init satisfies the tea.Model interface.
func (m SubmitModel) Init() tea.Cmd {
	return nil
}

func (m *SubmitModel) initViewport() {
	w := m.width
	if w < 40 {
		w = 80
	}
	h := m.height - 4 // reserve for header + footer
	if h < 10 {
		h = 20
	}
	m.resultVP = viewport.New(w, h)
	m.resultVP.MouseWheelEnabled = true
	m.resultVPReady = true
}

// Update handles the multi-step submission wizard.
func (m SubmitModel) Update(msg tea.Msg) (SubmitModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.resultVPReady {
			m.resultVP.Width = msg.Width
			m.resultVP.Height = msg.Height - 4
		}

	case SubmitResultMsg:
		m.submitting = false
		m.result = &msg
		m.showSolution = false
		if !m.resultVPReady {
			m.initViewport()
		}
		m.resultVP.SetContent(m.buildResultContent())
		m.resultVP.GotoTop()
		if msg.Passed {
			m.aiEvalRunning = true
			return m, RunAIEval(m.scenario,
				m.categories[m.catCursor],
				m.selectedServicesList(),
				m.description,
				m.fix,
			)
		}
		return m, nil

	case AIEvalMsg:
		m.aiEvalRunning = false
		if msg.Err != nil {
			m.aiEvalErr = msg.Err
		} else {
			m.aiFeedback = msg.Feedback
		}
		// Rebuild content with AI feedback
		m.resultVP.SetContent(m.buildResultContent())
		return m, nil

	case tea.KeyMsg:
		if m.submitting {
			return m, nil
		}

		// Result screen — viewport handles scroll
		if m.result != nil {
			switch msg.String() {
			case "esc", "q":
				return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenRunning} }
			case "r":
				return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenRunning} }
			case "s":
				if !m.result.Passed && !m.showSolution {
					m.showSolution = true
					m.resultVP.SetContent(m.buildResultContent())
					m.resultVP.GotoTop()
					m.aiEvalRunning = true
					return m, RunAIEval(m.scenario,
						m.categories[m.catCursor],
						m.selectedServicesList(),
						m.description,
						m.fix,
					)
				}
			default:
				// Let viewport handle j/k/up/down/pgup/pgdn/mouse
				var cmd tea.Cmd
				m.resultVP, cmd = m.resultVP.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		switch m.step {
		case 0:
			return m.updateCategoryStep(msg)
		case 1:
			return m.updateServicesStep(msg)
		case 2:
			return m.updateDescriptionStep(msg)
		case 3:
			return m.updateConfirmStep(msg)
		}

	case tea.MouseMsg:
		// Let viewport handle mouse scroll on result screen
		if m.result != nil && m.resultVPReady {
			var cmd tea.Cmd
			m.resultVP, cmd = m.resultVP.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m SubmitModel) updateCategoryStep(msg tea.KeyMsg) (SubmitModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.catCursor > 0 {
			m.catCursor--
		}
	case "down", "j":
		if m.catCursor < len(m.categories)-1 {
			m.catCursor++
		}
	case "enter":
		m.step = 1
	case "esc":
		return m, func() tea.Msg { return ScreenChangeMsg{Screen: ScreenRunning} }
	}
	return m, nil
}

func (m SubmitModel) updateServicesStep(msg tea.KeyMsg) (SubmitModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.svcCursor > 0 {
			m.svcCursor--
		}
	case "down", "j":
		if m.svcCursor < len(m.services)-1 {
			m.svcCursor++
		}
	case " ":
		m.svcSelected[m.svcCursor] = !m.svcSelected[m.svcCursor]
	case "enter":
		m.step = 2
		m.editingDesc = true
		m.editingFix = false
	case "esc":
		m.step = 0
	}
	return m, nil
}

func (m SubmitModel) updateDescriptionStep(msg tea.KeyMsg) (SubmitModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.step = 1
		m.editingDesc = false
		m.editingFix = false
		return m, nil
	case tea.KeyTab:
		m.editingDesc = !m.editingDesc
		m.editingFix = !m.editingFix
		return m, nil
	case tea.KeyEnter:
		if m.description != "" {
			m.step = 3
			m.editingDesc = false
			m.editingFix = false
		}
		return m, nil
	case tea.KeyBackspace:
		if m.editingDesc && len(m.description) > 0 {
			m.description = m.description[:len(m.description)-1]
		} else if m.editingFix && len(m.fix) > 0 {
			m.fix = m.fix[:len(m.fix)-1]
		}
		return m, nil
	case tea.KeyRunes:
		if m.editingDesc {
			m.description += string(msg.Runes)
		} else if m.editingFix {
			m.fix += string(msg.Runes)
		}
		return m, nil
	case tea.KeySpace:
		if m.editingDesc {
			m.description += " "
		} else if m.editingFix {
			m.fix += " "
		}
		return m, nil
	}
	return m, nil
}

func (m SubmitModel) updateConfirmStep(msg tea.KeyMsg) (SubmitModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.submitting = true
		category := m.categories[m.catCursor]
		services := m.selectedServicesList()
		return m, RunSubmission(m.projectRoot, m.scenario, m.session, category, services, m.description, m.fix)
	case "esc":
		m.step = 2
		m.editingDesc = true
	}
	return m, nil
}

func (m SubmitModel) selectedServicesList() string {
	var selected []string
	for i, svc := range m.services {
		if m.svcSelected[i] {
			selected = append(selected, svc)
		}
	}
	return strings.Join(selected, ",")
}

// View renders the current step of the submission wizard.
func (m SubmitModel) View() string {
	var b strings.Builder

	if m.result != nil {
		// Header
		if m.result.Passed {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).
				Render("  INCIDENT RESOLVED"))
		} else {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorDanger).
				Render("  NOT YET RESOLVED"))
		}
		b.WriteString("\n")

		// Viewport (scrollable)
		b.WriteString(m.resultVP.View())
		b.WriteString("\n")

		// Footer with scroll indicator
		pct := int(m.resultVP.ScrollPercent() * 100)
		footer := KeyHelpStyle.Render(
			fmt.Sprintf("[j/k/scroll] Navigate (%d%%)  [r] Back to Dashboard  [s] See Solution  [esc] Back", pct))
		if m.result.Passed || m.showSolution {
			footer = KeyHelpStyle.Render(
				fmt.Sprintf("[j/k/scroll] Navigate (%d%%)  [r] Back to Dashboard  [esc] Back", pct))
		}
		b.WriteString(footer)
		return b.String()
	}

	b.WriteString(TitleStyle.Render("Submit RCA"))
	b.WriteString("\n")

	// Step indicator
	steps := []string{"Category", "Services", "Description", "Confirm"}
	var stepIndicators []string
	for i, s := range steps {
		if i == m.step {
			stepIndicators = append(stepIndicators,
				lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render(fmt.Sprintf("[%d] %s", i+1, s)))
		} else if i < m.step {
			stepIndicators = append(stepIndicators,
				lipgloss.NewStyle().Foreground(ColorSuccess).Render(fmt.Sprintf("[%d] %s", i+1, s)))
		} else {
			stepIndicators = append(stepIndicators,
				lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("[%d] %s", i+1, s)))
		}
	}
	b.WriteString(strings.Join(stepIndicators, "  >  "))
	b.WriteString("\n\n")

	if m.submitting {
		b.WriteString(PanelStyle.Render(
			lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).
				Render("Running validations..."),
		))
		return b.String()
	}

	switch m.step {
	case 0:
		b.WriteString(m.viewCategoryStep())
	case 1:
		b.WriteString(m.viewServicesStep())
	case 2:
		b.WriteString(m.viewDescriptionStep())
	case 3:
		b.WriteString(m.viewConfirmStep())
	}

	return b.String()
}

func (m SubmitModel) viewCategoryStep() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("What category best describes the root cause?"))
	b.WriteString("\n\n")

	for i, cat := range m.categories {
		if i == m.catCursor {
			b.WriteString(SelectedActionStyle.Render("> " + cat))
		} else {
			b.WriteString(ActionItemStyle.Render("  " + cat))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(KeyHelpStyle.Render("[up/down] Navigate  [enter] Select  [esc] Back"))
	return b.String()
}

func (m SubmitModel) viewServicesStep() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Which services are affected? (space to toggle)"))
	b.WriteString("\n\n")

	for i, svc := range m.services {
		checkbox := "[ ]"
		if m.svcSelected[i] {
			checkbox = lipgloss.NewStyle().Foreground(ColorSuccess).Render("[x]")
		}

		line := fmt.Sprintf("%s %s", checkbox, svc)
		if i == m.svcCursor {
			b.WriteString(SelectedActionStyle.Render("> " + line))
		} else {
			b.WriteString(ActionItemStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(KeyHelpStyle.Render("[up/down] Navigate  [space] Toggle  [enter] Continue  [esc] Back"))
	return b.String()
}

func (m SubmitModel) viewDescriptionStep() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Describe the root cause and your fix"))
	b.WriteString("\n\n")

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(60)

	descLabel := "Root cause:"
	if m.editingDesc {
		descLabel = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("Root cause:")
		inputStyle = inputStyle.BorderForeground(ColorAccent)
	} else {
		descLabel = lipgloss.NewStyle().Foreground(ColorMuted).Render("Root cause:")
		inputStyle = inputStyle.BorderForeground(ColorMuted)
	}
	descContent := m.description
	if m.editingDesc {
		descContent += lipgloss.NewStyle().Foreground(ColorAccent).Render("_")
	}
	if descContent == "" && !m.editingDesc {
		descContent = lipgloss.NewStyle().Foreground(ColorMuted).Render("(empty)")
	}
	b.WriteString(descLabel + "\n")
	b.WriteString(inputStyle.Render(descContent))
	b.WriteString("\n\n")

	fixInputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(60)

	fixLabel := "What fix did you apply?"
	if m.editingFix {
		fixLabel = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("What fix did you apply?")
		fixInputStyle = fixInputStyle.BorderForeground(ColorAccent)
	} else {
		fixLabel = lipgloss.NewStyle().Foreground(ColorMuted).Render("What fix did you apply?")
		fixInputStyle = fixInputStyle.BorderForeground(ColorMuted)
	}
	fixContent := m.fix
	if m.editingFix {
		fixContent += lipgloss.NewStyle().Foreground(ColorAccent).Render("_")
	}
	if fixContent == "" && !m.editingFix {
		fixContent = lipgloss.NewStyle().Foreground(ColorMuted).Render("(optional)")
	}
	b.WriteString(fixLabel + "\n")
	b.WriteString(fixInputStyle.Render(fixContent))

	b.WriteString("\n\n")
	b.WriteString(KeyHelpStyle.Render("[tab] Switch field  [enter] Continue  [esc] Back"))
	return b.String()
}

func (m SubmitModel) viewConfirmStep() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Review your submission"))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Width(14)
	valueStyle := lipgloss.NewStyle()

	b.WriteString(labelStyle.Render("Category:"))
	b.WriteString(valueStyle.Render(" " + m.categories[m.catCursor]))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Services:"))
	svcs := m.selectedServicesList()
	if svcs == "" {
		svcs = "(none selected)"
	}
	b.WriteString(valueStyle.Render(" " + svcs))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Root cause:"))
	b.WriteString(valueStyle.Render(" " + m.description))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Fix:"))
	fixText := m.fix
	if fixText == "" {
		fixText = "(not provided)"
	}
	b.WriteString(valueStyle.Render(" " + fixText))
	b.WriteString("\n\n")

	b.WriteString(KeyHelpStyle.Render("[enter] Submit  [esc] Go back and edit"))
	return b.String()
}

// buildResultContent generates the full scrollable content for the result viewport.
func (m SubmitModel) buildResultContent() string {
	var b strings.Builder
	result := m.result

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	dimStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	sepWidth := m.width - 4
	if sepWidth < 40 {
		sepWidth = 40
	}
	if sepWidth > 70 {
		sepWidth = 70
	}
	sep := dimStyle.Render(strings.Repeat("─", sepWidth))

	// ── VALIDATION CHECKS ──
	b.WriteString(headerStyle.Render("VALIDATION CHECKS"))
	b.WriteString("\n" + sep + "\n")
	for _, d := range result.Details {
		icon := StatusUpStyle.Render("[PASS]")
		if !d.Passed {
			icon = StatusDownStyle.Render("[FAIL]")
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n", icon, d.Name))
		if !d.Passed && d.Output != "" {
			// Show validation output for failed checks
			for _, line := range strings.Split(strings.TrimSpace(d.Output), "\n") {
				b.WriteString(fmt.Sprintf("          > %s\n", line))
			}
		}
	}
	b.WriteString("\n")

	// ── CATEGORY ──
	b.WriteString(headerStyle.Render("CATEGORY"))
	b.WriteString("\n" + sep + "\n")
	if result.CategoryMatch {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			result.ExpectedCategory,
			StatusUpStyle.Render("CORRECT")))
	} else {
		b.WriteString(fmt.Sprintf("  Yours:    %s\n", result.UserCategory))
		b.WriteString(fmt.Sprintf("  Expected: %s   %s\n",
			result.ExpectedCategory,
			StatusDownStyle.Render("WRONG")))
	}
	b.WriteString("\n")

	// FAILED without solution — show prompt
	if !result.Passed && !m.showSolution {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).
			Render("Keep debugging! Source code at:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s/services/\n", m.projectRoot))
		return b.String()
	}

	// ── YOUR ANALYSIS ──
	b.WriteString(headerStyle.Render("YOUR ANALYSIS"))
	b.WriteString("\n" + sep + "\n")
	b.WriteString(fmt.Sprintf("  Category:   %s\n", m.categories[m.catCursor]))
	b.WriteString(fmt.Sprintf("  Services:   %s\n", m.selectedServicesList()))
	b.WriteString(fmt.Sprintf("  Root cause: %s\n", m.description))
	if m.fix != "" {
		b.WriteString(fmt.Sprintf("  Fix:        %s\n", m.fix))
	}
	b.WriteString("\n")

	// ── FULL EXPLANATION (rendered as plain text, not in a box) ──
	if m.scenario.Explanation != "" {
		b.WriteString(headerStyle.Render("FULL EXPLANATION"))
		b.WriteString("\n" + sep + "\n\n")
		// Render explanation as plain text — preserves ASCII diagrams
		b.WriteString(strings.TrimSpace(m.scenario.Explanation))
		b.WriteString("\n\n")
	}

	// ── AI COACH ──
	wrapWidth := m.width - 8
	if wrapWidth < 40 {
		wrapWidth = 40
	}
	if m.aiEvalRunning {
		b.WriteString(headerStyle.Render("AI COACH"))
		b.WriteString("\n" + sep + "\n")
		b.WriteString(dimStyle.Render("  Evaluating your analysis..."))
		b.WriteString("\n\n")
	} else if m.aiFeedback != "" {
		b.WriteString(headerStyle.Render("AI COACH FEEDBACK"))
		b.WriteString("\n" + sep + "\n\n")
		cleaned := stripMarkdown(m.aiFeedback)
		b.WriteString(wordWrap(cleaned, wrapWidth))
		b.WriteString("\n\n")
	}

	// ── WAR STORY ──
	if m.scenario.WarStory != "" {
		b.WriteString(headerStyle.Render("WAR STORY"))
		b.WriteString("\n" + sep + "\n\n")
		b.WriteString(lipgloss.NewStyle().Italic(true).
			Render(strings.TrimSpace(m.scenario.WarStory)))
		b.WriteString("\n")
	}

	return b.String()
}

// stripMarkdown removes common markdown formatting so text fits cleanly in the viewport.
func stripMarkdown(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		// Convert headers: "# Header" -> "HEADER", "## Header" -> "HEADER"
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			stripped := strings.TrimLeft(trimmed, "# ")
			lines = append(lines, strings.ToUpper(stripped))
			continue
		}
		// Strip bold **text** and __text__
		line = stripPairs(line, "**")
		line = stripPairs(line, "__")
		// Strip italic *text* and _text_ (single)
		line = stripSingleMarkers(line, "*")
		// Strip inline code backticks
		line = stripPairs(line, "`")
		// Strip triple backtick fences
		if strings.HasPrefix(trimmed, "```") {
			continue
		}
		// Convert "- bullet" to "  - bullet"
		if strings.HasPrefix(trimmed, "- ") {
			line = "  " + trimmed
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// stripPairs removes paired markers like ** or ` from text.
func stripPairs(s, marker string) string {
	for {
		idx := strings.Index(s, marker)
		if idx == -1 {
			break
		}
		end := strings.Index(s[idx+len(marker):], marker)
		if end == -1 {
			break
		}
		s = s[:idx] + s[idx+len(marker):idx+len(marker)+end] + s[idx+len(marker)+end+len(marker):]
	}
	return s
}

// stripSingleMarkers removes single-char markers like * used for italic.
func stripSingleMarkers(s, marker string) string {
	for {
		idx := strings.Index(s, marker)
		if idx == -1 {
			break
		}
		// Skip if it's a double marker (already handled by stripPairs)
		if idx+1 < len(s) && string(s[idx+1]) == marker {
			break
		}
		end := strings.Index(s[idx+len(marker):], marker)
		if end == -1 {
			break
		}
		s = s[:idx] + s[idx+len(marker):idx+len(marker)+end] + s[idx+len(marker)+end+len(marker):]
	}
	return s
}
