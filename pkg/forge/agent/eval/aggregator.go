package eval

// AggregatedResult summarizes multiple runs of the same scenario.
type AggregatedResult struct {
	ScenarioName string                `json:"scenario_name"`
	TotalRuns    int                   `json:"total_runs"`
	PassRate     float64               `json:"pass_rate"`
	Assertions   []AggregatedAssertion `json:"assertions"`
}

// AggregatedAssertion summarizes a single assertion type across runs.
type AggregatedAssertion struct {
	Type     string   `json:"type"`
	PassRate float64  `json:"pass_rate"`
	Failures []string `json:"failures,omitempty"`
}

// Aggregate computes pass rates across multiple scenario results.
func Aggregate(scenarioName string, results []*ScenarioResult) *AggregatedResult {
	if len(results) == 0 {
		return &AggregatedResult{ScenarioName: scenarioName}
	}

	totalRuns := len(results)
	passedRuns := 0
	for _, r := range results {
		if r.Passed {
			passedRuns++
		}
	}

	// Collect assertion results by type across all runs
	assertionsByType := map[string][]AssertionResult{}
	for _, r := range results {
		for _, a := range r.Assertions {
			assertionsByType[a.Type] = append(assertionsByType[a.Type], a)
		}
	}

	// Build aggregated assertions preserving order from first result
	seen := map[string]bool{}
	var orderedTypes []string
	for _, r := range results {
		for _, a := range r.Assertions {
			if !seen[a.Type] {
				seen[a.Type] = true
				orderedTypes = append(orderedTypes, a.Type)
			}
		}
	}

	aggregatedAssertions := make([]AggregatedAssertion, 0, len(orderedTypes))
	for _, aType := range orderedTypes {
		all := assertionsByType[aType]
		passed := 0
		var failures []string
		for _, ar := range all {
			if ar.Passed {
				passed++
			} else {
				failures = append(failures, ar.Message)
			}
		}
		aggregatedAssertions = append(aggregatedAssertions, AggregatedAssertion{
			Type:     aType,
			PassRate: float64(passed) / float64(len(all)),
			Failures: failures,
		})
	}

	return &AggregatedResult{
		ScenarioName: scenarioName,
		TotalRuns:    totalRuns,
		PassRate:     float64(passedRuns) / float64(totalRuns),
		Assertions:   aggregatedAssertions,
	}
}
