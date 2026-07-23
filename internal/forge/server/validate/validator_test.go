package validate

import (
	"encoding/json"
	"testing"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/protocol"
)

func TestValidateAllRulesRun(t *testing.T) {
	v := NewValidator()
	tools := []protocol.Tool{wellFormedTool()}

	report := v.Validate(tools)
	if report.Summary.TotalTools != 1 {
		t.Errorf("TotalTools = %d, want 1", report.Summary.TotalTools)
	}
	if report.Summary.TotalRules == 0 {
		t.Error("TotalRules should be > 0")
	}
}

func TestValidateCategoryFilter(t *testing.T) {
	v := NewValidator()
	tools := []protocol.Tool{{
		Name:        "BadName",
		Description: "A tool",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}}

	// Only naming rules
	report := v.ValidateCategory(tools, CategoryNaming)

	for _, viol := range report.Violations {
		if viol.Category != CategoryNaming {
			t.Errorf("expected only naming violations, got category %q", viol.Category)
		}
	}

	// Should have naming-001 error (not snake_case)
	found := false
	for _, viol := range report.Violations {
		if viol.RuleID == "naming-001" {
			found = true
		}
	}
	if !found {
		t.Error("expected naming-001 violation for non-snake_case name")
	}
}

func TestValidateCleanTool(t *testing.T) {
	v := NewValidator()
	tools := []protocol.Tool{wellFormedTool()}

	report := v.Validate(tools)
	if report.Summary.Errors != 0 {
		t.Errorf("expected 0 errors for well-formed tool, got %d", report.Summary.Errors)
		for _, viol := range report.Violations {
			if viol.Severity == SeverityError {
				t.Logf("  %s: %s", viol.RuleID, viol.Message)
			}
		}
	}
}

func TestValidateMultipleTools(t *testing.T) {
	v := NewValidator()
	tools := []protocol.Tool{
		wellFormedTool(),
		{Name: "Bad Name!", Description: ""},
	}

	report := v.Validate(tools)
	if report.Summary.TotalTools != 2 {
		t.Errorf("TotalTools = %d, want 2", report.Summary.TotalTools)
	}
	if report.Summary.Errors == 0 {
		t.Error("expected errors for malformed tool")
	}
}

func TestReportSummaryCounts(t *testing.T) {
	v := NewValidator()
	tools := []protocol.Tool{{
		Name:        "BadName",
		Description: "",
		InputSchema: nil,
	}}

	report := v.Validate(tools)

	total := report.Summary.Errors + report.Summary.Warnings + report.Summary.Infos
	if total != len(report.Violations) {
		t.Errorf("summary counts (%d) don't match violation count (%d)", total, len(report.Violations))
	}

	catTotal := 0
	for _, count := range report.Summary.ByCategory {
		catTotal += count
	}
	if catTotal != len(report.Violations) {
		t.Errorf("by-category total (%d) doesn't match violation count (%d)", catTotal, len(report.Violations))
	}
}

func TestFormatJSON(t *testing.T) {
	report := &Report{
		Violations: []Violation{{
			RuleID:   "naming-001",
			Category: CategoryNaming,
			Severity: SeverityError,
			ToolName: "BadName",
			Message:  "not snake_case",
		}},
		Summary: ReportSummary{
			TotalTools: 1,
			TotalRules: 1,
			Errors:     1,
			ByCategory: map[Category]int{CategoryNaming: 1},
		},
	}

	data, err := FormatJSON(report)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if _, ok := parsed["violations"]; !ok {
		t.Error("JSON output missing 'violations' key")
	}
	if _, ok := parsed["summary"]; !ok {
		t.Error("JSON output missing 'summary' key")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	report := &Report{
		Summary: ReportSummary{ByCategory: make(map[Category]int)},
	}

	text := FormatText(report)
	if text != "No violations found." {
		t.Errorf("expected 'No violations found.', got %q", text)
	}
}

func TestFormatTextWithViolations(t *testing.T) {
	report := &Report{
		Violations: []Violation{
			{RuleID: "naming-001", Category: CategoryNaming, Severity: SeverityError, ToolName: "test", Message: "bad name"},
			{RuleID: "schema-001", Category: CategorySchema, Severity: SeverityError, ToolName: "test", Message: "no schema"},
		},
		Summary: ReportSummary{
			TotalTools: 1,
			TotalRules: 2,
			Errors:     2,
			ByCategory: map[Category]int{CategoryNaming: 1, CategorySchema: 1},
		},
	}

	text := FormatText(report)
	if text == "No violations found." {
		t.Error("should not say 'No violations found' when there are violations")
	}
}

// wellFormedTool returns a tool definition that should pass all rules without errors.
func wellFormedTool() protocol.Tool {
	boolTrue := true
	return protocol.Tool{
		Name:        "get_weather",
		Description: "Returns the current weather for a given city.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"city": {"type": "string", "description": "The city name"}
			},
			"required": ["city"]
		}`),
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint: &boolTrue,
		},
	}
}
