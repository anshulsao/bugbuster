package tui

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/facets-cloud/bugbuster/internal/docker"
	"github.com/facets-cloud/bugbuster/internal/hints"
	"github.com/facets-cloud/bugbuster/internal/scenario"
	"github.com/facets-cloud/bugbuster/internal/scoring"
)

// LoadScenarios returns a command that lists all available scenarios.
func LoadScenarios(projectRoot string) tea.Cmd {
	return func() tea.Msg {
		scenarios, err := scenario.ListAll(projectRoot)
		return ScenariosLoadedMsg{Scenarios: scenarios, Err: err}
	}
}


// dockerStreamMsg delivers the log channel to the startup model after docker starts.
type dockerStreamMsg struct {
	lines <-chan string
	errs  <-chan error
}

// StartDocker starts docker compose and returns a channel-based stream.
// The startup model receives a dockerStreamMsg, then chains WaitForDockerLine calls.
func StartDocker(projectRoot string, composeFiles []string) tea.Cmd {
	return func() tea.Msg {
		base := docker.ComposeCommand()
		var args []string
		for _, f := range composeFiles {
			args = append(args, "-f", f)
		}
		args = append(args, "up", "-d", "--build")
		fullArgs := append(base[1:], args...)

		cmd := exec.Command(base[0], fullArgs...)
		cmd.Dir = projectRoot

		pr, pw := io.Pipe()
		cmd.Stdout = pw
		cmd.Stderr = pw

		if err := cmd.Start(); err != nil {
			return DockerDoneMsg{Err: fmt.Errorf("failed to start docker compose: %w", err)}
		}

		lines := make(chan string, 100)
		errs := make(chan error, 1)

		// Wait for docker to finish and close the pipe writer so scanner gets EOF.
		go func() {
			err := cmd.Wait()
			pw.Close()
			errs <- err
		}()

		// Read lines from pipe reader until EOF (pw closed above).
		go func() {
			scanner := bufio.NewScanner(pr)
			scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
			for scanner.Scan() {
				lines <- scanner.Text()
			}
			close(lines)
		}()

		return dockerStreamMsg{lines: lines, errs: errs}
	}
}

// WaitForDockerLine reads the next line from the docker output channel.
func WaitForDockerLine(lines <-chan string, errs <-chan error) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lines
		if !ok {
			// Channel closed — docker finished
			var err error
			select {
			case err = <-errs:
			default:
			}
			return DockerDoneMsg{Err: err}
		}
		return DockerLogMsg{Line: line}
	}
}

// PollServices returns a command that runs docker compose ps --format json and reports status.
func PollServices(projectRoot string, composeFiles []string) tea.Cmd {
	return func() tea.Msg {
		base := docker.ComposeCommand()
		var args []string
		for _, f := range composeFiles {
			args = append(args, "-f", f)
		}
		args = append(args, "ps", "--format", "json")
		fullArgs := append(base[1:], args...)
		cmd := exec.Command(base[0], fullArgs...)
		cmd.Dir = projectRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			return ServiceStatusMsg{Err: fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)}
		}
		return ServiceStatusMsg{Output: string(out)}
	}
}

// FireAPI makes an HTTP request and returns the result as an APIResultMsg.
func FireAPI(baseURL, method, path string, body io.Reader) tea.Cmd {
	return func() tea.Msg {
		fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
		req, err := http.NewRequest(method, fullURL, body)
		if err != nil {
			return APIResultMsg{Method: method, Path: path, Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}

		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)
		if err != nil {
			return APIResultMsg{Method: method, Path: path, Duration: elapsed, Err: err}
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
		if err != nil {
			return APIResultMsg{
				Method:   method,
				Path:     path,
				Status:   resp.StatusCode,
				Duration: elapsed,
				Err:      fmt.Errorf("reading response body: %w", err),
			}
		}

		return APIResultMsg{
			Method:   method,
			Path:     path,
			Status:   resp.StatusCode,
			Duration: elapsed,
			Body:     string(respBody),
		}
	}
}

// TimerTick returns a command that sends a TimerTickMsg every second.
func TimerTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return TimerTickMsg{}
	})
}

// RevealHint reveals the next hint and persists the session.
func RevealHint(projectRoot string, sc *scenario.Scenario, sess *scoring.Session) tea.Cmd {
	return func() tea.Msg {
		text, _, index, err := hints.NextHint(sc.Hints, sess.HintsUsed)
		if err != nil {
			return HintRevealedMsg{Err: err}
		}

		sess.HintsUsed = append(sess.HintsUsed, index)

		if saveErr := scoring.SaveSession(projectRoot, sess); saveErr != nil {
			return HintRevealedMsg{Err: fmt.Errorf("saving session: %w", saveErr)}
		}

		return HintRevealedMsg{Text: text, Index: index}
	}
}

// RunSubmission validates the user's RCA submission against the scenario's
// expected RCA and validation checks.
func RunSubmission(projectRoot string, sc *scenario.Scenario, sess *scoring.Session, category, services, description, fix string) tea.Cmd {
	return func() tea.Msg {
		// Run each validation command.
		var details []ValidationResult
		allPassed := true
		for _, v := range sc.Validation {
			passed, output := scenario.RunValidation(projectRoot, v)
			details = append(details, ValidationResult{
				Name:   v.Name,
				Passed: passed,
				Output: output,
			})
			if !passed {
				allPassed = false
			}
		}

		// Check category match.
		categoryMatch := strings.EqualFold(category, sc.ExpectedRCA.Category)
		overallPassed := allPassed && categoryMatch

		// Persist submission.
		sess.Submission = &scoring.Submission{
			Category:    category,
			Services:    services,
			Description: description,
			Fix:         fix,
		}
		if overallPassed {
			sess.Active = false
		}
		_ = scoring.SaveSession(projectRoot, sess)

		return SubmitResultMsg{
			Passed:            overallPassed,
			CategoryMatch:     categoryMatch,
			ValidationsPassed: allPassed,
			UserCategory:      category,
			ExpectedCategory:  sc.ExpectedRCA.Category,
			Details:           details,
		}
	}
}

// browserOpenedMsg is a no-op message sent after opening a browser.
type browserOpenedMsg struct{}

// OpenBrowser opens the given URL in the default browser (macOS).
func OpenBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("open", url).Start()
		return browserOpenedMsg{}
	}
}

// ServicePollTick returns a command that ticks every 10 seconds for service polling.
func ServicePollTick() tea.Cmd {
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg {
		return ServiceStatusMsg{}
	})
}

// detectAICLI checks for claude or gemini CLI availability.
// Returns ("claude", args) or ("gemini", args) or ("", nil).
func detectAICLI() (string, []string) {
	if _, err := exec.LookPath("claude"); err == nil {
		return "claude", []string{"--dangerously-skip-permissions", "-p"}
	}
	if _, err := exec.LookPath("gemini"); err == nil {
		return "gemini", []string{"--yolo", "-p"}
	}
	return "", nil
}

// RunAIEval uses an available AI CLI to evaluate the user's RCA against the expected answer.
func RunAIEval(sc *scenario.Scenario, userCategory, userServices, userDescription, userFix string) tea.Cmd {
	return func() tea.Msg {
		cli, baseArgs := detectAICLI()
		if cli == "" {
			return AIEvalMsg{Err: fmt.Errorf("no AI CLI available")}
		}

		prompt := fmt.Sprintf(`You are evaluating a junior engineer's Root Cause Analysis for an incident debugging exercise.

EXPECTED ANSWER:
- Category: %s
- Root Cause: %s
- Key concepts they should mention: %s

ENGINEER'S SUBMISSION:
- Category: %s
- Affected services: %s
- Their root cause description: %s
- Their proposed fix: %s

Give a brief, encouraging evaluation (3-5 bullet points max):
1. What they got right
2. What they missed or got wrong
3. One key learning they should take away

Be specific and constructive. Reference the actual technical concepts (thread pools, accept queues, fail-fast, USE method, etc). Keep it under 200 words. Do not use markdown headers.`,
			sc.ExpectedRCA.Category,
			sc.ExpectedRCA.Description,
			strings.Join(sc.ExpectedRCA.Keywords, ", "),
			userCategory,
			userServices,
			userDescription,
			userFix,
		)

		args := append(baseArgs, prompt)
		cmd := exec.Command(cli, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return AIEvalMsg{Err: fmt.Errorf("%s failed: %s: %w", cli, strings.TrimSpace(string(out)), err)}
		}

		return AIEvalMsg{Feedback: strings.TrimSpace(string(out))}
	}
}
