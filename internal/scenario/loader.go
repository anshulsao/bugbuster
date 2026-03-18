package scenario

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Scenario represents a parsed scenario.yaml file.
type Scenario struct {
	Name                 string       `yaml:"name"`
	Level                int          `yaml:"level"`
	EstimatedTimeMinutes int          `yaml:"estimated_time_minutes"`
	Incident             Incident     `yaml:"incident"`
	Injection            []Injection  `yaml:"injection"`
	ExpectedRCA          ExpectedRCA  `yaml:"expected_rca"`
	Hints                []Hint       `yaml:"hints"`
	Explanation          string       `yaml:"explanation"`
	WarStory             string       `yaml:"war_story"`
	Validation           []Validation `yaml:"validation"`

	// Dir is the directory name (not from YAML, set by loader)
	Dir string `yaml:"-"`
}

type Incident struct {
	Alert       string `yaml:"alert"`
	Observation string `yaml:"observation"`
}

type Injection struct {
	Type   string            `yaml:"type"`
	Target string            `yaml:"target"`
	Config map[string]string `yaml:"config"`
}

type ExpectedRCA struct {
	Category    string   `yaml:"category"`
	Keywords    []string `yaml:"keywords"`
	Description string   `yaml:"description"`
}

type Hint struct {
	Cost int    `yaml:"cost"`
	Text string `yaml:"text"`
}

type Validation struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
	Expect  string `yaml:"expect"`
}

// Load reads and parses a scenario YAML from scenarios/<name>/scenario.yaml
func Load(projectRoot, name string) (*Scenario, error) {
	path := filepath.Join(projectRoot, "scenarios", name, "scenario.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario '%s' not found: %w", name, err)
	}

	var sc Scenario
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("invalid scenario YAML: %w", err)
	}
	sc.Dir = name
	return &sc, nil
}

// ListAll scans the scenarios/ directory and loads metadata from each scenario.yaml.
func ListAll(projectRoot string) ([]*Scenario, error) {
	scenariosDir := filepath.Join(projectRoot, "scenarios")
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read scenarios directory: %w", err)
	}

	var results []*Scenario
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sc, err := Load(projectRoot, e.Name())
		if err != nil {
			// Skip directories without a valid scenario.yaml
			continue
		}
		results = append(results, sc)
	}
	return results, nil
}
