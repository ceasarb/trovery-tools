package validate

import "github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"

// Severity indicates how critical a rule violation is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Category groups related validation rules.
type Category string

const (
	CategoryNaming     Category = "naming"
	CategorySchema     Category = "schema"
	CategoryAnnotation Category = "annotations"
	CategoryError      Category = "errors"
	CategoryResponse   Category = "response"
	CategoryPagination Category = "pagination"
	CategorySecurity   Category = "security"
)

// Rule defines a single validation check against an MCP tool definition.
type Rule struct {
	ID          string
	Category    Category
	Severity    Severity
	Description string
	Check       func(tool protocol.Tool) []Violation
}

// Violation represents a single rule failure for a specific tool.
type Violation struct {
	RuleID     string   `json:"rule_id"`
	Category   Category `json:"category"`
	Severity   Severity `json:"severity"`
	ToolName   string   `json:"tool_name"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion,omitempty"`
}
