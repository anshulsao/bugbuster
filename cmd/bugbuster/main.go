package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	bugbuster "github.com/facets-cloud/bugbuster"
	"github.com/facets-cloud/bugbuster/internal/docker"
	"github.com/facets-cloud/bugbuster/internal/hints"
	"github.com/facets-cloud/bugbuster/internal/scenario"
	"github.com/facets-cloud/bugbuster/internal/scoring"
	"github.com/facets-cloud/bugbuster/internal/tui"
	"github.com/facets-cloud/bugbuster/internal/workspace"
	"github.com/spf13/cobra"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Red       = "\033[31m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Blue      = "\033[34m"
	Magenta   = "\033[35m"
	Cyan      = "\033[36m"
	White     = "\033[37m"
	BgRed     = "\033[41m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
)

var version = "dev"

const repoURL = "https://github.com/anshulsao/bugbuster"

const banner = `
` + Red + Bold + `
  ____              ____            _
 | __ ) _   _  __ _| __ ) _   _ ___| |_ ___ _ __
 |  _ \| | | |/ _` + "`" + ` |  _ \| | | / __| __/ _ \ '__|
 | |_) | |_| | (_| | |_) | |_| \__ \ ||  __/ |
 |____/ \__,_|\__, |____/ \__,_|___/\__\___|_|
              |___/
` + Reset + Dim + `  Incident Response Training Platform
  Source: ` + repoURL + Reset + `
`

func main() {
	rootCmd := &cobra.Command{
		Use:   "bugbuster",
		Short: "BugBuster - Incident Response Training Platform",
		Long:  banner + "\n  Practice debugging real-world production incidents in safe Docker environments.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Only check Docker prereqs for commands that need it
			needsDocker := map[string]bool{
				"start": true, "stop": true, "submit": true,
				"status": true, "bugbuster": true,
			}
			if !needsDocker[cmd.Name()] {
				return nil
			}
			return checkPrerequisites()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := ensureWorkspace()
			if err != nil {
				return fmt.Errorf("failed to set up workspace: %w", err)
			}
			m := tui.NewModel(root)
			p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
			_, err = p.Run()
			return err
		},
	}

	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(hintCmd())
	rootCmd.AddCommand(submitCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(leaderboardCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ensureWorkspace extracts embedded assets to ~/.bugbuster/workspace/ and returns the path.
func ensureWorkspace() (string, error) {
	return workspace.Extract(bugbuster.Assets, version)
}

func projectRoot() string {
	// First check cwd — if we're inside the repo, use it directly
	dir, _ := os.Getwd()
	d := dir
	for {
		if _, err := os.Stat(filepath.Join(d, "docker-compose.yml")); err == nil {
			if _, err := os.Stat(filepath.Join(d, "scenarios")); err == nil {
				return d
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}

	// Not in repo — extract embedded assets and use workspace
	wsDir, err := ensureWorkspace()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to extract workspace: %s\n", err)
		return dir
	}
	return wsDir
}

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <scenario>",
		Short: "Start an incident scenario",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			root := projectRoot()
			name := args[0]

			fmt.Print(banner)

			// Check for existing session
			sess, err := scoring.LoadSession(root)
			if err == nil && sess.Active {
				fmt.Printf("%s%s[!] Session already active for scenario '%s'.%s\n", Bold, Yellow, sess.Scenario, Reset)
				fmt.Printf("    Run %sbugbuster stop%s first, or %sbugbuster status%s to check progress.\n", Cyan, Reset, Cyan, Reset)
				return
			}

			// Load scenario
			sc, err := scenario.Load(root, name)
			if err != nil {
				fmt.Printf("%s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
				return
			}

			fmt.Printf("%s%s[*] Loading scenario: %s%s\n", Bold, Cyan, sc.Name, Reset)
			fmt.Printf("    Level: %s  |  Est. time: %d min\n\n", levelBadge(sc.Level), sc.EstimatedTimeMinutes)

			// Start containers
			fmt.Printf("%s%s[*] Starting environment...%s\n", Bold, Blue, Reset)
			composeFiles := docker.ComposeFiles(root, name)
			if err := docker.Up(root, composeFiles); err != nil {
				fmt.Printf("%s%s[ERROR] Failed to start containers: %s%s\n", Bold, Red, err, Reset)
				return
			}
			fmt.Printf("%s%s[OK] Environment is up.%s\n\n", Bold, Green, Reset)

			// Create session
			if err := scoring.NewSession(root, name); err != nil {
				fmt.Printf("%s%s[ERROR] Failed to create session: %s%s\n", Bold, Red, err, Reset)
				return
			}

			// Display incident alert
			printAlertBox(sc)

			// Show observability URLs
			fmt.Printf("  %s%sYour investigation toolkit:%s\n\n", Bold, Cyan, Reset)
			fmt.Printf("    %sGrafana%s     http://localhost:3000       %s(admin / bugbuster)%s\n", Bold, Reset, Dim, Reset)
			fmt.Printf("    %sJaeger%s      http://localhost:16686      %s(distributed traces)%s\n", Bold, Reset, Dim, Reset)
			fmt.Printf("    %sPrometheus%s  http://localhost:9091       %s(raw metrics)%s\n", Bold, Reset, Dim, Reset)
			fmt.Printf("    %sRabbitMQ%s    http://localhost:15672      %s(bugbuster / bugbuster)%s\n", Bold, Reset, Dim, Reset)
			fmt.Printf("    %sMailHog%s     http://localhost:8025       %s(email viewer)%s\n", Bold, Reset, Dim, Reset)
			fmt.Printf("    %sAPI%s         http://localhost:8888       %s(api gateway)%s\n\n", Bold, Reset, Dim, Reset)
			fmt.Printf("  %sTimer started. Good luck, engineer.%s\n\n", Dim, Reset)
		},
	}
}

func hintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hint",
		Short: "Get the next hint (costs points)",
		Run: func(cmd *cobra.Command, args []string) {
			root := projectRoot()

			sess, err := scoring.LoadSession(root)
			if err != nil || !sess.Active {
				fmt.Printf("%s%s[!] No active session. Run 'bugbuster start <scenario>' first.%s\n", Bold, Yellow, Reset)
				return
			}

			sc, err := scenario.Load(root, sess.Scenario)
			if err != nil {
				fmt.Printf("%s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
				return
			}

			hint, cost, idx, err := hints.NextHint(sc.Hints, sess.HintsUsed)
			if err != nil {
				fmt.Printf("%s%s[!] %s%s\n", Bold, Yellow, err, Reset)
				return
			}

			// Deduct points and record hint
			sess.Points -= cost
			sess.HintsUsed = append(sess.HintsUsed, idx)
			if err := scoring.SaveSession(root, sess); err != nil {
				fmt.Printf("%s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
				return
			}

			fmt.Printf("\n%s%s  HINT #%d  (-%d points)  %s\n", Bold, BgYellow, idx+1, cost, Reset)
			fmt.Printf("\n  %s%s%s\n\n", Yellow, hint, Reset)
			fmt.Printf("  Points remaining: %s%s%d%s\n\n", Bold, pointsColor(sess.Points), sess.Points, Reset)
		},
	}
}

func submitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "submit",
		Short: "Submit your root cause analysis and validate the fix",
		Run: func(cmd *cobra.Command, args []string) {
			root := projectRoot()

			sess, err := scoring.LoadSession(root)
			if err != nil || !sess.Active {
				fmt.Printf("%s%s[!] No active session. Run 'bugbuster start <scenario>' first.%s\n", Bold, Yellow, Reset)
				return
			}

			sc, err := scenario.Load(root, sess.Scenario)
			if err != nil {
				fmt.Printf("%s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
				return
			}

			// Prompt for RCA
			fmt.Printf("\n%s%s  SUBMIT ROOT CAUSE ANALYSIS  %s\n\n", Bold, BgBlue, Reset)

			categories := []string{
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
			fmt.Printf("%sRoot cause categories:%s\n", Bold, Reset)
			for i, c := range categories {
				fmt.Printf("  %s%d%s) %s\n", Cyan, i+1, Reset, c)
			}
			fmt.Printf("\n%sSelect category [1-%d]: %s", Bold, len(categories), Reset)
			var catIdx int
			fmt.Scanln(&catIdx)
			if catIdx < 1 || catIdx > len(categories) {
				fmt.Printf("%s%s[!] Invalid selection.%s\n", Bold, Red, Reset)
				return
			}
			selectedCategory := categories[catIdx-1]

			fmt.Printf("%sAffected service(s) (comma-separated): %s", Bold, Reset)
			var services string
			fmt.Scanln(&services)

			fmt.Printf("%sDescribe the root cause: %s", Bold, Reset)
			var description string
			fmt.Scanln(&description)

			fmt.Printf("%sWhat fix did you apply? %s", Bold, Reset)
			var fix string
			fmt.Scanln(&fix)

			// Check RCA match
			rcaMatch := selectedCategory == sc.ExpectedRCA.Category
			if rcaMatch {
				fmt.Printf("\n%s%s  [CORRECT] Root cause category matches!  %s\n", Bold, Green, Reset)
			} else {
				fmt.Printf("\n%s%s  [MISMATCH] Expected category: %s  %s\n", Bold, Red, sc.ExpectedRCA.Category, Reset)
				sess.Points -= 100 // penalty for wrong category
			}

			// Run validation checks
			fmt.Printf("\n%s%s[*] Running validation checks...%s\n\n", Bold, Cyan, Reset)
			allPassed := true
			for _, v := range sc.Validation {
				passed, output := scenario.RunValidation(root, v)
				if passed {
					fmt.Printf("  %s[PASS]%s %s\n", Green, Reset, v.Name)
				} else {
					fmt.Printf("  %s[FAIL]%s %s\n", Red, Reset, v.Name)
					if output != "" {
						fmt.Printf("        %s%s%s\n", Dim, output, Reset)
					}
					allPassed = false
				}
			}

			// Apply time penalty
			elapsed := scoring.ElapsedMinutes(sess)
			if elapsed > float64(sc.EstimatedTimeMinutes) {
				overMinutes := int(elapsed) - sc.EstimatedTimeMinutes
				timePenalty := overMinutes * 10
				sess.Points -= timePenalty
				fmt.Printf("\n  %sTime penalty: -%d points (%d min over estimate)%s\n", Yellow, timePenalty, overMinutes, Reset)
			}

			if sess.Points < 0 {
				sess.Points = 0
			}

			// Final score
			fmt.Printf("\n%s%s", Bold, strings.Repeat("=", 50))
			if allPassed && rcaMatch {
				fmt.Printf("\n%s  INCIDENT RESOLVED!  %s\n", Green, Reset)
				fmt.Printf("%s  Final Score: %s%d / 1000%s\n", Bold, Green, sess.Points, Reset)
				scoring.RecordLeaderboard(root, sess.Scenario, sess.Points, elapsed)
			} else {
				fmt.Printf("\n%s  NOT YET RESOLVED  %s\n", Red, Reset)
				fmt.Printf("%s  Current Score: %s%d / 1000%s\n", Bold, Yellow, sess.Points, Reset)
				fmt.Printf("  Fix the remaining issues and submit again.\n")
			}
			fmt.Printf("%s%s%s\n\n", Bold, strings.Repeat("=", 50), Reset)

			sess.Submission = &scoring.Submission{
				Category:    selectedCategory,
				Services:    services,
				Description: description,
				Fix:         fix,
			}
			if allPassed && rcaMatch {
				sess.Active = false
			}
			scoring.SaveSession(root, sess)
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current session status",
		Run: func(cmd *cobra.Command, args []string) {
			root := projectRoot()

			sess, err := scoring.LoadSession(root)
			if err != nil || !sess.Active {
				fmt.Printf("%s%s[!] No active session.%s\n", Bold, Yellow, Reset)
				return
			}

			sc, err := scenario.Load(root, sess.Scenario)
			if err != nil {
				fmt.Printf("%s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
				return
			}

			elapsed := scoring.ElapsedMinutes(sess)

			fmt.Printf("\n%s%s  SESSION STATUS  %s\n\n", Bold, BgMagenta, Reset)
			fmt.Printf("  Scenario:    %s%s%s\n", Cyan, sc.Name, Reset)
			fmt.Printf("  Level:       %s\n", levelBadge(sc.Level))
			fmt.Printf("  Elapsed:     %s%.1f min%s", Bold, elapsed, Reset)
			if elapsed > float64(sc.EstimatedTimeMinutes) {
				fmt.Printf("  %s(over %d min estimate!)%s", Red, sc.EstimatedTimeMinutes, Reset)
			}
			fmt.Println()
			fmt.Printf("  Points:      %s%s%d%s\n", Bold, pointsColor(sess.Points), sess.Points, Reset)
			fmt.Printf("  Hints used:  %d / %d\n\n", len(sess.HintsUsed), len(sc.Hints))

			// Show container status
			fmt.Printf("%s%s  CONTAINERS  %s\n\n", Bold, BgBlue, Reset)
			composeFiles := docker.ComposeFiles(root, sess.Scenario)
			output, err := docker.Ps(root, composeFiles)
			if err != nil {
				fmt.Printf("  %s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
			} else {
				fmt.Println(output)
			}
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available scenarios",
		Run: func(cmd *cobra.Command, args []string) {
			root := projectRoot()
			scenarios, err := scenario.ListAll(root)
			if err != nil {
				fmt.Printf("%s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
				return
			}

			if len(scenarios) == 0 {
				fmt.Printf("%s%s[!] No scenarios found in scenarios/ directory.%s\n", Bold, Yellow, Reset)
				return
			}

			fmt.Printf("\n%s%s  AVAILABLE SCENARIOS  %s\n\n", Bold, BgBlue, Reset)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintf(w, "  %s#\tNAME\tLEVEL\tTIME\tCOMMAND%s\n", Bold, Reset)
			fmt.Fprintf(w, "  %s─\t────\t─────\t────\t───────%s\n", Dim, Reset)
			for i, s := range scenarios {
				fmt.Fprintf(w, "  %d\t%s\t%s\t%d min\t%sbugbuster start %s%s\n",
					i+1, s.Name, levelBadge(s.Level), s.EstimatedTimeMinutes, Cyan, s.Dir, Reset)
			}
			w.Flush()
			fmt.Printf("\n  %sPick a scenario and run the command to begin.%s\n\n", Dim, Reset)
		},
	}
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the current scenario and tear down containers",
		Run: func(cmd *cobra.Command, args []string) {
			root := projectRoot()

			sess, err := scoring.LoadSession(root)
			if err != nil || !sess.Active {
				fmt.Printf("%s%s[!] No active session.%s\n", Bold, Yellow, Reset)
				return
			}

			fmt.Printf("%s%s[*] Stopping environment for '%s'...%s\n", Bold, Blue, sess.Scenario, Reset)
			composeFiles := docker.ComposeFiles(root, sess.Scenario)
			if err := docker.Down(root, composeFiles); err != nil {
				fmt.Printf("%s%s[ERROR] %s%s\n", Bold, Red, err, Reset)
				return
			}

			sess.Active = false
			scoring.SaveSession(root, sess)

			fmt.Printf("%s%s[OK] Environment torn down. Session ended.%s\n", Bold, Green, Reset)
		},
	}
}

func leaderboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "leaderboard",
		Short: "Show the local leaderboard",
		Run: func(cmd *cobra.Command, args []string) {
			root := projectRoot()
			entries, err := scoring.LoadLeaderboard(root)
			if err != nil || len(entries) == 0 {
				fmt.Printf("%s%s[!] No leaderboard entries yet. Solve a scenario first!%s\n", Bold, Yellow, Reset)
				return
			}

			fmt.Printf("\n%s%s  LEADERBOARD  %s\n\n", Bold, BgMagenta, Reset)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintf(w, "  %s#\tSCENARIO\tSCORE\tTIME\tDATE%s\n", Bold, Reset)
			fmt.Fprintf(w, "  %s─\t────────\t─────\t────\t────%s\n", Dim, Reset)
			for i, e := range entries {
				medal := fmt.Sprintf("%d", i+1)
				if i == 0 {
					medal = Yellow + "1" + Reset
				}
				fmt.Fprintf(w, "  %s\t%s\t%s%d%s\t%.1f min\t%s\n",
					medal, e.Scenario, pointsColor(e.Score), e.Score, Reset, e.TimeMinutes, e.Date)
			}
			w.Flush()
			fmt.Println()
		},
	}
}

// --- helpers ---

func printAlertBox(sc *scenario.Scenario) {
	width := 60
	border := strings.Repeat("═", width)

	fmt.Printf("\n  %s%s╔%s╗%s\n", Bold, Red, border, Reset)
	fmt.Printf("  %s%s║%s  INCIDENT ALERT%s║%s\n", Bold, Red, BgRed+White+Bold, strings.Repeat(" ", width-16), Reset)
	fmt.Printf("  %s%s╠%s╣%s\n", Bold, Red, border, Reset)

	// Word-wrap the alert text
	lines := wordWrap(sc.Incident.Alert, width-4)
	for _, line := range lines {
		padding := width - len(line) - 2
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("  %s%s║%s %s%s%s║%s\n", Bold, Red, Reset, Yellow, line, strings.Repeat(" ", padding+1), Reset)
	}

	fmt.Printf("  %s%s╠%s╣%s\n", Bold, Red, border, Reset)

	// Observation
	obsLines := wordWrap(sc.Incident.Observation, width-4)
	for _, line := range obsLines {
		padding := width - len(line) - 2
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("  %s%s║%s %s%s%s║%s\n", Bold, Red, Reset, Dim, line, strings.Repeat(" ", padding+1), Reset)
	}

	fmt.Printf("  %s%s╚%s╝%s\n\n", Bold, Red, border, Reset)

	fmt.Printf("  %sUse %sbugbuster hint%s for help, %sbugbuster submit%s when ready.%s\n\n",
		Dim, Cyan, Dim, Cyan, Dim, Reset)
}

func wordWrap(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		if len(current)+1+len(w) > maxWidth {
			lines = append(lines, current)
			current = w
		} else {
			current += " " + w
		}
	}
	lines = append(lines, current)
	return lines
}

func levelBadge(level int) string {
	switch {
	case level <= 1:
		return Green + "EASY" + Reset
	case level == 2:
		return Yellow + "MEDIUM" + Reset
	case level == 3:
		return Red + "HARD" + Reset
	default:
		return Magenta + "EXTREME" + Reset
	}
}

func pointsColor(pts int) string {
	switch {
	case pts >= 800:
		return Green
	case pts >= 500:
		return Yellow
	default:
		return Red
	}
}

func checkPrerequisites() error {
	issues := docker.CheckPrerequisites()
	if len(issues) == 0 {
		return nil
	}

	fmt.Printf("\n%s%s  PREREQUISITES CHECK FAILED  %s\n\n", Bold, BgRed, Reset)
	for _, issue := range issues {
		fmt.Printf("  %s%s[X]%s %s\n", Bold, Red, Reset, issue)
	}
	fmt.Printf("\n  %sInstall the prerequisites above and try again.%s\n\n", Dim, Reset)
	return fmt.Errorf("prerequisites not met")
}
