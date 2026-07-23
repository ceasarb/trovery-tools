package eval

import (
	"encoding/json"
	"time"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/storage"
)

// Regression represents a scenario that previously passed but now fails.
type Regression struct {
	ScenarioName   string `json:"scenario_name"`
	PreviousStatus string `json:"previous_status"`
	CurrentStatus  string `json:"current_status"`
}

// DetectRegressions compares current scenario results against stored baselines.
// A regression is a scenario whose baseline status was "passed" but now fails.
func DetectRegressions(store *storage.EvalStore, suiteName string, results []ScenarioResult) ([]Regression, error) {
	var regressions []Regression

	for _, sr := range results {
		baseline, err := store.GetBaseline("server", suiteName, suiteName, sr.Name)
		if err != nil {
			return nil, err
		}
		if baseline == nil {
			continue // no baseline yet
		}

		// Parse baseline to get previous status
		var prev struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(baseline.BaselineJSON), &prev); err != nil {
			continue
		}

		if prev.Status == "passed" && sr.Status != "passed" {
			regressions = append(regressions, Regression{
				ScenarioName:   sr.Name,
				PreviousStatus: prev.Status,
				CurrentStatus:  sr.Status,
			})
		}
	}

	return regressions, nil
}

// UpdateBaselines stores the current scenario results as the new baselines.
func UpdateBaselines(store *storage.EvalStore, suiteName string, results []ScenarioResult) error {
	now := time.Now()

	for _, sr := range results {
		baselineData, err := json.Marshal(map[string]any{
			"status":     sr.Status,
			"assertions": sr.Assertions,
		})
		if err != nil {
			return err
		}

		baseline := &storage.EvalBaseline{
			Source:       "server",
			TargetName:   suiteName,
			SuiteName:    suiteName,
			ScenarioName: sr.Name,
			BaselineJSON: string(baselineData),
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := store.SetBaseline(baseline); err != nil {
			return err
		}
	}

	return nil
}
