package tui

import (
	"time"

	"github.com/facets-cloud/bugbuster/internal/scenario"
)

// ScreenType identifies which screen the TUI is currently showing.
type ScreenType int

const (
	ScreenHome ScreenType = iota
	ScreenStartup
	ScreenRunning
	ScreenAPITester
	ScreenHints
	ScreenSubmit
)

// ScreenChangeMsg requests a screen transition.
type ScreenChangeMsg struct {
	Screen ScreenType
}

// DockerLogMsg carries a single line of docker compose output.
type DockerLogMsg struct {
	Line string
}

// DockerDoneMsg signals that docker compose has finished (or failed).
type DockerDoneMsg struct {
	Err error
}

// ServiceStatusMsg carries the result of docker compose ps.
type ServiceStatusMsg struct {
	Output string
	Err    error
}

// APIResultMsg carries the result of an HTTP request fired from the API tester.
type APIResultMsg struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
	Body     string
	Err      error
}

// TimerTickMsg is sent every second to update the elapsed-time display.
type TimerTickMsg struct{}

// HintRevealedMsg carries a newly revealed hint.
type HintRevealedMsg struct {
	Text  string
	Index int
	Err   error
}

// SubmitResultMsg carries the outcome of an RCA submission.
type SubmitResultMsg struct {
	Passed            bool   // CategoryMatch AND ValidationsPassed
	CategoryMatch     bool
	ValidationsPassed bool
	UserCategory      string
	ExpectedCategory  string
	Details           []ValidationResult
}

// ValidationResult is one validation check outcome within a submission.
type ValidationResult struct {
	Name   string
	Passed bool
	Output string
}

// AIEvalMsg carries the AI's evaluation of the user's RCA vs the expected answer.
type AIEvalMsg struct {
	Feedback string
	Err      error
}

// ScenarioSelectedMsg is sent when the user picks a scenario from the list.
type ScenarioSelectedMsg struct {
	Scenario *scenario.Scenario
}

// ScenariosLoadedMsg carries the list of available scenarios.
type ScenariosLoadedMsg struct {
	Scenarios []*scenario.Scenario
	Err       error
}

