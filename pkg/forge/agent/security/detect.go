package security

import (
	"regexp"
	"strings"
)

// Detection holds the result of injection pattern analysis.
type Detection struct {
	IsSuspicious bool
	Patterns     []string
	RiskScore    float64 // 0.0 to 1.0
}

type pattern struct {
	name   string
	re     *regexp.Regexp
	weight float64
}

var injectionPatterns = []pattern{
	// Instruction override attempts
	{name: "instruction_override", re: regexp.MustCompile(`(?i)(ignore|disregard|forget|override)\s+(all\s+)?(previous|above|prior|earlier)\s+(instructions?|rules?|prompts?|guidelines?)`), weight: 0.4},
	{name: "instruction_override", re: regexp.MustCompile(`(?i)new\s+(instructions?|rules?|task|objective)\s*:`), weight: 0.3},
	{name: "instruction_override", re: regexp.MustCompile(`(?i)from\s+now\s+on\s+(you\s+)?(are|will|must|should)`), weight: 0.3},

	// Role assumption
	{name: "role_assumption", re: regexp.MustCompile(`(?i)you\s+are\s+(now|actually|really)\s+`), weight: 0.3},
	{name: "role_assumption", re: regexp.MustCompile(`(?i)(act|behave|respond|pretend)\s+(as|like|to\s+be)\s+`), weight: 0.3},
	{name: "role_assumption", re: regexp.MustCompile(`(?i)enter\s+(developer|debug|admin|sudo|root|god)\s+mode`), weight: 0.4},

	// System prompt extraction
	{name: "system_prompt_extraction", re: regexp.MustCompile(`(?i)(show|reveal|display|print|output|repeat|tell\s+me)\s+(your\s+)?(system\s+prompt|instructions|rules|initial\s+prompt|configuration)`), weight: 0.5},
	{name: "system_prompt_extraction", re: regexp.MustCompile(`(?i)what\s+(are|were)\s+your\s+(original\s+)?(instructions|rules|guidelines|system\s+prompt)`), weight: 0.4},

	// Delimiter escape
	{name: "delimiter_escape", re: regexp.MustCompile(`\[/(USER_MESSAGE|TOOL_RESULT|SKILL_METADATA)`), weight: 0.5},
	{name: "delimiter_escape", re: regexp.MustCompile(`\[(SYSTEM|ASSISTANT|USER)\]`), weight: 0.3},
}

// DetectInjection analyzes input for prompt injection patterns.
// Returns a Detection with matched patterns and an aggregate risk score.
func DetectInjection(s string) Detection {
	if s == "" {
		return Detection{}
	}

	var matched []string
	var totalWeight float64
	seen := make(map[string]bool)

	lower := strings.ToLower(s)
	_ = lower // patterns use (?i) flag

	for _, p := range injectionPatterns {
		if p.re.MatchString(s) {
			if !seen[p.name] {
				matched = append(matched, p.name)
				seen[p.name] = true
			}
			totalWeight += p.weight
		}
	}

	// Cap risk score at 1.0
	if totalWeight > 1.0 {
		totalWeight = 1.0
	}

	return Detection{
		IsSuspicious: len(matched) > 0,
		Patterns:     matched,
		RiskScore:    totalWeight,
	}
}
