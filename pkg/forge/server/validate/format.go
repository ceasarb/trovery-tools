package validate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatText produces a human-readable text report grouped by category and colored by severity.
func FormatText(report *Report) string {
	if len(report.Violations) == 0 {
		return "No violations found."
	}

	// Group violations by category
	grouped := make(map[Category][]Violation)
	for _, v := range report.Violations {
		grouped[v.Category] = append(grouped[v.Category], v)
	}

	var b strings.Builder

	// Order categories for consistent output
	categories := []Category{
		CategoryNaming,
		CategorySchema,
		CategoryAnnotation,
		CategoryError,
		CategoryResponse,
		CategoryPagination,
		CategorySecurity,
	}

	for _, cat := range categories {
		violations, ok := grouped[cat]
		if !ok {
			continue
		}

		b.WriteString(fmt.Sprintf("\n── %s ──\n", strings.ToUpper(string(cat))))

		for _, v := range violations {
			icon := severityIcon(v.Severity)
			b.WriteString(fmt.Sprintf("  %s [%s] %s: %s\n", icon, v.RuleID, v.ToolName, v.Message))
			if v.Suggestion != "" {
				b.WriteString(fmt.Sprintf("    → %s\n", v.Suggestion))
			}
		}
	}

	// Summary
	b.WriteString(fmt.Sprintf("\nSummary: %d error(s), %d warning(s), %d info(s) across %d tool(s)\n",
		report.Summary.Errors, report.Summary.Warnings, report.Summary.Infos, report.Summary.TotalTools))

	return b.String()
}

// FormatJSON produces a machine-readable JSON report.
func FormatJSON(report *Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func severityIcon(s Severity) string {
	switch s {
	case SeverityError:
		return "ERR"
	case SeverityWarning:
		return "WRN"
	case SeverityInfo:
		return "INF"
	default:
		return "???"
	}
}
