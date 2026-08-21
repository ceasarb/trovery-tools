package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Suite is a collection of test fixtures loaded from YAML.
type Suite struct {
	Tests []TestCase `yaml:"tests"`
}

// TestCase is a single test fixture.
type TestCase struct {
	Name   string                 `yaml:"name"`
	Tool   string                 `yaml:"tool"`
	Input  map[string]interface{} `yaml:"input"`
	Expect Expectation            `yaml:"expect"`
}

// Expectation defines what the test expects.
type Expectation struct {
	Status          string  `yaml:"status"`
	ContentContains string  `yaml:"content_contains"`
	MaxLatencyMs    float64 `yaml:"max_latency_ms"`
}

// TestResult holds the outcome of running a single test.
type TestResult struct {
	Name     string
	Passed   bool
	Error    string
	Duration time.Duration
}

// LoadFixtures reads all YAML fixture files from a directory.
func LoadFixtures(dir string) ([]Suite, error) {
	var suites []Suite

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read fixtures dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}

		var suite Suite
		if err := yaml.Unmarshal(data, &suite); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}

		suites = append(suites, suite)
	}

	return suites, nil
}

// CheckExpectation validates a tool call result against expectations.
func CheckExpectation(expect Expectation, result *ToolCallOutput) error {
	// Check status
	if expect.Status != "" {
		if expect.Status == "success" && result.IsError {
			return fmt.Errorf("expected success, got error")
		}
		if expect.Status == "error" && !result.IsError {
			return fmt.Errorf("expected error, got success")
		}
	}

	// Check content contains
	if expect.ContentContains != "" {
		if !strings.Contains(result.Text, expect.ContentContains) {
			return fmt.Errorf("expected content to contain %q, got %q", expect.ContentContains, result.Text)
		}
	}

	// Check latency
	if expect.MaxLatencyMs > 0 {
		if result.Duration.Milliseconds() > int64(expect.MaxLatencyMs) {
			return fmt.Errorf("latency %dms exceeds max %dms", result.Duration.Milliseconds(), int64(expect.MaxLatencyMs))
		}
	}

	return nil
}

// ToolCallOutput is a simplified tool call result for assertion checking.
type ToolCallOutput struct {
	Text     string
	IsError  bool
	Duration time.Duration
}
