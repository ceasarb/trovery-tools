package eval

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Suite is an eval suite loaded from YAML.
type Suite struct {
	Name      string     `yaml:"name"`
	Server    string     `yaml:"server"`
	Scenarios []Scenario `yaml:"scenarios"`
}

// Scenario is a single eval scenario within a suite.
type Scenario struct {
	Name       string         `yaml:"name"`
	Setup      []SetupStep    `yaml:"setup,omitempty"`
	Tool       string         `yaml:"tool"`
	Input      map[string]any `yaml:"input"`
	Assertions []Assertion    `yaml:"assertions"`
}

// SetupStep is a tool call executed before the main scenario tool call.
type SetupStep struct {
	Tool  string         `yaml:"tool"`
	Input map[string]any `yaml:"input"`
}

// Assertion defines a single check against a tool call result.
type Assertion struct {
	Type     string `yaml:"type"`
	Field    string `yaml:"field"`
	Expected any    `yaml:"expected"`
}

// LoadSuite reads and parses an eval suite from a YAML file.
func LoadSuite(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval suite: %w", err)
	}

	var suite Suite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse eval suite %s: %w", path, err)
	}

	if suite.Name == "" {
		suite.Name = filepath.Base(path)
	}

	return &suite, nil
}

// DiscoverSuites finds all *.eval.yaml files in a directory.
func DiscoverSuites(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read evals dir: %w", err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if matched, _ := filepath.Match("*.eval.yaml", name); matched {
			paths = append(paths, filepath.Join(dir, name))
		}
		if matched, _ := filepath.Match("*.eval.yml", name); matched {
			paths = append(paths, filepath.Join(dir, name))
		}
	}

	return paths, nil
}
