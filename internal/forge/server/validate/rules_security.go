package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/protocol"
)

func securityRules() []Rule {
	return []Rule{
		{
			ID:          "security-001",
			Category:    CategorySecurity,
			Severity:    SeverityWarning,
			Description: "Tool parameters should not accept raw SQL",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.InputSchema) == 0 {
					return nil
				}
				var s parsedSchema
				if err := json.Unmarshal(tool.InputSchema, &s); err != nil {
					return nil
				}
				sqlIndicators := []string{"sql", "query", "statement"}
				var violations []Violation
				for name := range s.Properties {
					lower := strings.ToLower(name)
					for _, ind := range sqlIndicators {
						if lower == ind || strings.HasSuffix(lower, "_"+ind) {
							violations = append(violations, Violation{
								RuleID:     "security-001",
								Category:   CategorySecurity,
								Severity:   SeverityWarning,
								ToolName:   tool.Name,
								Message:    fmt.Sprintf("Parameter %q may accept raw SQL", name),
								Suggestion: "Use parameterized queries instead of accepting raw SQL strings",
							})
							break
						}
					}
				}
				return violations
			},
		},
		{
			ID:          "security-002",
			Category:    CategorySecurity,
			Severity:    SeverityWarning,
			Description: "Tool parameters should not accept file paths without validation note",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.InputSchema) == 0 {
					return nil
				}
				var s parsedSchema
				if err := json.Unmarshal(tool.InputSchema, &s); err != nil {
					return nil
				}
				pathIndicators := []string{"path", "file", "filepath", "file_path", "filename"}
				var violations []Violation
				for name, prop := range s.Properties {
					lower := strings.ToLower(name)
					isPath := false
					for _, ind := range pathIndicators {
						if lower == ind || strings.HasSuffix(lower, "_"+ind) || strings.HasPrefix(lower, ind+"_") {
							isPath = true
							break
						}
					}
					if !isPath {
						continue
					}
					descLower := strings.ToLower(prop.Description)
					if strings.Contains(descLower, "validat") || strings.Contains(descLower, "sanitiz") || strings.Contains(descLower, "allowlist") || strings.Contains(descLower, "whitelist") {
						continue
					}
					violations = append(violations, Violation{
						RuleID:     "security-002",
						Category:   CategorySecurity,
						Severity:   SeverityWarning,
						ToolName:   tool.Name,
						Message:    fmt.Sprintf("Parameter %q accepts a file path without validation note", name),
						Suggestion: "Document path validation/sanitization in the property description",
					})
				}
				return violations
			},
		},
		{
			ID:          "security-003",
			Category:    CategorySecurity,
			Severity:    SeverityError,
			Description: "Tool description should not mention credentials or secrets",
			Check: func(tool protocol.Tool) []Violation {
				desc := strings.ToLower(tool.Description)
				secrets := []string{"password", "secret", "api_key", "apikey", "token", "credential", "private_key"}
				for _, s := range secrets {
					if strings.Contains(desc, s) {
						return []Violation{{
							RuleID:     "security-003",
							Category:   CategorySecurity,
							Severity:   SeverityError,
							ToolName:   tool.Name,
							Message:    fmt.Sprintf("Tool description mentions %q — secrets should not be in descriptions", s),
							Suggestion: "Remove credential references from the description; pass secrets via environment variables",
						}}
					}
				}
				return nil
			},
		},
	}
}
