package eval

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/storage"
)

// --- Suite YAML parsing ---

func TestLoadSuite(t *testing.T) {
	yaml := `
name: basic-eval
agent: assistant
scenarios:
  - name: greeting
    messages:
      - role: user
        content: "Hello"
    assertions:
      - type: output_contains
        expected: "hello"
      - type: max_turns
        expected: 3
    runs: 5
  - name: tool-use
    messages:
      - role: user
        content: "Look up the weather"
    assertions:
      - type: tool_called
        expected: get_weather
    mock_errors:
      - tool: get_weather
        error: "connection timeout"
        count: 2
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}

	if suite.Name != "basic-eval" {
		t.Errorf("Name = %q, want basic-eval", suite.Name)
	}
	if len(suite.Scenarios) != 2 {
		t.Fatalf("Scenarios = %d, want 2", len(suite.Scenarios))
	}
	if suite.Scenarios[0].Runs != 5 {
		t.Errorf("s0.Runs = %d, want 5", suite.Scenarios[0].Runs)
	}
	if suite.Scenarios[1].MockErrors[0].Count != 2 {
		t.Errorf("MockError.Count = %d, want 2", suite.Scenarios[1].MockErrors[0].Count)
	}
}

func TestLoadSuiteCasesAlias(t *testing.T) {
	yaml := `
name: old-style
agent: my-agent
cases:
  - name: test1
    input: "Hello there"
    assertions:
      - type: output_contains
        values: ["Hello"]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "old.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	if len(suite.Scenarios) != 1 {
		t.Fatalf("Scenarios = %d, want 1", len(suite.Scenarios))
	}
	// input should be converted to messages
	if len(suite.Scenarios[0].Messages) != 1 {
		t.Fatalf("Messages = %d, want 1", len(suite.Scenarios[0].Messages))
	}
	if suite.Scenarios[0].Messages[0].Content != "Hello there" {
		t.Errorf("Content = %q", suite.Scenarios[0].Messages[0].Content)
	}
}

func TestLoadSuiteInputShorthand(t *testing.T) {
	yaml := `
name: test
agent: bot
scenarios:
  - name: simple
    input: "What's the weather?"
    assertions:
      - type: tool_called
        tools: [get_weather]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	s := suite.Scenarios[0]
	if len(s.Messages) != 1 || s.Messages[0].Role != "user" {
		t.Errorf("input shorthand not converted: messages=%v", s.Messages)
	}
}

func TestLoadSuiteWithSettings(t *testing.T) {
	yaml := `
name: configured
agent: bot
settings:
  model: claude-sonnet-4-6
  temperature: 0.0
scenarios:
  - name: s1
    input: "hi"
    assertions: []
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	if suite.Settings.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q", suite.Settings.Model)
	}
}

func TestLoadSuiteWithTags(t *testing.T) {
	yaml := `
name: tagged
agent: bot
cases:
  - name: s1
    input: "test"
    tags: [happy-path, sog]
    assertions: []
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	if len(suite.Scenarios[0].Tags) != 2 {
		t.Errorf("Tags = %v", suite.Scenarios[0].Tags)
	}
}

// --- Assertions: tool_called ---

func TestAssertToolCalled(t *testing.T) {
	c := &CapturedRun{ToolCalls: []CapturedToolCall{{Name: "get_weather"}, {Name: "send_email"}}}

	// Single tool via expected
	r := RunAssertions([]Assertion{{Type: "tool_called", Expected: "get_weather"}}, c)
	if !r[0].Passed {
		t.Error("should find tool")
	}

	// Multiple tools via tools field
	r = RunAssertions([]Assertion{{Type: "tool_called", Tools: []string{"get_weather", "send_email"}}}, c)
	if len(r) != 2 {
		t.Fatalf("expected 2 results, got %d", len(r))
	}
	if !r[0].Passed || !r[1].Passed {
		t.Error("both tools should be found")
	}

	// Missing tool
	r = RunAssertions([]Assertion{{Type: "tool_called", Tools: []string{"missing"}}}, c)
	if r[0].Passed {
		t.Error("missing tool should fail")
	}
}

func TestAssertToolNotCalled(t *testing.T) {
	c := &CapturedRun{ToolCalls: []CapturedToolCall{{Name: "send_email"}}}

	r := RunAssertions([]Assertion{{Type: "tool_not_called", Expected: "get_weather"}}, c)
	if !r[0].Passed {
		t.Error("should pass when tool not called")
	}

	r = RunAssertions([]Assertion{{Type: "tool_not_called", Expected: "send_email"}}, c)
	if r[0].Passed {
		t.Error("should fail when tool was called")
	}
}

func TestAssertToolSequenceIncludes(t *testing.T) {
	c := &CapturedRun{ToolCalls: []CapturedToolCall{{Name: "search"}, {Name: "fetch"}, {Name: "summarize"}}}

	tests := []struct {
		name   string
		tools  []string
		passed bool
	}{
		{"exact order", []string{"search", "fetch", "summarize"}, true},
		{"subsequence", []string{"search", "summarize"}, true},
		{"wrong order", []string{"summarize", "search"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RunAssertions([]Assertion{{Type: "tool_sequence_includes", Tools: tt.tools}}, c)
			if r[0].Passed != tt.passed {
				t.Errorf("Passed = %v, want %v: %s", r[0].Passed, tt.passed, r[0].Message)
			}
		})
	}
}

func TestAssertMinToolCalls(t *testing.T) {
	c := &CapturedRun{ToolCalls: []CapturedToolCall{{Name: "a"}, {Name: "b"}, {Name: "c"}}}

	r := RunAssertions([]Assertion{{Type: "min_tool_calls", Count: 2}}, c)
	if !r[0].Passed {
		t.Error("3 >= 2 should pass")
	}

	r = RunAssertions([]Assertion{{Type: "min_tool_calls", Count: 5}}, c)
	if r[0].Passed {
		t.Error("3 < 5 should fail")
	}
}

func TestAssertMaxToolCalls(t *testing.T) {
	c := &CapturedRun{ToolCalls: []CapturedToolCall{{Name: "a"}, {Name: "b"}, {Name: "c"}}}

	r := RunAssertions([]Assertion{{Type: "max_tool_calls", Count: 5}}, c)
	if !r[0].Passed {
		t.Error("3 <= 5 should pass")
	}

	r = RunAssertions([]Assertion{{Type: "max_tool_calls", Count: 2}}, c)
	if r[0].Passed {
		t.Error("3 > 2 should fail")
	}
}

// --- Assertions: output ---

func TestAssertOutputContains(t *testing.T) {
	c := &CapturedRun{FinalOutput: "The weather in Tokyo is sunny and 25°C"}

	// Single value via expected
	r := RunAssertions([]Assertion{{Type: "output_contains", Expected: "sunny"}}, c)
	if !r[0].Passed {
		t.Error("should find substring")
	}

	// Multiple values via values field
	r = RunAssertions([]Assertion{{Type: "output_contains", Values: []string{"Tokyo", "sunny"}}}, c)
	if len(r) != 2 {
		t.Fatalf("expected 2 results, got %d", len(r))
	}
	if !r[0].Passed || !r[1].Passed {
		t.Error("both values should be found")
	}

	// Case insensitive
	r = RunAssertions([]Assertion{{Type: "output_contains", Expected: "SUNNY"}}, c)
	if !r[0].Passed {
		t.Error("should be case insensitive")
	}
}

func TestAssertOutputNotContains(t *testing.T) {
	c := &CapturedRun{FinalOutput: "RECOMMENDATION: PASS on this prop"}

	r := RunAssertions([]Assertion{{Type: "output_not_contains", Values: []string{"RECOMMENDATION: VALUE"}}}, c)
	if !r[0].Passed {
		t.Error("should pass when exact string not found")
	}

	r = RunAssertions([]Assertion{{Type: "output_not_contains", Values: []string{"RECOMMENDATION"}}}, c)
	if r[0].Passed {
		t.Error("should fail when string found")
	}
}

func TestAssertOutputMatches(t *testing.T) {
	c := &CapturedRun{FinalOutput: "Temperature: 25°C, Humidity: 60%"}

	// Via pattern field
	r := RunAssertions([]Assertion{{Type: "output_matches", Pattern: `\d+°C`}}, c)
	if !r[0].Passed {
		t.Error("should match regex via pattern field")
	}

	// Via expected field (backward compat)
	r = RunAssertions([]Assertion{{Type: "output_matches", Expected: `\d+°F`}}, c)
	if r[0].Passed {
		t.Error("should not match")
	}
}

// --- Assertions: budget ---

func TestAssertMaxTurns(t *testing.T) {
	c := &CapturedRun{TurnCount: 3}

	r := RunAssertions([]Assertion{{Type: "max_turns", Count: 5}}, c)
	if !r[0].Passed {
		t.Error("3 <= 5 should pass")
	}

	// Also works via expected (backward compat)
	r = RunAssertions([]Assertion{{Type: "max_turns", Expected: 2}}, c)
	if r[0].Passed {
		t.Error("3 > 2 should fail")
	}
}

func TestAssertMaxTokensUsed(t *testing.T) {
	c := &CapturedRun{TotalTokens: 500}

	r := RunAssertions([]Assertion{{Type: "max_tokens_used", Tokens: 1000}}, c)
	if !r[0].Passed {
		t.Error("500 <= 1000 should pass")
	}

	r = RunAssertions([]Assertion{{Type: "max_tokens_used", Tokens: 100}}, c)
	if r[0].Passed {
		t.Error("500 > 100 should fail")
	}
}

func TestAssertMaxCostUSD(t *testing.T) {
	c := &CapturedRun{TotalCost: 0.03}

	r := RunAssertions([]Assertion{{Type: "max_cost_usd", Amount: 0.05}}, c)
	if !r[0].Passed {
		t.Error("$0.03 <= $0.05 should pass")
	}

	r = RunAssertions([]Assertion{{Type: "max_cost_usd", Amount: 0.01}}, c)
	if r[0].Passed {
		t.Error("$0.03 > $0.01 should fail")
	}
}

func TestAssertMaxLatencySeconds(t *testing.T) {
	c := &CapturedRun{Duration: 5 * time.Second}

	r := RunAssertions([]Assertion{{Type: "max_latency_seconds", Seconds: 10}}, c)
	if !r[0].Passed {
		t.Error("5s <= 10s should pass")
	}

	r = RunAssertions([]Assertion{{Type: "max_latency_seconds", Seconds: 3}}, c)
	if r[0].Passed {
		t.Error("5s > 3s should fail")
	}
}

// --- Assertions: quality ---

func TestAssertNoToolErrors(t *testing.T) {
	c := &CapturedRun{ToolCalls: []CapturedToolCall{{Name: "a", Result: "ok"}, {Name: "b", Result: "ok"}}}
	r := RunAssertions([]Assertion{{Type: "no_tool_errors"}}, c)
	if !r[0].Passed {
		t.Error("no errors should pass")
	}

	c = &CapturedRun{ToolCalls: []CapturedToolCall{{Name: "a"}, {Name: "b", Error: "timeout"}}}
	r = RunAssertions([]Assertion{{Type: "no_tool_errors"}}, c)
	if r[0].Passed {
		t.Error("should fail with tool error")
	}
}

func TestAssertNoHallucinatedStats(t *testing.T) {
	c := &CapturedRun{
		FinalOutput: "He averaged 3.5 SOG over 10 games",
		ToolCalls:   []CapturedToolCall{{Name: "stats", Result: `{"avg_sog": 3.5, "games": 10}`}},
	}
	r := RunAssertions([]Assertion{{Type: "no_hallucinated_stats"}}, c)
	if !r[0].Passed {
		t.Errorf("grounded stats should pass: %s", r[0].Message)
	}
}

func TestAssertCitesDataSource(t *testing.T) {
	c := &CapturedRun{
		FinalOutput: "Based on the stats data, the player scored 3 goals",
		ToolCalls:   []CapturedToolCall{{Name: "stats", Result: `{"goals": 3}`}},
	}
	r := RunAssertions([]Assertion{{Type: "cites_data_source"}}, c)
	if !r[0].Passed {
		t.Errorf("should pass when output references tool name: %s", r[0].Message)
	}
}

func TestAssertUnknownType(t *testing.T) {
	c := &CapturedRun{}
	r := RunAssertions([]Assertion{{Type: "nonexistent_assertion"}}, c)
	if r[0].Passed {
		t.Error("unknown assertion should fail")
	}
}

// --- Old-style YAML compatibility ---

func TestOldStyleYAMLParsing(t *testing.T) {
	yaml := `
name: prop-scanning
description: "Tests the agent's ability to gather data"
agent: nhl-prop-scanner
settings:
  model: claude-sonnet-4-6
  temperature: 0.0
cases:
  - name: "Specific player SOG prop analysis"
    input: "Analyze Connor McDavid SOG O3.5 for tonight's game"
    tags: [happy-path, sog]
    assertions:
      - type: tool_called
        tools: [nhl_get_schedule]
      - type: tool_called
        tools: [nhl_get_team_roster]
      - type: output_contains
        values: ["McDavid", "SOG"]
      - type: max_cost_usd
        amount: 0.10
  - name: "Cost guard"
    input: "Quick take on McDavid SOG O3.5"
    tags: [performance]
    assertions:
      - type: max_tool_calls
        count: 10
      - type: max_latency_seconds
        seconds: 60
`
	dir := t.TempDir()
	path := filepath.Join(dir, "old-style.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}

	if suite.Name != "prop-scanning" {
		t.Errorf("Name = %q", suite.Name)
	}
	if suite.Settings.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q", suite.Settings.Model)
	}
	if len(suite.Scenarios) != 2 {
		t.Fatalf("Scenarios = %d, want 2", len(suite.Scenarios))
	}

	// First case: input converted to messages
	s0 := suite.Scenarios[0]
	if len(s0.Messages) != 1 {
		t.Fatalf("s0.Messages = %d, want 1", len(s0.Messages))
	}
	if s0.Messages[0].Content != "Analyze Connor McDavid SOG O3.5 for tonight's game" {
		t.Error("input not converted to message")
	}
	if len(s0.Tags) != 2 {
		t.Errorf("s0.Tags = %v", s0.Tags)
	}
	if len(s0.Assertions) != 4 {
		t.Errorf("s0.Assertions = %d, want 4", len(s0.Assertions))
	}
	// tool_called with tools list
	if len(s0.Assertions[0].Tools) != 1 || s0.Assertions[0].Tools[0] != "nhl_get_schedule" {
		t.Errorf("assertion 0 tools = %v", s0.Assertions[0].Tools)
	}
	// output_contains with values list
	if len(s0.Assertions[2].Values) != 2 {
		t.Errorf("assertion 2 values = %v", s0.Assertions[2].Values)
	}
	// max_cost_usd with amount field
	if s0.Assertions[3].Amount != 0.10 {
		t.Errorf("assertion 3 amount = %f", s0.Assertions[3].Amount)
	}

	// Second case: named fields
	s1 := suite.Scenarios[1]
	if s1.Assertions[0].Count != 10 {
		t.Errorf("max_tool_calls count = %d", s1.Assertions[0].Count)
	}
	if s1.Assertions[1].Seconds != 60 {
		t.Errorf("max_latency_seconds seconds = %f", s1.Assertions[1].Seconds)
	}
}

// --- Mock injection ---

func TestMockInjectorAlwaysFail(t *testing.T) {
	inner := &fakeCaller{result: "ok"}
	injector := NewMockInjector(inner, []MockError{
		{Tool: "broken_tool", Error: "always broken"},
	})

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _, err := injector.CallTool(ctx, "broken_tool", nil)
		if err == nil {
			t.Fatalf("call %d: expected error", i)
		}
	}

	result, _, err := injector.CallTool(ctx, "good_tool", nil)
	if err != nil {
		t.Fatalf("good_tool: %v", err)
	}
	if result == nil {
		t.Error("expected result from good_tool")
	}
}

func TestMockInjectorCountedFail(t *testing.T) {
	inner := &fakeCaller{result: "success"}
	injector := NewMockInjector(inner, []MockError{
		{Tool: "flaky_tool", Error: "temporary error", Count: 2},
	})

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_, _, err := injector.CallTool(ctx, "flaky_tool", nil)
		if err == nil {
			t.Fatalf("call %d: expected error", i)
		}
	}

	result, _, err := injector.CallTool(ctx, "flaky_tool", nil)
	if err != nil {
		t.Fatalf("call 3: %v", err)
	}
	if result == nil {
		t.Error("expected result on 3rd call")
	}
}

func TestMockInjectorReset(t *testing.T) {
	inner := &fakeCaller{result: "ok"}
	injector := NewMockInjector(inner, []MockError{
		{Tool: "tool", Error: "fail", Count: 1},
	})

	ctx := context.Background()

	_, _, err := injector.CallTool(ctx, "tool", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	_, _, err = injector.CallTool(ctx, "tool", nil)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}

	injector.Reset()
	_, _, err = injector.CallTool(ctx, "tool", nil)
	if err == nil {
		t.Fatal("expected error after reset")
	}
}

// --- Aggregation ---

func TestAggregateMultipleRuns(t *testing.T) {
	results := []*ScenarioResult{
		{ScenarioName: "test", Passed: true, Assertions: []AssertionResult{{Type: "tool_called", Passed: true}, {Type: "max_turns", Passed: true}}},
		{ScenarioName: "test", Passed: false, Assertions: []AssertionResult{{Type: "tool_called", Passed: true}, {Type: "max_turns", Passed: false, Message: "too many"}}},
		{ScenarioName: "test", Passed: true, Assertions: []AssertionResult{{Type: "tool_called", Passed: true}, {Type: "max_turns", Passed: true}}},
	}

	agg := Aggregate("test", results)
	if agg.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", agg.TotalRuns)
	}

	expectedRate := 2.0 / 3.0
	if agg.PassRate < expectedRate-0.01 || agg.PassRate > expectedRate+0.01 {
		t.Errorf("PassRate = %f, want ~%f", agg.PassRate, expectedRate)
	}
}

// --- Report generation ---

func TestFormatTextReport(t *testing.T) {
	result := &RunResult{
		SuiteName: "basic",
		AgentName: "assistant",
		Status:    "passed",
		Passed:    1,
		Duration:  2 * time.Second,
		Scenarios: []*ScenarioResult{{ScenarioName: "greeting", Passed: true, Duration: 500 * time.Millisecond, Assertions: []AssertionResult{{Type: "output_contains", Passed: true, Message: "ok"}}}},
	}

	var buf bytes.Buffer
	FormatTextReport(&buf, result)
	if !bytes.Contains(buf.Bytes(), []byte("PASSED")) {
		t.Error("report should contain PASSED")
	}
}

func TestWriteJSONReport(t *testing.T) {
	result := &RunResult{SuiteName: "test", AgentName: "bot", Status: "passed", Passed: 1}
	var buf bytes.Buffer
	if err := WriteJSONReport(&buf, result); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"suite_name"`)) {
		t.Error("JSON should contain suite_name")
	}
}

func TestWriteHTMLReport(t *testing.T) {
	result := &RunResult{
		SuiteName: "test", AgentName: "bot", Status: "failed", Passed: 1, Failed: 1, Duration: 3 * time.Second,
		Scenarios: []*ScenarioResult{{ScenarioName: "s1", Passed: true}, {ScenarioName: "s2", Passed: false, Error: "timeout"}},
	}
	path := filepath.Join(t.TempDir(), "report.html")
	if err := WriteHTMLReport(path, result); err != nil {
		t.Fatalf("WriteHTMLReport: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !bytes.Contains(data, []byte("Agent Eval Report")) {
		t.Error("HTML should contain title")
	}
}

// --- Storage integration ---

func TestEngineStoreIntegration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "eval.db")
	store, err := storage.NewEvalStore(dbPath)
	if err != nil {
		t.Fatalf("NewEvalStore: %v", err)
	}
	defer store.Close()

	engine := New(store)
	if engine == nil {
		t.Fatal("New returned nil")
	}

	runs, err := store.ListRuns("agent", 10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

// --- helpers ---

type fakeCaller struct {
	result string
}

func (f *fakeCaller) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: f.result}},
	}, 10 * time.Millisecond, nil
}
