package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/storage"
)

// --- Suite YAML parsing tests ---

func TestLoadSuite(t *testing.T) {
	yaml := `
name: weather-eval
server: uv run python server.py
scenarios:
  - name: basic lookup
    tool: get_weather
    input:
      city: London
    assertions:
      - type: schema
        field: temperature
        expected: number
      - type: contains
        field: text
        expected: London
  - name: with setup
    setup:
      - tool: set_unit
        input:
          unit: celsius
    tool: get_weather
    input:
      city: Paris
    assertions:
      - type: range
        field: temperature
        expected:
          min: -50
          max: 60
`
	dir := t.TempDir()
	path := filepath.Join(dir, "weather.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}

	if suite.Name != "weather-eval" {
		t.Errorf("name = %q, want %q", suite.Name, "weather-eval")
	}
	if suite.Server != "uv run python server.py" {
		t.Errorf("server = %q, want %q", suite.Server, "uv run python server.py")
	}
	if len(suite.Scenarios) != 2 {
		t.Fatalf("scenarios count = %d, want 2", len(suite.Scenarios))
	}

	// First scenario
	s0 := suite.Scenarios[0]
	if s0.Name != "basic lookup" {
		t.Errorf("scenario[0].name = %q", s0.Name)
	}
	if s0.Tool != "get_weather" {
		t.Errorf("scenario[0].tool = %q", s0.Tool)
	}
	if len(s0.Setup) != 0 {
		t.Errorf("scenario[0].setup len = %d", len(s0.Setup))
	}
	if len(s0.Assertions) != 2 {
		t.Errorf("scenario[0].assertions len = %d", len(s0.Assertions))
	}

	// Second scenario with setup
	s1 := suite.Scenarios[1]
	if len(s1.Setup) != 1 {
		t.Fatalf("scenario[1].setup len = %d, want 1", len(s1.Setup))
	}
	if s1.Setup[0].Tool != "set_unit" {
		t.Errorf("scenario[1].setup[0].tool = %q", s1.Setup[0].Tool)
	}
}

func TestLoadSuite_DefaultName(t *testing.T) {
	yaml := `
server: python server.py
scenarios: []
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.eval.yaml")
	os.WriteFile(path, []byte(yaml), 0o644)

	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}

	if suite.Name != "test.eval.yaml" {
		t.Errorf("default name = %q, want %q", suite.Name, "test.eval.yaml")
	}
}

func TestDiscoverSuites(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "api.eval.yaml"), []byte("name: a\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "data.eval.yml"), []byte("name: b\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# hi\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("name: c\n"), 0o644)

	paths, err := DiscoverSuites(dir)
	if err != nil {
		t.Fatalf("DiscoverSuites: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("discovered %d suites, want 2: %v", len(paths), paths)
	}
}

// --- Assertion tests ---

func TestSchemaAssertion(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		expected string
		pass     bool
	}{
		{"string match", "hello", "string", true},
		{"number match", 42.0, "number", true},
		{"boolean match", true, "boolean", true},
		{"array match", []any{1, 2}, "array", true},
		{"object match", map[string]any{"a": 1}, "object", true},
		{"null match", nil, "null", true},
		{"type mismatch", "hello", "number", false},
		{"array vs object", []any{}, "object", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"val": tt.val}
			a := Assertion{Type: "schema", Field: "val", Expected: tt.expected}
			r := RunAssertion(a, data)
			if r.Passed != tt.pass {
				t.Errorf("passed = %v, want %v: %s", r.Passed, tt.pass, r.Message)
			}
		})
	}
}

func TestRangeAssertion(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		expected map[string]any
		pass     bool
	}{
		{"in range", 50.0, map[string]any{"min": 0, "max": 100}, true},
		{"at min", 0.0, map[string]any{"min": 0, "max": 100}, true},
		{"at max", 100.0, map[string]any{"min": 0, "max": 100}, true},
		{"below min", -1.0, map[string]any{"min": 0, "max": 100}, false},
		{"above max", 101.0, map[string]any{"min": 0, "max": 100}, false},
		{"min only", 5.0, map[string]any{"min": 0}, true},
		{"max only", 5.0, map[string]any{"max": 10}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"val": tt.val}
			a := Assertion{Type: "range", Field: "val", Expected: tt.expected}
			r := RunAssertion(a, data)
			if r.Passed != tt.pass {
				t.Errorf("passed = %v, want %v: %s", r.Passed, tt.pass, r.Message)
			}
		})
	}
}

func TestLengthAssertion(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		expected map[string]any
		pass     bool
	}{
		{"string in range", "hello", map[string]any{"min": 1, "max": 10}, true},
		{"string too short", "", map[string]any{"min": 1, "max": 10}, false},
		{"string too long", "hello world!!!!", map[string]any{"min": 1, "max": 5}, false},
		{"array in range", []any{1, 2, 3}, map[string]any{"min": 1, "max": 5}, true},
		{"array too short", []any{}, map[string]any{"min": 1}, false},
		{"min only pass", "hi", map[string]any{"min": 1}, true},
		{"max only pass", "hi", map[string]any{"max": 10}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"val": tt.val}
			a := Assertion{Type: "length", Field: "val", Expected: tt.expected}
			r := RunAssertion(a, data)
			if r.Passed != tt.pass {
				t.Errorf("passed = %v, want %v: %s", r.Passed, tt.pass, r.Message)
			}
		})
	}
}

func TestContainsAssertion(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		expected any
		pass     bool
	}{
		{"string contains", "hello world", "world", true},
		{"string missing", "hello world", "foo", false},
		{"array contains", []any{"a", "b", "c"}, "b", true},
		{"array missing", []any{"a", "b"}, "z", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"val": tt.val}
			a := Assertion{Type: "contains", Field: "val", Expected: tt.expected}
			r := RunAssertion(a, data)
			if r.Passed != tt.pass {
				t.Errorf("passed = %v, want %v: %s", r.Passed, tt.pass, r.Message)
			}
		})
	}
}

func TestGoldenFileAssertion(t *testing.T) {
	dir := t.TempDir()

	// Write golden file
	golden := map[string]any{"name": "test", "count": float64(42)}
	goldenData, _ := json.Marshal(golden)
	goldenPath := filepath.Join(dir, "golden.json")
	os.WriteFile(goldenPath, goldenData, 0o644)

	// Matching value
	t.Run("match", func(t *testing.T) {
		data := map[string]any{"result": map[string]any{"name": "test", "count": float64(42)}}
		a := Assertion{Type: "golden_file", Field: "result", Expected: goldenPath}
		r := RunAssertion(a, data)
		if !r.Passed {
			t.Errorf("should pass: %s", r.Message)
		}
	})

	// Non-matching value
	t.Run("mismatch", func(t *testing.T) {
		data := map[string]any{"result": map[string]any{"name": "different"}}
		a := Assertion{Type: "golden_file", Field: "result", Expected: goldenPath}
		r := RunAssertion(a, data)
		if r.Passed {
			t.Errorf("should fail")
		}
	})

	// Missing golden file
	t.Run("missing file", func(t *testing.T) {
		data := map[string]any{"result": "anything"}
		a := Assertion{Type: "golden_file", Field: "result", Expected: "/nonexistent/golden.json"}
		r := RunAssertion(a, data)
		if r.Passed {
			t.Errorf("should fail for missing golden file")
		}
	})
}

func TestFieldResolution(t *testing.T) {
	data := map[string]any{
		"top": "value",
		"nested": map[string]any{
			"deep": map[string]any{
				"val": 42.0,
			},
		},
	}

	// Root field
	t.Run("root field", func(t *testing.T) {
		a := Assertion{Type: "schema", Field: "top", Expected: "string"}
		r := RunAssertion(a, data)
		if !r.Passed {
			t.Errorf("root field failed: %s", r.Message)
		}
	})

	// Nested field
	t.Run("nested field", func(t *testing.T) {
		a := Assertion{Type: "schema", Field: "nested.deep.val", Expected: "number"}
		r := RunAssertion(a, data)
		if !r.Passed {
			t.Errorf("nested field failed: %s", r.Message)
		}
	})

	// Missing field
	t.Run("missing field", func(t *testing.T) {
		a := Assertion{Type: "schema", Field: "nonexistent", Expected: "string"}
		r := RunAssertion(a, data)
		if r.Passed {
			t.Errorf("should fail for missing field")
		}
	})

	// Whole object
	t.Run("whole object", func(t *testing.T) {
		a := Assertion{Type: "schema", Field: ".", Expected: "object"}
		r := RunAssertion(a, data)
		if !r.Passed {
			t.Errorf("whole object failed: %s", r.Message)
		}
	})
}

func TestUnknownAssertionType(t *testing.T) {
	data := map[string]any{"val": "test"}
	a := Assertion{Type: "bogus", Field: "val", Expected: "anything"}
	r := RunAssertion(a, data)
	if r.Passed {
		t.Errorf("unknown assertion type should fail")
	}
	if !strings.Contains(r.Message, "unknown assertion type") {
		t.Errorf("message should mention unknown type: %s", r.Message)
	}
}

// --- Baseline / regression detection tests ---

func TestDetectRegressions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "eval.db")

	store, err := storage.NewEvalStore(dbPath)
	if err != nil {
		t.Fatalf("NewEvalStore: %v", err)
	}
	defer store.Close()

	suiteName := "test-suite"

	// Set baseline with "passed" status
	baselineJSON, _ := json.Marshal(map[string]any{"status": "passed"})
	store.SetBaseline(&storage.EvalBaseline{
		Source:       "server",
		TargetName:   suiteName,
		SuiteName:    suiteName,
		ScenarioName: "scenario-a",
		BaselineJSON: string(baselineJSON),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	// Scenario that now fails = regression
	results := []ScenarioResult{
		{Name: "scenario-a", Status: "failed"},
		{Name: "scenario-b", Status: "passed"}, // no baseline, so no regression
	}

	regressions, err := DetectRegressions(store, suiteName, results)
	if err != nil {
		t.Fatalf("DetectRegressions: %v", err)
	}

	if len(regressions) != 1 {
		t.Fatalf("regressions = %d, want 1", len(regressions))
	}
	if regressions[0].ScenarioName != "scenario-a" {
		t.Errorf("regression scenario = %q", regressions[0].ScenarioName)
	}
	if regressions[0].PreviousStatus != "passed" {
		t.Errorf("previous status = %q", regressions[0].PreviousStatus)
	}
	if regressions[0].CurrentStatus != "failed" {
		t.Errorf("current status = %q", regressions[0].CurrentStatus)
	}
}

func TestDetectRegressions_NoneWhenStillPassing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "eval.db")

	store, err := storage.NewEvalStore(dbPath)
	if err != nil {
		t.Fatalf("NewEvalStore: %v", err)
	}
	defer store.Close()

	suiteName := "stable-suite"

	baselineJSON, _ := json.Marshal(map[string]any{"status": "passed"})
	store.SetBaseline(&storage.EvalBaseline{
		Source:       "server",
		TargetName:   suiteName,
		SuiteName:    suiteName,
		ScenarioName: "scenario-a",
		BaselineJSON: string(baselineJSON),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	results := []ScenarioResult{
		{Name: "scenario-a", Status: "passed"},
	}

	regressions, err := DetectRegressions(store, suiteName, results)
	if err != nil {
		t.Fatalf("DetectRegressions: %v", err)
	}
	if len(regressions) != 0 {
		t.Errorf("expected no regressions, got %d", len(regressions))
	}
}

func TestUpdateBaselines(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "eval.db")

	store, err := storage.NewEvalStore(dbPath)
	if err != nil {
		t.Fatalf("NewEvalStore: %v", err)
	}
	defer store.Close()

	suiteName := "test-suite"
	results := []ScenarioResult{
		{
			Name:   "scenario-x",
			Status: "passed",
			Assertions: []AssertionResult{
				{Type: "schema", Field: "val", Passed: true, Message: "type is string"},
			},
		},
	}

	if err := UpdateBaselines(store, suiteName, results); err != nil {
		t.Fatalf("UpdateBaselines: %v", err)
	}

	baseline, err := store.GetBaseline("server", suiteName, suiteName, "scenario-x")
	if err != nil {
		t.Fatalf("GetBaseline: %v", err)
	}
	if baseline == nil {
		t.Fatal("baseline is nil after update")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(baseline.BaselineJSON), &parsed); err != nil {
		t.Fatalf("unmarshal baseline: %v", err)
	}
	if parsed["status"] != "passed" {
		t.Errorf("baseline status = %v", parsed["status"])
	}
}

// --- Golden file / snapshot tests ---

func TestUpdateSnapshots(t *testing.T) {
	dir := t.TempDir()

	suite := &Suite{
		Name: "snap-suite",
		Scenarios: []Scenario{
			{
				Name: "snap-test",
				Assertions: []Assertion{
					{Type: "golden_file", Field: "result", Expected: filepath.Join(dir, "golden.json")},
				},
			},
		},
	}

	resultData := map[string]any{
		"snap-test": map[string]any{"name": "snapshot", "value": 123},
	}

	if err := UpdateSnapshots(suite, resultData, dir); err != nil {
		t.Fatalf("UpdateSnapshots: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	if parsed["name"] != "snapshot" {
		t.Errorf("golden name = %v", parsed["name"])
	}
}

func TestJsonEqual(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{"same", `{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		{"different values", `{"a":1}`, `{"a":2}`, false},
		{"different keys", `{"a":1}`, `{"b":1}`, false},
		{"arrays", `[1,2,3]`, `[1,2,3]`, true},
		{"arrays different order", `[1,2,3]`, `[3,2,1]`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonEqual([]byte(tt.a), []byte(tt.b)); got != tt.equal {
				t.Errorf("jsonEqual = %v, want %v", got, tt.equal)
			}
		})
	}
}

// --- Report generation test ---

func TestGenerateReport(t *testing.T) {
	result := &RunResult{
		RunID:     "test-run-123",
		SuiteName: "Test Suite",
		Status:    "failed",
		Total:     3,
		Passed:    2,
		Failed:    1,
		Scenarios: []ScenarioResult{
			{Name: "passing test", Status: "passed", Assertions: []AssertionResult{
				{Type: "schema", Field: "val", Passed: true, Message: "type is string"},
			}},
			{Name: "failing test", Status: "failed", Assertions: []AssertionResult{
				{Type: "range", Field: "count", Passed: false, Message: "101 is outside [0, 100]"},
			}},
			{Name: "error test", Status: "error", Error: "tool call failed"},
		},
	}

	regressions := []Regression{
		{ScenarioName: "failing test", PreviousStatus: "passed", CurrentStatus: "failed"},
	}

	html, err := GenerateReport(result, regressions)
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}

	// Verify it's valid HTML with expected content
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE")
	}
	if !strings.Contains(html, "Test Suite") {
		t.Error("missing suite name")
	}
	if !strings.Contains(html, "passing test") {
		t.Error("missing passing scenario")
	}
	if !strings.Contains(html, "failing test") {
		t.Error("missing failing scenario")
	}
	if !strings.Contains(html, "Regressions Detected") {
		t.Error("missing regressions section")
	}
	if !strings.Contains(html, "</html>") {
		t.Error("missing closing html tag")
	}
}

func TestGenerateReport_NoRegressions(t *testing.T) {
	result := &RunResult{
		RunID:     "clean-run",
		SuiteName: "Clean Suite",
		Status:    "passed",
		Total:     1,
		Passed:    1,
		Scenarios: []ScenarioResult{
			{Name: "test", Status: "passed"},
		},
	}

	html, err := GenerateReport(result, nil)
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}

	if strings.Contains(html, "Regressions Detected") {
		t.Error("should not contain regressions section when there are none")
	}
}
