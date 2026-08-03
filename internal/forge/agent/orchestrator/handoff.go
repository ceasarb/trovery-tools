package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
)

// HandoffValidation holds the result of validating agent output against a schema.
type HandoffValidation struct {
	Valid    bool
	Errors  []string
	Warnings []string
	Data     map[string]interface{} // parsed structured data (nil if not JSON)
}

// ValidateHandoff checks the agent output against the output_schema defined for the agent.
// Returns validation result. If no schema is defined, always valid.
func ValidateHandoff(output string, schema *agentcfg.HandoffSchema) *HandoffValidation {
	if schema == nil {
		return &HandoffValidation{Valid: true}
	}

	result := &HandoffValidation{Valid: true}

	// Try to parse output as JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		msg := fmt.Sprintf("output is not valid JSON: %v", err)
		if schema.Strict {
			result.Valid = false
			result.Errors = append(result.Errors, msg)
		} else {
			result.Warnings = append(result.Warnings, msg)
		}
		return result
	}
	result.Data = data

	// Check required fields
	for _, req := range schema.Required {
		if _, ok := data[req]; !ok {
			msg := fmt.Sprintf("missing required field: %s", req)
			if schema.Strict {
				result.Valid = false
				result.Errors = append(result.Errors, msg)
			} else {
				result.Warnings = append(result.Warnings, msg)
			}
		}
	}

	// Check property types
	for name, prop := range schema.Properties {
		val, ok := data[name]
		if !ok {
			continue // missing optional field is fine
		}

		if !matchesType(val, prop.Type) {
			msg := fmt.Sprintf("field %q: expected type %s, got %T", name, prop.Type, val)
			if schema.Strict {
				result.Valid = false
				result.Errors = append(result.Errors, msg)
			} else {
				result.Warnings = append(result.Warnings, msg)
			}
		}
	}

	return result
}

// FormatValidation returns a human-readable string of validation results.
func FormatValidation(v *HandoffValidation) string {
	if v.Valid && len(v.Warnings) == 0 {
		return ""
	}

	var parts []string
	for _, e := range v.Errors {
		parts = append(parts, "ERROR: "+e)
	}
	for _, w := range v.Warnings {
		parts = append(parts, "WARN: "+w)
	}
	return strings.Join(parts, "; ")
}

// matchesType checks if a JSON value matches the expected JSON Schema type.
func matchesType(val interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := val.(string)
		return ok
	case "number":
		_, ok := val.(float64)
		return ok
	case "integer":
		f, ok := val.(float64)
		return ok && f == float64(int64(f))
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "object":
		_, ok := val.(map[string]interface{})
		return ok
	case "array":
		_, ok := val.([]interface{})
		return ok
	case "null":
		return val == nil
	default:
		return true // unknown type, pass
	}
}
