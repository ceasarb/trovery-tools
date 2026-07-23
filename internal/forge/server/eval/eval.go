package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ceasarb/demigo-tools/internal/forge/server/harness"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/storage"
	"github.com/google/uuid"
)

// RunResult is the aggregate outcome of running an entire eval suite.
type RunResult struct {
	RunID      string           `json:"run_id"`
	SuiteName  string           `json:"suite_name"`
	Status     string           `json:"status"` // passed, failed, error
	Total      int              `json:"total"`
	Passed     int              `json:"passed"`
	Failed     int              `json:"failed"`
	Skipped    int              `json:"skipped"`
	Duration   time.Duration    `json:"duration"`
	Scenarios  []ScenarioResult `json:"scenarios"`
	ResultData map[string]any   `json:"-"` // scenario name -> raw result data
}

// Engine orchestrates eval suite execution and persistence.
type Engine struct {
	store *storage.EvalStore
}

// New creates a new eval Engine backed by the given store.
func New(store *storage.EvalStore) *Engine {
	return &Engine{store: store}
}

// RunSuite executes all scenarios in a suite against a server in serverDir.
func (e *Engine) RunSuite(ctx context.Context, suite *Suite, serverDir string) (*RunResult, error) {
	// Parse server command from suite or discover from config
	command := suite.Server
	if command == "" {
		return nil, fmt.Errorf("suite %q has no server command", suite.Name)
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty server command in suite %q", suite.Name)
	}

	// Create eval run record
	runID := uuid.New().String()
	now := time.Now()
	run := &storage.EvalRun{
		ID:             runID,
		Source:         "server",
		TargetName:     suite.Name,
		SuiteName:      suite.Name,
		StartedAt:      now,
		TotalScenarios: len(suite.Scenarios),
		Status:         "running",
	}
	if err := e.store.CreateRun(run); err != nil {
		return nil, fmt.Errorf("create eval run: %w", err)
	}

	// Start MCP server
	client, err := harness.Start(ctx, parts[0], parts[1:], serverDir)
	if err != nil {
		run.Status = "error"
		finished := time.Now()
		run.FinishedAt = &finished
		e.store.UpdateRun(run)
		return nil, fmt.Errorf("start server: %w", err)
	}
	defer client.Close()

	// Execute scenarios
	result := &RunResult{
		RunID:      runID,
		SuiteName:  suite.Name,
		ResultData: make(map[string]any),
	}

	start := time.Now()

	for _, scenario := range suite.Scenarios {
		sr := RunScenario(ctx, client, scenario)
		result.Scenarios = append(result.Scenarios, sr)
		result.ResultData[scenario.Name] = sr.ResultData

		switch sr.Status {
		case "passed":
			result.Passed++
		case "failed":
			result.Failed++
		default:
			result.Skipped++
		}

		// Persist individual result
		durationMs := sr.Duration.Milliseconds()
		assertionsJSON, _ := json.Marshal(sr.Assertions)
		assertionsStr := string(assertionsJSON)

		evalResult := &storage.EvalResult{
			RunID:          runID,
			ScenarioName:   sr.Name,
			Status:         sr.Status,
			DurationMs:     &durationMs,
			AssertionsJSON: &assertionsStr,
			CreatedAt:      time.Now(),
		}
		if sr.Error != "" {
			evalResult.ErrorMessage = &sr.Error
		}
		e.store.CreateResult(evalResult)
	}

	result.Total = len(suite.Scenarios)
	result.Duration = time.Since(start)

	if result.Failed == 0 && result.Skipped == 0 {
		result.Status = "passed"
	} else {
		result.Status = "failed"
	}

	// Update run record
	finished := time.Now()
	run.FinishedAt = &finished
	run.Passed = result.Passed
	run.Failed = result.Failed
	run.Skipped = result.Skipped
	run.Status = result.Status
	e.store.UpdateRun(run)

	return result, nil
}
