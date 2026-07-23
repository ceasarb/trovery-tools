package validate

import (
	"strings"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/protocol"
)

func annotationRules() []Rule {
	return []Rule{
		{
			ID:          "annotations-001",
			Category:    CategoryAnnotation,
			Severity:    SeverityInfo,
			Description: "Tools should have annotations",
			Check: func(tool protocol.Tool) []Violation {
				if tool.Annotations != nil {
					return nil
				}
				return []Violation{{
					RuleID:     "annotations-001",
					Category:   CategoryAnnotation,
					Severity:   SeverityInfo,
					ToolName:   tool.Name,
					Message:    "Tool has no annotations",
					Suggestion: "Add annotations to describe tool behavior (readOnlyHint, destructiveHint, etc.)",
				}}
			},
		},
		{
			ID:          "annotations-002",
			Category:    CategoryAnnotation,
			Severity:    SeverityInfo,
			Description: "readOnlyHint should be set for read-only tools",
			Check: func(tool protocol.Tool) []Violation {
				if tool.Annotations != nil {
					return nil // has annotations, assume developer considered it
				}
				name := strings.ToLower(tool.Name)
				desc := strings.ToLower(tool.Description)
				readIndicators := []string{"get", "list", "fetch", "read", "search", "find", "query", "lookup"}
				for _, ind := range readIndicators {
					if strings.Contains(name, ind) || strings.HasPrefix(desc, ind) {
						return []Violation{{
							RuleID:     "annotations-002",
							Category:   CategoryAnnotation,
							Severity:   SeverityInfo,
							ToolName:   tool.Name,
							Message:    "Tool appears to be read-only but has no readOnlyHint annotation",
							Suggestion: "Add annotations with readOnlyHint: true",
						}}
					}
				}
				return nil
			},
		},
		{
			ID:          "annotations-003",
			Category:    CategoryAnnotation,
			Severity:    SeverityWarning,
			Description: "destructiveHint should be set for write/delete tools",
			Check: func(tool protocol.Tool) []Violation {
				name := strings.ToLower(tool.Name)
				desc := strings.ToLower(tool.Description)
				destructiveIndicators := []string{"delete", "remove", "drop", "destroy", "purge", "truncate"}
				isDestructive := false
				for _, ind := range destructiveIndicators {
					if strings.Contains(name, ind) || strings.Contains(desc, ind) {
						isDestructive = true
						break
					}
				}
				if !isDestructive {
					return nil
				}
				if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil {
					return []Violation{{
						RuleID:     "annotations-003",
						Category:   CategoryAnnotation,
						Severity:   SeverityWarning,
						ToolName:   tool.Name,
						Message:    "Tool appears destructive but destructiveHint is not set",
						Suggestion: "Add annotations with destructiveHint: true",
					}}
				}
				return nil
			},
		},
	}
}
