package reporting

import (
	"encoding/json"

	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

// SARIF 2.1.0 structures for GitHub Security tab integration.

type sarifLog struct {
	Schema  string      `json:"$schema"`
	Version string      `json:"version"`
	Runs    []sarifRun  `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string       `json:"name"`
	InformationURI string       `json:"informationUri"`
	Version        string       `json:"version"`
	Rules          []sarifRule  `json:"rules"`
}

type sarifRule struct {
	ID               string          `json:"id"`
	ShortDescription sarifMessage    `json:"shortDescription"`
	DefaultConfig    sarifRuleConfig `json:"defaultConfiguration"`
}

type sarifRuleConfig struct {
	Level string `json:"level"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// FormatSARIF converts violations to SARIF 2.1.0 JSON. version is the running
// build's version (from ldflags), reported as the driver version.
func FormatSARIF(violations []session.PolicyViolation, version string) ([]byte, error) {
	// Collect unique rules.
	ruleMap := make(map[string]bool)
	var rules []sarifRule
	for _, v := range violations {
		if !ruleMap[v.Rule] {
			ruleMap[v.Rule] = true
			level := "warning"
			if v.Severity == "error" {
				level = "error"
			}
			rules = append(rules, sarifRule{
				ID:               v.Rule,
				ShortDescription: sarifMessage{Text: v.Rule},
				DefaultConfig:    sarifRuleConfig{Level: level},
			})
		}
	}

	var results []sarifResult
	for _, v := range violations {
		level := "warning"
		if v.Severity == "error" {
			level = "error"
		}

		r := sarifResult{
			RuleID:  v.Rule,
			Level:   level,
			Message: sarifMessage{Text: v.Message},
		}

		if v.File != "" {
			loc := sarifLocation{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: v.File},
				},
			}
			if v.Line > 0 {
				loc.PhysicalLocation.Region = &sarifRegion{StartLine: v.Line}
			}
			r.Locations = append(r.Locations, loc)
		}

		results = append(results, r)
	}

	log := sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{
			{
				Tool: sarifTool{
					Driver: sarifDriver{
						Name:           "trovery-vigil",
						InformationURI: "https://github.com/ceasarb/trovery-tools",
						Version:        version,
						Rules:          rules,
					},
				},
				Results: results,
			},
		},
	}

	return json.MarshalIndent(log, "", "  ")
}

// FormatViolationsJSON converts violations to a simple JSON array.
func FormatViolationsJSON(violations []session.PolicyViolation) ([]byte, error) {
	return json.MarshalIndent(violations, "", "  ")
}
