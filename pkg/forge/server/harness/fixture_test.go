package harness

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFixtures(t *testing.T) {
	dir := t.TempDir()

	yaml := `tests:
  - name: hello returns greeting
    tool: hello
    input:
      name: "World"
    expect:
      status: success
      content_contains: "Hello, World!"
  - name: hello with latency check
    tool: hello
    input:
      name: "Test"
    expect:
      status: success
      max_latency_ms: 5000
`
	if err := os.WriteFile(filepath.Join(dir, "test_tools.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	// Non-yaml file should be ignored
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644)

	suites, err := LoadFixtures(dir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}

	if len(suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(suites))
	}

	if len(suites[0].Tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(suites[0].Tests))
	}

	tc := suites[0].Tests[0]
	if tc.Name != "hello returns greeting" {
		t.Errorf("name = %q", tc.Name)
	}
	if tc.Tool != "hello" {
		t.Errorf("tool = %q", tc.Tool)
	}
	if tc.Expect.Status != "success" {
		t.Errorf("status = %q", tc.Expect.Status)
	}
	if tc.Expect.ContentContains != "Hello, World!" {
		t.Errorf("content_contains = %q", tc.Expect.ContentContains)
	}
}

func TestLoadFixturesMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"a.yaml", "b.yml"} {
		yaml := `tests:
  - name: test
    tool: hello
    input: {}
    expect:
      status: success
`
		os.WriteFile(filepath.Join(dir, name), []byte(yaml), 0o644)
	}

	suites, err := LoadFixtures(dir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}

	if len(suites) != 2 {
		t.Errorf("expected 2 suites, got %d", len(suites))
	}
}

func TestLoadFixturesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	suites, err := LoadFixtures(dir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}

	if len(suites) != 0 {
		t.Errorf("expected 0 suites, got %d", len(suites))
	}
}

func TestLoadFixturesMissingDir(t *testing.T) {
	_, err := LoadFixtures("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing dir")
	}
}

func TestCheckExpectationSuccess(t *testing.T) {
	output := &ToolCallOutput{
		Text:     "Hello, World! Welcome.",
		IsError:  false,
		Duration: 50 * time.Millisecond,
	}

	err := CheckExpectation(Expectation{
		Status:          "success",
		ContentContains: "Hello, World!",
		MaxLatencyMs:    1000,
	}, output)

	if err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckExpectationStatusFail(t *testing.T) {
	output := &ToolCallOutput{IsError: true}

	err := CheckExpectation(Expectation{Status: "success"}, output)
	if err == nil {
		t.Error("expected failure for error status")
	}
}

func TestCheckExpectationStatusError(t *testing.T) {
	output := &ToolCallOutput{IsError: false}

	err := CheckExpectation(Expectation{Status: "error"}, output)
	if err == nil {
		t.Error("expected failure when expecting error but got success")
	}
}

func TestCheckExpectationContentMismatch(t *testing.T) {
	output := &ToolCallOutput{Text: "Goodbye"}

	err := CheckExpectation(Expectation{ContentContains: "Hello"}, output)
	if err == nil {
		t.Error("expected failure for content mismatch")
	}
}

func TestCheckExpectationLatencyExceeded(t *testing.T) {
	output := &ToolCallOutput{Duration: 2 * time.Second}

	err := CheckExpectation(Expectation{MaxLatencyMs: 500}, output)
	if err == nil {
		t.Error("expected failure for exceeded latency")
	}
}

func TestCheckExpectationLatencyOk(t *testing.T) {
	output := &ToolCallOutput{Duration: 100 * time.Millisecond}

	err := CheckExpectation(Expectation{MaxLatencyMs: 500}, output)
	if err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckExpectationEmpty(t *testing.T) {
	// No expectations = always pass
	output := &ToolCallOutput{Text: "anything"}
	err := CheckExpectation(Expectation{}, output)
	if err != nil {
		t.Errorf("expected pass for empty expectation, got: %v", err)
	}
}
