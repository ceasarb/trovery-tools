package validate

import (
	"encoding/json"
	"strings"

	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

func paginationRules() []Rule {
	return []Rule{
		{
			ID:          "pagination-001",
			Category:    CategoryPagination,
			Severity:    SeverityInfo,
			Description: "Tools returning lists should support cursor/pagination parameters",
			Check: func(tool protocol.Tool) []Violation {
				name := strings.ToLower(tool.Name)
				desc := strings.ToLower(tool.Description)
				listIndicators := []string{"list", "search", "find_all", "get_all", "fetch_all"}
				isList := false
				for _, ind := range listIndicators {
					if strings.Contains(name, ind) || strings.Contains(desc, ind) {
						isList = true
						break
					}
				}
				if !isList {
					return nil
				}
				// Check if schema has pagination params
				if len(tool.InputSchema) > 0 {
					var s parsedSchema
					if err := json.Unmarshal(tool.InputSchema, &s); err == nil {
						paginationParams := []string{"cursor", "page", "offset", "limit", "page_size", "next_token"}
						for prop := range s.Properties {
							for _, pp := range paginationParams {
								if strings.Contains(strings.ToLower(prop), pp) {
									return nil
								}
							}
						}
					}
				}
				return []Violation{{
					RuleID:     "pagination-001",
					Category:   CategoryPagination,
					Severity:   SeverityInfo,
					ToolName:   tool.Name,
					Message:    "Tool appears to return a list but has no pagination parameters",
					Suggestion: "Add cursor, page, or limit parameters for paginated results",
				}}
			},
		},
	}
}
