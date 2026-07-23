package eval

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Suite defines an agent evaluation suite loaded from YAML.
type Suite struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Agent       string     `yaml:"agent"`
	Settings    Settings   `yaml:"settings,omitempty"`
	Scenarios   []Scenario `yaml:"scenarios,omitempty"`
	Cases       []Scenario `yaml:"cases,omitempty"` // alias for scenarios
}

// Settings configures suite-level overrides.
type Settings struct {
	Model       string  `yaml:"model,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty"`
}

// Scenario defines a single test scenario.
// Supports both `input` (single-turn shorthand) and `messages` (multi-turn).
type Scenario struct {
	Name       string      `yaml:"name"`
	Input      string      `yaml:"input,omitempty"`      // single-turn shorthand
	Messages   []Message   `yaml:"messages,omitempty"`    // multi-turn
	Tags       []string    `yaml:"tags,omitempty"`
	Assertions []Assertion `yaml:"assertions"`
	MockErrors []MockError `yaml:"mock_errors,omitempty"`
	Runs       int         `yaml:"runs,omitempty"` // multi-run count (default: 1)
}

// Message represents a conversation turn in a scenario.
type Message struct {
	Role    string `yaml:"role"`
	Content string `yaml:"content"`
}

// MockError configures a forced tool failure for testing resilience.
type MockError struct {
	Tool  string `yaml:"tool"`
	Error string `yaml:"error"`
	Count int    `yaml:"count,omitempty"` // 0 = always fail
}

// Assertion defines a check to run against captured data.
// Supports both the generic `expected` field and named fields for clarity.
type Assertion struct {
	Type     string   `yaml:"type"`
	Expected any      `yaml:"expected,omitempty"` // generic
	Tools    []string `yaml:"tools,omitempty"`    // tool_called, tool_not_called
	Values   []string `yaml:"values,omitempty"`   // output_contains, output_not_contains
	Pattern  string   `yaml:"pattern,omitempty"`  // output_matches
	Count    int      `yaml:"count,omitempty"`    // min_tool_calls, max_tool_calls, max_turns
	Amount   float64  `yaml:"amount,omitempty"`   // max_cost_usd
	Tokens   int      `yaml:"tokens,omitempty"`   // max_tokens_used
	Seconds  float64  `yaml:"seconds,omitempty"`  // max_latency_seconds
}

// CapturedRun holds data collected from a single scenario execution.
type CapturedRun struct {
	ToolCalls   []CapturedToolCall
	FinalOutput string
	TotalTokens int
	TotalCost   float64
	TurnCount   int
	Duration    time.Duration
}

// CapturedToolCall records a single tool invocation.
type CapturedToolCall struct {
	Name      string
	Arguments map[string]any
	Result    string
	Error     string
	Duration  time.Duration
}

// LoadSuite reads and parses an eval suite YAML file.
func LoadSuite(path string) (*Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval suite: %w", err)
	}

	var suite Suite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse eval suite: %w", err)
	}

	// Support `cases` as alias for `scenarios`
	if len(suite.Cases) > 0 && len(suite.Scenarios) == 0 {
		suite.Scenarios = suite.Cases
	}
	suite.Cases = nil

	// Normalize scenarios
	for i := range suite.Scenarios {
		s := &suite.Scenarios[i]

		// Default runs to 1
		if s.Runs <= 0 {
			s.Runs = 1
		}

		// Convert `input` shorthand to `messages`
		if s.Input != "" && len(s.Messages) == 0 {
			s.Messages = []Message{{Role: "user", Content: s.Input}}
		}
	}

	return &suite, nil
}
