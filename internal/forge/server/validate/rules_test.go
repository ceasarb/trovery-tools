package validate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"
)

// helper to run a single rule against a tool and return violations
func runRule(rule Rule, tool protocol.Tool) []Violation {
	return rule.Check(tool)
}

func findRule(rules []Rule, id string) Rule {
	for _, r := range rules {
		if r.ID == id {
			return r
		}
	}
	panic("rule not found: " + id)
}

// --- Naming Rules ---

func TestNaming001(t *testing.T) {
	rule := findRule(namingRules(), "naming-001")

	tests := []struct {
		name    string
		tool    string
		wantErr bool
	}{
		{"valid snake_case", "get_weather", false},
		{"single word", "hello", false},
		{"with numbers", "get_item_2", false},
		{"camelCase", "getWeather", true},
		{"PascalCase", "GetWeather", true},
		{"kebab-case", "get-weather", true},
		{"spaces", "get weather", true},
		{"leading underscore", "_get", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := runRule(rule, protocol.Tool{Name: tt.tool})
			if tt.wantErr && len(v) == 0 {
				t.Errorf("expected violation for %q", tt.tool)
			}
			if !tt.wantErr && len(v) > 0 {
				t.Errorf("unexpected violation for %q: %s", tt.tool, v[0].Message)
			}
		})
	}
}

func TestNaming002(t *testing.T) {
	rule := findRule(namingRules(), "naming-002")

	short := "a"
	long := strings.Repeat("a", 65)
	exact := strings.Repeat("a", 64)

	tests := []struct {
		name    string
		tool    string
		wantErr bool
	}{
		{"short name", short, false},
		{"exact 64", exact, false},
		{"too long", long, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := runRule(rule, protocol.Tool{Name: tt.tool})
			if tt.wantErr && len(v) == 0 {
				t.Error("expected violation")
			}
			if !tt.wantErr && len(v) > 0 {
				t.Errorf("unexpected violation: %s", v[0].Message)
			}
		})
	}
}

func TestNaming003(t *testing.T) {
	rule := findRule(namingRules(), "naming-003")

	v := runRule(rule, protocol.Tool{Name: "test", Description: ""})
	if len(v) == 0 {
		t.Error("expected violation for empty description")
	}

	v = runRule(rule, protocol.Tool{Name: "test", Description: "Does something"})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

func TestNaming004(t *testing.T) {
	rule := findRule(namingRules(), "naming-004")

	long := strings.Repeat("x", 1025)
	v := runRule(rule, protocol.Tool{Name: "test", Description: long})
	if len(v) == 0 {
		t.Error("expected violation for long description")
	}

	ok := strings.Repeat("x", 1024)
	v = runRule(rule, protocol.Tool{Name: "test", Description: ok})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

// --- Schema Rules ---

func TestSchema001(t *testing.T) {
	rule := findRule(schemaRules(), "schema-001")

	v := runRule(rule, protocol.Tool{Name: "test"})
	if len(v) == 0 {
		t.Error("expected violation for missing schema")
	}

	v = runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(`{"type":"object"}`)})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

func TestSchema002(t *testing.T) {
	rule := findRule(schemaRules(), "schema-002")

	v := runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(`{"type":"string"}`)})
	if len(v) == 0 {
		t.Error("expected violation for non-object schema type")
	}

	v = runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(`{"type":"object"}`)})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}

	// Missing schema should not trigger this rule
	v = runRule(rule, protocol.Tool{Name: "test"})
	if len(v) > 0 {
		t.Error("should not trigger for missing schema")
	}
}

func TestSchema003(t *testing.T) {
	rule := findRule(schemaRules(), "schema-003")

	// Required field not in properties
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name","age"]}`
	v := runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) == 0 {
		t.Error("expected violation for missing required field 'age'")
	}

	// All required fields present
	schema = `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	v = runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

func TestSchema004(t *testing.T) {
	rule := findRule(schemaRules(), "schema-004")

	// Property without description
	schema := `{"type":"object","properties":{"name":{"type":"string"}}}`
	v := runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) == 0 {
		t.Error("expected violation for property without description")
	}

	// Property with description
	schema = `{"type":"object","properties":{"name":{"type":"string","description":"The name"}}}`
	v = runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

// --- Annotation Rules ---

func TestAnnotations001(t *testing.T) {
	rule := findRule(annotationRules(), "annotations-001")

	v := runRule(rule, protocol.Tool{Name: "test"})
	if len(v) == 0 {
		t.Error("expected violation for missing annotations")
	}

	v = runRule(rule, protocol.Tool{Name: "test", Annotations: &protocol.ToolAnnotations{}})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

func TestAnnotations002(t *testing.T) {
	rule := findRule(annotationRules(), "annotations-002")

	// Read-like tool without annotations
	v := runRule(rule, protocol.Tool{Name: "get_users"})
	if len(v) == 0 {
		t.Error("expected violation for read-like tool without annotations")
	}

	// Read-like tool with annotations — no violation
	v = runRule(rule, protocol.Tool{Name: "get_users", Annotations: &protocol.ToolAnnotations{}})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}

	// Non-read tool without annotations — no violation from this rule
	v = runRule(rule, protocol.Tool{Name: "create_user"})
	if len(v) > 0 {
		t.Errorf("unexpected violation for non-read tool: %s", v[0].Message)
	}
}

func TestAnnotations003(t *testing.T) {
	rule := findRule(annotationRules(), "annotations-003")

	// Destructive tool without annotations
	v := runRule(rule, protocol.Tool{Name: "delete_user", Description: "Deletes a user"})
	if len(v) == 0 {
		t.Error("expected violation for destructive tool without annotations")
	}

	// Destructive tool with destructiveHint set
	boolTrue := true
	v = runRule(rule, protocol.Tool{
		Name:        "delete_user",
		Description: "Deletes a user",
		Annotations: &protocol.ToolAnnotations{DestructiveHint: &boolTrue},
	})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}

	// Non-destructive tool — no violation
	v = runRule(rule, protocol.Tool{Name: "create_user", Description: "Creates a user"})
	if len(v) > 0 {
		t.Errorf("unexpected violation for non-destructive tool: %s", v[0].Message)
	}
}

// --- Error Rules ---

func TestErrors001(t *testing.T) {
	rule := findRule(errorRules(), "errors-001")

	// Tool that may fail, no error docs
	v := runRule(rule, protocol.Tool{Name: "send_email", Description: "Sends an email"})
	if len(v) == 0 {
		t.Error("expected violation for tool that may fail without error docs")
	}

	// Tool that may fail, with error docs
	v = runRule(rule, protocol.Tool{Name: "send_email", Description: "Sends an email. Returns error if recipient is invalid"})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}

	// Tool that doesn't suggest failure
	v = runRule(rule, protocol.Tool{Name: "get_time", Description: "Gets the current time"})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

// --- Response Rules ---

func TestResponse001(t *testing.T) {
	rule := findRule(responseRules(), "response-001")

	// No return info
	v := runRule(rule, protocol.Tool{Name: "test", Description: "Does something"})
	if len(v) == 0 {
		t.Error("expected violation for description without return info")
	}

	// Has return info
	v = runRule(rule, protocol.Tool{Name: "test", Description: "Returns the weather data"})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}

	// Empty description — should not trigger (naming-003 covers that)
	v = runRule(rule, protocol.Tool{Name: "test", Description: ""})
	if len(v) > 0 {
		t.Error("should not trigger for empty description")
	}
}

// --- Pagination Rules ---

func TestPagination001(t *testing.T) {
	rule := findRule(paginationRules(), "pagination-001")

	// List tool without pagination
	v := runRule(rule, protocol.Tool{
		Name:        "list_users",
		Description: "Lists users",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"filter":{"type":"string"}}}`),
	})
	if len(v) == 0 {
		t.Error("expected violation for list tool without pagination")
	}

	// List tool with pagination
	v = runRule(rule, protocol.Tool{
		Name:        "list_users",
		Description: "Lists users",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"cursor":{"type":"string"},"limit":{"type":"integer"}}}`),
	})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}

	// Non-list tool
	v = runRule(rule, protocol.Tool{Name: "get_user", Description: "Gets a user"})
	if len(v) > 0 {
		t.Errorf("unexpected violation for non-list tool: %s", v[0].Message)
	}
}

// --- Security Rules ---

func TestSecurity001(t *testing.T) {
	rule := findRule(securityRules(), "security-001")

	// SQL parameter
	schema := `{"type":"object","properties":{"sql":{"type":"string"}}}`
	v := runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) == 0 {
		t.Error("expected violation for SQL parameter")
	}

	// Safe parameter
	schema = `{"type":"object","properties":{"name":{"type":"string"}}}`
	v = runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

func TestSecurity002(t *testing.T) {
	rule := findRule(securityRules(), "security-002")

	// File path without validation note
	schema := `{"type":"object","properties":{"file_path":{"type":"string"}}}`
	v := runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) == 0 {
		t.Error("expected violation for file path without validation note")
	}

	// File path with validation note
	schema = `{"type":"object","properties":{"file_path":{"type":"string","description":"Validated against allowlist"}}}`
	v = runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}

	// Non-path parameter
	schema = `{"type":"object","properties":{"name":{"type":"string"}}}`
	v = runRule(rule, protocol.Tool{Name: "test", InputSchema: json.RawMessage(schema)})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}

func TestSecurity003(t *testing.T) {
	rule := findRule(securityRules(), "security-003")

	// Description mentions password
	v := runRule(rule, protocol.Tool{Name: "test", Description: "Requires password authentication"})
	if len(v) == 0 {
		t.Error("expected violation for description mentioning password")
	}

	// Description mentions api_key
	v = runRule(rule, protocol.Tool{Name: "test", Description: "Pass your api_key"})
	if len(v) == 0 {
		t.Error("expected violation for description mentioning api_key")
	}

	// Clean description
	v = runRule(rule, protocol.Tool{Name: "test", Description: "Gets the weather"})
	if len(v) > 0 {
		t.Errorf("unexpected violation: %s", v[0].Message)
	}
}
