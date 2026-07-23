package validate

import (
	"strings"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/protocol"
)

func errorRules() []Rule {
	return []Rule{
		{
			ID:          "errors-001",
			Category:    CategoryError,
			Severity:    SeverityInfo,
			Description: "Tool name suggests it may fail — ensure error content is documented",
			Check: func(tool protocol.Tool) []Violation {
				desc := strings.ToLower(tool.Description)
				name := strings.ToLower(tool.Name)
				failIndicators := []string{"send", "upload", "connect", "request", "submit", "execute"}
				mayFail := false
				for _, ind := range failIndicators {
					if strings.Contains(name, ind) {
						mayFail = true
						break
					}
				}
				if !mayFail {
					return nil
				}
				errorDocTerms := []string{"error", "fail", "exception", "invalid"}
				for _, term := range errorDocTerms {
					if strings.Contains(desc, term) {
						return nil // error behavior is mentioned
					}
				}
				return []Violation{{
					RuleID:     "errors-001",
					Category:   CategoryError,
					Severity:   SeverityInfo,
					ToolName:   tool.Name,
					Message:    "Tool may fail but description does not mention error behavior",
					Suggestion: "Document what happens when the tool encounters an error",
				}}
			},
		},
	}
}
