package validate

import (
	"fmt"
	"regexp"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"
)

var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

func namingRules() []Rule {
	return []Rule{
		{
			ID:          "naming-001",
			Category:    CategoryNaming,
			Severity:    SeverityError,
			Description: "Tool names should use snake_case",
			Check: func(tool protocol.Tool) []Violation {
				if tool.Name == "" || snakeCaseRe.MatchString(tool.Name) {
					return nil
				}
				return []Violation{{
					RuleID:     "naming-001",
					Category:   CategoryNaming,
					Severity:   SeverityError,
					ToolName:   tool.Name,
					Message:    fmt.Sprintf("Tool name %q is not snake_case", tool.Name),
					Suggestion: "Rename to use lowercase letters, digits, and underscores (e.g., get_weather)",
				}}
			},
		},
		{
			ID:          "naming-002",
			Category:    CategoryNaming,
			Severity:    SeverityWarning,
			Description: "Tool names should not exceed 64 characters",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.Name) <= 64 {
					return nil
				}
				return []Violation{{
					RuleID:     "naming-002",
					Category:   CategoryNaming,
					Severity:   SeverityWarning,
					ToolName:   tool.Name,
					Message:    fmt.Sprintf("Tool name is %d characters (max 64)", len(tool.Name)),
					Suggestion: "Shorten the tool name to 64 characters or fewer",
				}}
			},
		},
		{
			ID:          "naming-003",
			Category:    CategoryNaming,
			Severity:    SeverityError,
			Description: "Tool description should not be empty",
			Check: func(tool protocol.Tool) []Violation {
				if tool.Description != "" {
					return nil
				}
				return []Violation{{
					RuleID:     "naming-003",
					Category:   CategoryNaming,
					Severity:   SeverityError,
					ToolName:   tool.Name,
					Message:    "Tool has no description",
					Suggestion: "Add a clear description explaining what the tool does",
				}}
			},
		},
		{
			ID:          "naming-004",
			Category:    CategoryNaming,
			Severity:    SeverityWarning,
			Description: "Tool description should not exceed 1024 characters",
			Check: func(tool protocol.Tool) []Violation {
				if len(tool.Description) <= 1024 {
					return nil
				}
				return []Violation{{
					RuleID:     "naming-004",
					Category:   CategoryNaming,
					Severity:   SeverityWarning,
					ToolName:   tool.Name,
					Message:    fmt.Sprintf("Tool description is %d characters (max 1024)", len(tool.Description)),
					Suggestion: "Shorten the description to 1024 characters or fewer",
				}}
			},
		},
	}
}
