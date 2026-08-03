package validate

import "github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"

// Validator runs validation rules against MCP tool definitions.
type Validator struct {
	rules []Rule
}

// NewValidator creates a validator with all built-in rules registered.
func NewValidator() *Validator {
	v := &Validator{}
	v.rules = append(v.rules, namingRules()...)
	v.rules = append(v.rules, schemaRules()...)
	v.rules = append(v.rules, annotationRules()...)
	v.rules = append(v.rules, errorRules()...)
	v.rules = append(v.rules, responseRules()...)
	v.rules = append(v.rules, paginationRules()...)
	v.rules = append(v.rules, securityRules()...)
	return v
}

// Validate runs all rules against the given tools and produces a report.
func (v *Validator) Validate(tools []protocol.Tool) *Report {
	return v.validate(tools, nil)
}

// ValidateCategory runs only rules in the specified category.
func (v *Validator) ValidateCategory(tools []protocol.Tool, category Category) *Report {
	return v.validate(tools, &category)
}

func (v *Validator) validate(tools []protocol.Tool, filterCategory *Category) *Report {
	var violations []Violation
	rulesRun := 0

	for _, rule := range v.rules {
		if filterCategory != nil && rule.Category != *filterCategory {
			continue
		}
		rulesRun++
		for _, tool := range tools {
			if vv := rule.Check(tool); len(vv) > 0 {
				violations = append(violations, vv...)
			}
		}
	}

	report := &Report{
		Violations: violations,
		Summary: ReportSummary{
			TotalTools: len(tools),
			TotalRules: rulesRun,
			ByCategory: make(map[Category]int),
		},
	}

	for _, v := range violations {
		switch v.Severity {
		case SeverityError:
			report.Summary.Errors++
		case SeverityWarning:
			report.Summary.Warnings++
		case SeverityInfo:
			report.Summary.Infos++
		}
		report.Summary.ByCategory[v.Category]++
	}

	return report
}

// Report holds the complete validation results.
type Report struct {
	Violations []Violation   `json:"violations"`
	Summary    ReportSummary `json:"summary"`
}

// ReportSummary provides aggregate counts of validation findings.
type ReportSummary struct {
	TotalTools int             `json:"total_tools"`
	TotalRules int             `json:"total_rules"`
	Errors     int             `json:"errors"`
	Warnings   int             `json:"warnings"`
	Infos      int             `json:"infos"`
	ByCategory map[Category]int `json:"by_category"`
}
