package reporting

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

func TestFormatSARIF_Basic(t *testing.T) {
	violations := []session.PolicyViolation{
		{
			Rule:     "secrets.block_patterns",
			Severity: "error",
			Message:  "AWS key detected",
			File:     "config.go",
			Line:     42,
		},
		{
			Rule:     "filesystem.read_only",
			Severity: "error",
			Message:  "Write to .env",
			File:     ".env",
		},
	}

	b, err := FormatSARIF(violations, "1.2.3")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	// Verify it's valid JSON.
	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	output := string(b)
	if !strings.Contains(output, `"version": "2.1.0"`) {
		t.Error("expected SARIF version 2.1.0")
	}
	if !strings.Contains(output, `"version": "1.2.3"`) {
		t.Error("expected driver version to reflect the build version passed in")
	}
	if !strings.Contains(output, `"trovery-vigil"`) {
		t.Error("expected tool name")
	}
	if !strings.Contains(output, "config.go") {
		t.Error("expected file location")
	}
	if !strings.Contains(output, `"startLine": 42`) {
		t.Error("expected line number")
	}
}

func TestFormatSARIF_Empty(t *testing.T) {
	b, err := FormatSARIF(nil, "1.2.3")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(string(b), `"results": []`) && !strings.Contains(string(b), `"results": null`) {
		t.Error("expected empty results")
	}
}

func TestFormatViolationsJSON(t *testing.T) {
	violations := []session.PolicyViolation{
		{Rule: "test", Severity: "warning", Message: "test msg"},
	}
	b, err := FormatViolationsJSON(violations)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(string(b), `"rule": "test"`) {
		t.Error("expected rule in JSON output")
	}
}
