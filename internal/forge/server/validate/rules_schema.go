package validate

import (
	"encoding/json"
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/protocol"
)

// parsedSchema is used internally to inspect input schema fields.
type parsedSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required"`
}

type schemaProperty struct {
	Description string `json:"description"`
}

func schemaRules() []Rule {
	return []Rule{
		{
			ID:          "schema-001",
			Category:    CategorySchema,
			Severity:    SeverityError,
			Description: "Input schema should be defined",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.InputSchema) > 0 {
					return nil
				}
				return []Violation{{
					RuleID:     "schema-001",
					Category:   CategorySchema,
					Severity:   SeverityError,
					ToolName:   tool.Name,
					Message:    "Tool has no input schema defined",
					Suggestion: "Define an inputSchema with type \"object\" and properties",
				}}
			},
		},
		{
			ID:          "schema-002",
			Category:    CategorySchema,
			Severity:    SeverityError,
			Description: "Input schema should be type \"object\"",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.InputSchema) == 0 {
					return nil // schema-001 covers this
				}
				var s parsedSchema
				if err := json.Unmarshal(tool.InputSchema, &s); err != nil {
					return nil // unparseable schema is a different concern
				}
				if s.Type == "object" {
					return nil
				}
				return []Violation{{
					RuleID:     "schema-002",
					Category:   CategorySchema,
					Severity:   SeverityError,
					ToolName:   tool.Name,
					Message:    fmt.Sprintf("Input schema type is %q, expected \"object\"", s.Type),
					Suggestion: "Set the inputSchema type to \"object\"",
				}}
			},
		},
		{
			ID:          "schema-003",
			Category:    CategorySchema,
			Severity:    SeverityError,
			Description: "Required fields should exist in properties",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.InputSchema) == 0 {
					return nil
				}
				var s parsedSchema
				if err := json.Unmarshal(tool.InputSchema, &s); err != nil {
					return nil
				}
				var violations []Violation
				for _, req := range s.Required {
					if _, ok := s.Properties[req]; !ok {
						violations = append(violations, Violation{
							RuleID:     "schema-003",
							Category:   CategorySchema,
							Severity:   SeverityError,
							ToolName:   tool.Name,
							Message:    fmt.Sprintf("Required field %q not found in properties", req),
							Suggestion: fmt.Sprintf("Add %q to the properties object or remove it from required", req),
						})
					}
				}
				return violations
			},
		},
		{
			ID:          "schema-004",
			Category:    CategorySchema,
			Severity:    SeverityWarning,
			Description: "Properties should have descriptions",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.InputSchema) == 0 {
					return nil
				}
				var s parsedSchema
				if err := json.Unmarshal(tool.InputSchema, &s); err != nil {
					return nil
				}
				var violations []Violation
				for name, prop := range s.Properties {
					if prop.Description == "" {
						violations = append(violations, Violation{
							RuleID:     "schema-004",
							Category:   CategorySchema,
							Severity:   SeverityWarning,
							ToolName:   tool.Name,
							Message:    fmt.Sprintf("Property %q has no description", name),
							Suggestion: fmt.Sprintf("Add a description to property %q", name),
						})
					}
				}
				return violations
			},
		},
	}
}
