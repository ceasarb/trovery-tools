package orchestrator

import (
	"testing"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

func TestValidateHandoffNoSchema(t *testing.T) {
	v := ValidateHandoff("anything", nil)
	if !v.Valid {
		t.Fatal("nil schema should always be valid")
	}
}

func TestValidateHandoffValidJSON(t *testing.T) {
	schema := &agentcfg.HandoffSchema{
		Type: "object",
		Properties: map[string]agentcfg.SchemaProperty{
			"summary": {Type: "string"},
			"score":   {Type: "number"},
		},
		Required: []string{"summary"},
		Strict:   true,
	}

	output := `{"summary": "looks good", "score": 0.95}`
	v := ValidateHandoff(output, schema)
	if !v.Valid {
		t.Fatalf("expected valid, got errors: %v", v.Errors)
	}
	if v.Data["summary"] != "looks good" {
		t.Fatalf("expected parsed data, got %v", v.Data)
	}
}

func TestValidateHandoffMissingRequiredStrict(t *testing.T) {
	schema := &agentcfg.HandoffSchema{
		Type:     "object",
		Required: []string{"summary"},
		Strict:   true,
	}

	output := `{"other": "field"}`
	v := ValidateHandoff(output, schema)
	if v.Valid {
		t.Fatal("expected invalid for missing required field in strict mode")
	}
	if len(v.Errors) == 0 {
		t.Fatal("expected errors")
	}
}

func TestValidateHandoffMissingRequiredNonStrict(t *testing.T) {
	schema := &agentcfg.HandoffSchema{
		Type:     "object",
		Required: []string{"summary"},
		Strict:   false,
	}

	output := `{"other": "field"}`
	v := ValidateHandoff(output, schema)
	if !v.Valid {
		t.Fatal("expected valid in non-strict mode (warnings only)")
	}
	if len(v.Warnings) == 0 {
		t.Fatal("expected warnings")
	}
}

func TestValidateHandoffWrongTypeStrict(t *testing.T) {
	schema := &agentcfg.HandoffSchema{
		Type: "object",
		Properties: map[string]agentcfg.SchemaProperty{
			"count": {Type: "integer"},
		},
		Strict: true,
	}

	output := `{"count": "not a number"}`
	v := ValidateHandoff(output, schema)
	if v.Valid {
		t.Fatal("expected invalid for wrong type in strict mode")
	}
}

func TestValidateHandoffWrongTypeNonStrict(t *testing.T) {
	schema := &agentcfg.HandoffSchema{
		Type: "object",
		Properties: map[string]agentcfg.SchemaProperty{
			"count": {Type: "integer"},
		},
		Strict: false,
	}

	output := `{"count": "not a number"}`
	v := ValidateHandoff(output, schema)
	if !v.Valid {
		t.Fatal("expected valid in non-strict (warnings only)")
	}
	if len(v.Warnings) == 0 {
		t.Fatal("expected warnings")
	}
}

func TestValidateHandoffNotJSON(t *testing.T) {
	schema := &agentcfg.HandoffSchema{
		Type:   "object",
		Strict: true,
	}

	v := ValidateHandoff("just plain text", schema)
	if v.Valid {
		t.Fatal("expected invalid for non-JSON in strict mode")
	}
}

func TestValidateHandoffNotJSONNonStrict(t *testing.T) {
	schema := &agentcfg.HandoffSchema{
		Type:   "object",
		Strict: false,
	}

	v := ValidateHandoff("just plain text", schema)
	if !v.Valid {
		t.Fatal("expected valid in non-strict mode")
	}
	if len(v.Warnings) == 0 {
		t.Fatal("expected warnings for non-JSON")
	}
}

func TestMatchesType(t *testing.T) {
	tests := []struct {
		val      interface{}
		typeName string
		want     bool
	}{
		{"hello", "string", true},
		{42.0, "number", true},
		{42.0, "integer", true},
		{42.5, "integer", false},
		{true, "boolean", true},
		{map[string]interface{}{}, "object", true},
		{[]interface{}{}, "array", true},
		{nil, "null", true},
		{"hello", "number", false},
	}

	for _, tt := range tests {
		got := matchesType(tt.val, tt.typeName)
		if got != tt.want {
			t.Errorf("matchesType(%v, %q) = %v, want %v", tt.val, tt.typeName, got, tt.want)
		}
	}
}

func TestFormatValidationClean(t *testing.T) {
	v := &HandoffValidation{Valid: true}
	if s := FormatValidation(v); s != "" {
		t.Fatalf("expected empty string for clean validation, got %q", s)
	}
}

func TestFormatValidationWithErrors(t *testing.T) {
	v := &HandoffValidation{
		Valid:  false,
		Errors: []string{"missing field"},
	}
	s := FormatValidation(v)
	if s == "" {
		t.Fatal("expected non-empty format")
	}
}
