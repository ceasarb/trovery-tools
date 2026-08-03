package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/provider"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/storage"
)

// RunResult holds the complete outcome of running an eval suite.
type RunResult struct {
	RunID      string              `json:"run_id"`
	SuiteName  string              `json:"suite_name"`
	AgentName  string              `json:"agent_name"`
	Status     string              `json:"status"` // passed, failed, error
	Scenarios  []*ScenarioResult   `json:"scenarios"`
	Aggregated []*AggregatedResult `json:"aggregated,omitempty"`
	Passed     int                 `json:"passed"`
	Failed     int                 `json:"failed"`
	Duration   time.Duration       `json:"duration"`
}

// Engine orchestrates eval suite execution with persistence.
type Engine struct {
	store   *storage.EvalStore
	OnEvent func(Event) // optional live progress callback
}

// New creates an eval engine backed by the given store.
func New(store *storage.EvalStore) *Engine {
	return &Engine{store: store}
}

// RunSuite executes all scenarios in a suite, storing results in SQLite.
func (e *Engine) RunSuite(ctx context.Context, suite *Suite, cfg *agentcfg.AgentConfig, prov provider.Provider, caller ToolCaller, tools []provider.ToolDef, runsOverride int) (*RunResult, error) {
	start := time.Now()

	// Create eval run in store
	run := &storage.EvalRun{
		Source:     "agent",
		TargetName: cfg.Name,
		SuiteName:  suite.Name,
		StartedAt:  start,
		Status:     "running",
	}
	if err := e.store.CreateRun(run); err != nil {
		return nil, fmt.Errorf("create eval run: %w", err)
	}

	result := &RunResult{
		RunID:     run.ID,
		SuiteName: suite.Name,
		AgentName: cfg.Name,
	}

	for _, scenario := range suite.Scenarios {
		runs := scenario.Runs
		if runsOverride > 0 {
			runs = runsOverride
		}

		// Wrap caller with mock injector if scenario has mock errors
		scenarioCaller := caller
		if len(scenario.MockErrors) > 0 {
			scenarioCaller = NewMockInjector(caller, scenario.MockErrors)
		}
		runner := NewRunner(cfg, prov, scenarioCaller, tools)
		runner.OnEvent = e.OnEvent

		var scenarioResults []*ScenarioResult

		for r := 0; r < runs; r++ {
			// Reset mock injector between runs
			if mi, ok := scenarioCaller.(*MockInjector); ok {
				mi.Reset()
			}

			if e.OnEvent != nil {
				e.OnEvent(Event{Kind: EventScenarioStart, Scenario: scenario.Name, Run: r + 1, TotalRuns: runs})
			}

			sr := runner.RunScenario(ctx, scenario)

			if e.OnEvent != nil {
				e.OnEvent(Event{Kind: EventScenarioEnd, Scenario: scenario.Name, Run: r + 1, TotalRuns: runs, Passed: sr.Passed, Duration: sr.Duration, TokensUsed: captured(sr)})
			}
			scenarioResults = append(scenarioResults, sr)

			// Store each result
			durMs := sr.Duration.Milliseconds()
			assertionsJSON, _ := json.Marshal(sr.Assertions)
			assertionsStr := string(assertionsJSON)

			evalResult := &storage.EvalResult{
				RunID:          run.ID,
				ScenarioName:   scenario.Name,
				Status:         statusString(sr),
				DurationMs:     &durMs,
				AssertionsJSON: &assertionsStr,
				CreatedAt:      time.Now(),
			}
			if sr.Error != "" {
				evalResult.ErrorMessage = &sr.Error
			}
			e.store.CreateResult(evalResult)
		}

		// Aggregate if multi-run
		if runs > 1 {
			agg := Aggregate(scenario.Name, scenarioResults)
			result.Aggregated = append(result.Aggregated, agg)
		}

		// Use the last result (or single result) as the scenario outcome
		for _, sr := range scenarioResults {
			result.Scenarios = append(result.Scenarios, sr)
			if sr.Passed {
				result.Passed++
			} else {
				result.Failed++
			}
		}
	}

	// Finalize run
	now := time.Now()
	result.Duration = now.Sub(start)

	if result.Failed > 0 {
		result.Status = "failed"
		run.Status = "failed"
	} else {
		result.Status = "passed"
		run.Status = "passed"
	}

	run.FinishedAt = &now
	run.TotalScenarios = result.Passed + result.Failed
	run.Passed = result.Passed
	run.Failed = result.Failed
	e.store.UpdateRun(run)

	return result, nil
}

// UpdateBaselines saves current results as baselines for future comparison.
func (e *Engine) UpdateBaselines(suite *Suite, agentName string, result *RunResult) error {
	now := time.Now()
	for _, sr := range result.Scenarios {
		assertionsJSON, err := json.Marshal(sr.Assertions)
		if err != nil {
			continue
		}
		baseline := &storage.EvalBaseline{
			Source:       "agent",
			TargetName:   agentName,
			SuiteName:    suite.Name,
			ScenarioName: sr.ScenarioName,
			BaselineJSON: string(assertionsJSON),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := e.store.SetBaseline(baseline); err != nil {
			return fmt.Errorf("set baseline for %s: %w", sr.ScenarioName, err)
		}
	}
	return nil
}

// LoadLastRun reconstructs a RunResult from the most recent stored run for the given agent.
func LoadLastRun(store *storage.EvalStore, agentName string) (*RunResult, error) {
	runs, err := store.ListRuns("agent", 10)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	// Find the most recent run for this agent
	var run *storage.EvalRun
	for i := range runs {
		if runs[i].TargetName == agentName {
			run = &runs[i]
			break
		}
	}
	if run == nil {
		return nil, fmt.Errorf("no previous eval runs found for agent %q", agentName)
	}

	results, err := store.GetResultsByRun(run.ID)
	if err != nil {
		return nil, fmt.Errorf("get results: %w", err)
	}

	var duration time.Duration
	if run.FinishedAt != nil {
		duration = run.FinishedAt.Sub(run.StartedAt)
	}

	result := &RunResult{
		RunID:     run.ID,
		SuiteName: run.SuiteName,
		AgentName: run.TargetName,
		Status:    run.Status,
		Passed:    run.Passed,
		Failed:    run.Failed,
		Duration:  duration,
	}

	for _, r := range results {
		sr := &ScenarioResult{
			ScenarioName: r.ScenarioName,
			Passed:       r.Status == "passed",
			Error:        deref(r.ErrorMessage),
		}
		if r.DurationMs != nil {
			sr.Duration = time.Duration(*r.DurationMs) * time.Millisecond
		}
		if r.AssertionsJSON != nil {
			var assertions []AssertionResult
			if err := json.Unmarshal([]byte(*r.AssertionsJSON), &assertions); err == nil {
				sr.Assertions = assertions
			}
		}
		result.Scenarios = append(result.Scenarios, sr)
	}

	return result, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func captured(sr *ScenarioResult) int {
	if sr.Captured != nil {
		return sr.Captured.TotalTokens
	}
	return 0
}

func statusString(sr *ScenarioResult) string {
	if sr.Error != "" {
		return "error"
	}
	if sr.Passed {
		return "passed"
	}
	return "failed"
}
