package validate

import (
	"strings"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"
)

func responseRules() []Rule {
	return []Rule{
		{
			ID:          "response-001",
			Category:    CategoryResponse,
			Severity:    SeverityWarning,
			Description: "Tool should define what it returns in its description",
			Check: func(tool protocol.Tool) []Violation {
				if tool.Description == "" {
					return nil // naming-003 covers empty descriptions
				}
				desc := strings.ToLower(tool.Description)
				returnIndicators := []string{"returns", "responds", "outputs", "produces", "result", "yields"}
				for _, ind := range returnIndicators {
					if strings.Contains(desc, ind) {
						return nil
					}
				}
				return []Violation{{
					RuleID:     "response-001",
					Category:   CategoryResponse,
					Severity:   SeverityWarning,
					ToolName:   tool.Name,
					Message:    "Tool description does not mention what it returns",
					Suggestion: "Add what the tool returns to the description (e.g., \"Returns the current weather...\")",
				}}
			},
		},
	}
}
