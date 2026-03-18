package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ComposeFiles returns the list of docker compose file flags for a scenario.
// It always includes the base docker-compose.yml and observability overlay,
// plus the scenario-specific compose.override.yaml if it exists.
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

// Up starts the containers using docker compose up -d.
func Up(projectRoot string, composeFiles []string) error {
	args := append(composeArgs(composeFiles), "up", "-d", "--build")
	return runCompose(projectRoot, args...)
}

// Down tears down containers and volumes using docker compose down -v.
func Down(projectRoot string, composeFiles []string) error {
	args := append(composeArgs(composeFiles), "down", "-v")
	return runCompose(projectRoot, args...)
}

// Ps returns the output of docker compose ps.
func Ps(projectRoot string, composeFiles []string) (string, error) {
	args := append(composeArgs(composeFiles), "ps")
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

// runCompose executes a docker compose command, streaming output to stdout/stderr.
func runCompose(projectRoot string, args ...string) error {
	fullArgs := append([]string{"compose"}, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// UpStreaming starts docker compose up and returns a reader for streaming output.
// The caller must read from the reader and wait for the command to finish.
func UpStreaming(projectRoot string, composeFiles []string) (io.ReadCloser, *exec.Cmd, error) {
	args := append(composeArgs(composeFiles), "up", "-d", "--build")
	fullArgs := append([]string{"compose"}, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = projectRoot

	// Merge stdout and stderr into a single pipe
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, nil, fmt.Errorf("failed to start docker compose: %w", err)
	}

	// Close the write end when the command exits
	go func() {
		cmd.Wait()
		pw.Close()
	}()

	return pr, cmd, nil
}
