package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	composeCmd  []string // e.g. ["docker", "compose"] or ["docker-compose"]
	composeOnce sync.Once
)

// detectCompose figures out whether to use "docker compose" (v2) or "docker-compose" (v1).
func detectCompose() {
	// Try "docker compose version" first (v2 plugin)
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		composeCmd = []string{"docker", "compose"}
		return
	}
	// Fall back to "docker-compose" (v1 standalone)
	if _, err := exec.LookPath("docker-compose"); err == nil {
		composeCmd = []string{"docker-compose"}
		return
	}
	// Neither found — will be caught by CheckPrerequisites
	composeCmd = []string{"docker", "compose"}
}

// ComposeCommand returns the base command for docker compose.
// Use this from other packages that need to build compose commands.
func ComposeCommand() []string {
	composeOnce.Do(detectCompose)
	return composeCmd
}

// CheckPrerequisites verifies that docker and docker compose are installed and usable.
// Returns a list of errors (empty = all good).
func CheckPrerequisites() []string {
	var issues []string

	// Check docker
	if _, err := exec.LookPath("docker"); err != nil {
		issues = append(issues, "Docker is not installed. Install it from https://docs.docker.com/get-docker/")
		return issues // no point checking compose if docker is missing
	}

	// Check docker is running
	if err := exec.Command("docker", "info").Run(); err != nil {
		issues = append(issues, "Docker is not running. Start Docker Desktop or the Docker daemon.")
		return issues
	}

	// Check docker compose
	v2Err := exec.Command("docker", "compose", "version").Run()
	_, v1Err := exec.LookPath("docker-compose")
	if v2Err != nil && v1Err != nil {
		issues = append(issues, "Docker Compose is not installed.\n"+
			"         Install it: https://docs.docker.com/compose/install/\n"+
			"         Or: brew install docker-compose")
	}

	return issues
}

// ComposeFiles returns the list of docker compose file flags for a scenario.
func ComposeFiles(projectRoot, scenarioName string) []string {
	files := []string{
		filepath.Join(projectRoot, "docker-compose.yml"),
		filepath.Join(projectRoot, "docker-compose.observability.yml"),
	}

	override := filepath.Join(projectRoot, "scenarios", scenarioName, "compose.override.yaml")
	if _, err := os.Stat(override); err == nil {
		files = append(files, override)
	}

	return files
}

// composeArgs builds the -f flags for docker compose.
func composeArgs(files []string) []string {
	var args []string
	for _, f := range files {
		args = append(args, "-f", f)
	}
	return args
}

// buildCmd creates an exec.Cmd for a compose operation.
func buildCmd(projectRoot string, args ...string) *exec.Cmd {
	base := ComposeCommand()
	fullArgs := append(base[1:], args...)
	cmd := exec.Command(base[0], fullArgs...)
	cmd.Dir = projectRoot
	return cmd
}

// Up starts the containers using docker compose up -d.
func Up(projectRoot string, composeFiles []string) error {
	args := append(composeArgs(composeFiles), "up", "-d", "--build")
	cmd := buildCmd(projectRoot, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Down tears down containers and volumes using docker compose down -v.
func Down(projectRoot string, composeFiles []string) error {
	args := append(composeArgs(composeFiles), "down", "-v")
	cmd := buildCmd(projectRoot, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Ps returns the output of docker compose ps.
func Ps(projectRoot string, composeFiles []string) (string, error) {
	args := append(composeArgs(composeFiles), "ps")
	cmd := buildCmd(projectRoot, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

// UpStreaming starts docker compose up and returns a reader for streaming output.
func UpStreaming(projectRoot string, composeFiles []string) (io.ReadCloser, *exec.Cmd, error) {
	args := append(composeArgs(composeFiles), "up", "-d", "--build")
	cmd := buildCmd(projectRoot, args...)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, nil, fmt.Errorf("failed to start docker compose: %w", err)
	}

	go func() {
		cmd.Wait()
		pw.Close()
	}()

	return pr, cmd, nil
}
