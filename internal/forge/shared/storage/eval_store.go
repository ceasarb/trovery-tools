package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EvalRun represents a single evaluation run.
type EvalRun struct {
	ID             string
	Source         string // "server" or "agent"
	TargetName     string
	SuiteName      string
	StartedAt      time.Time
	FinishedAt     *time.Time
	TotalScenarios int
	Passed         int
	Failed         int
	Skipped        int
	Status         string // running, passed, failed, error
}

// EvalResult represents the outcome of a single scenario within a run.
type EvalResult struct {
	ID             string
	RunID          string
	ScenarioName   string
	Status         string // passed, failed, skipped, error
	DurationMs     *int64
	ErrorMessage   *string
	AssertionsJSON *string
	CreatedAt      time.Time
}

// EvalBaseline represents the expected results snapshot for a scenario.
type EvalBaseline struct {
	ID           string
	Source       string
	TargetName   string
	SuiteName    string
	ScenarioName string
	BaselineJSON string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var evalMigrations = []Migration{
	{
		Version:     1,
		Description: "create eval_runs table",
		SQL: `CREATE TABLE eval_runs (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			target_name TEXT NOT NULL,
			suite_name TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			total_scenarios INTEGER DEFAULT 0,
			passed INTEGER DEFAULT 0,
			failed INTEGER DEFAULT 0,
			skipped INTEGER DEFAULT 0,
			status TEXT DEFAULT 'running'
		)`,
	},
	{
		Version:     2,
		Description: "create eval_results table",
		SQL: `CREATE TABLE eval_results (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES eval_runs(id),
			scenario_name TEXT NOT NULL,
			status TEXT NOT NULL,
			duration_ms INTEGER,
			error_message TEXT,
			assertions_json TEXT,
			created_at DATETIME NOT NULL
		)`,
	},
	{
		Version:     3,
		Description: "create eval_baselines table",
		SQL: `CREATE TABLE eval_baselines (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			target_name TEXT NOT NULL,
			suite_name TEXT NOT NULL,
			scenario_name TEXT NOT NULL,
			baseline_json TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(source, target_name, suite_name, scenario_name)
		)`,
	},
	{
		Version:     4,
		Description: "add indexes for eval queries",
		SQL: `CREATE INDEX IF NOT EXISTS idx_eval_runs_source ON eval_runs(source);
			  CREATE INDEX IF NOT EXISTS idx_eval_runs_target ON eval_runs(target_name);
			  CREATE INDEX IF NOT EXISTS idx_eval_runs_started ON eval_runs(started_at);
			  CREATE INDEX IF NOT EXISTS idx_eval_results_run ON eval_results(run_id);
			  CREATE INDEX IF NOT EXISTS idx_eval_results_scenario ON eval_results(scenario_name)`,
	},
}

// EvalStore provides CRUD operations for eval runs, results, and baselines.
type EvalStore struct {
	db *DB
}

// NewEvalStore opens a SQLite database at dbPath and runs eval migrations.
func NewEvalStore(dbPath string) (*EvalStore, error) {
	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(evalMigrations); err != nil {
		db.Close()
		return nil, fmt.Errorf("eval migrations: %w", err)
	}
	return &EvalStore{db: db}, nil
}

// Close closes the underlying database.
func (s *EvalStore) Close() error {
	return s.db.Close()
}

// CreateRun inserts a new eval run. If ID is empty, a UUID is generated.
func (s *EvalStore) CreateRun(run *EvalRun) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	_, err := s.db.Conn().Exec(
		`INSERT INTO eval_runs (id, source, target_name, suite_name, started_at, finished_at, total_scenarios, passed, failed, skipped, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.Source, run.TargetName, run.SuiteName, run.StartedAt, run.FinishedAt,
		run.TotalScenarios, run.Passed, run.Failed, run.Skipped, run.Status,
	)
	if err != nil {
		return fmt.Errorf("insert eval_run: %w", err)
	}
	return nil
}

// UpdateRun updates an existing eval run.
func (s *EvalStore) UpdateRun(run *EvalRun) error {
	_, err := s.db.Conn().Exec(
		`UPDATE eval_runs SET source=?, target_name=?, suite_name=?, started_at=?, finished_at=?,
		 total_scenarios=?, passed=?, failed=?, skipped=?, status=? WHERE id=?`,
		run.Source, run.TargetName, run.SuiteName, run.StartedAt, run.FinishedAt,
		run.TotalScenarios, run.Passed, run.Failed, run.Skipped, run.Status, run.ID,
	)
	if err != nil {
		return fmt.Errorf("update eval_run: %w", err)
	}
	return nil
}

// GetRun retrieves an eval run by ID.
func (s *EvalStore) GetRun(id string) (*EvalRun, error) {
	var run EvalRun
	err := s.db.Conn().QueryRow(
		`SELECT id, source, target_name, suite_name, started_at, finished_at,
		 total_scenarios, passed, failed, skipped, status FROM eval_runs WHERE id=?`, id,
	).Scan(
		&run.ID, &run.Source, &run.TargetName, &run.SuiteName, &run.StartedAt, &run.FinishedAt,
		&run.TotalScenarios, &run.Passed, &run.Failed, &run.Skipped, &run.Status,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get eval_run: %w", err)
	}
	return &run, nil
}

// ListRuns returns eval runs ordered by started_at descending.
// If source is non-empty, results are filtered by source.
func (s *EvalStore) ListRuns(source string, limit int) ([]EvalRun, error) {
	var rows *sql.Rows
	var err error

	if source != "" {
		rows, err = s.db.Conn().Query(
			`SELECT id, source, target_name, suite_name, started_at, finished_at,
			 total_scenarios, passed, failed, skipped, status
			 FROM eval_runs WHERE source=? ORDER BY started_at DESC LIMIT ?`,
			source, limit,
		)
	} else {
		rows, err = s.db.Conn().Query(
			`SELECT id, source, target_name, suite_name, started_at, finished_at,
			 total_scenarios, passed, failed, skipped, status
			 FROM eval_runs ORDER BY started_at DESC LIMIT ?`,
			limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list eval_runs: %w", err)
	}
	defer rows.Close()

	var runs []EvalRun
	for rows.Next() {
		var r EvalRun
		if err := rows.Scan(
			&r.ID, &r.Source, &r.TargetName, &r.SuiteName, &r.StartedAt, &r.FinishedAt,
			&r.TotalScenarios, &r.Passed, &r.Failed, &r.Skipped, &r.Status,
		); err != nil {
			return nil, fmt.Errorf("scan eval_run: %w", err)
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// CreateResult inserts a new eval result. If ID is empty, a UUID is generated.
func (s *EvalStore) CreateResult(result *EvalResult) error {
	if result.ID == "" {
		result.ID = uuid.New().String()
	}
	_, err := s.db.Conn().Exec(
		`INSERT INTO eval_results (id, run_id, scenario_name, status, duration_ms, error_message, assertions_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.ID, result.RunID, result.ScenarioName, result.Status,
		result.DurationMs, result.ErrorMessage, result.AssertionsJSON, result.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert eval_result: %w", err)
	}
	return nil
}

// GetResultsByRun returns all eval results for a given run ID.
func (s *EvalStore) GetResultsByRun(runID string) ([]EvalResult, error) {
	rows, err := s.db.Conn().Query(
		`SELECT id, run_id, scenario_name, status, duration_ms, error_message, assertions_json, created_at
		 FROM eval_results WHERE run_id=? ORDER BY created_at`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list eval_results: %w", err)
	}
	defer rows.Close()

	var results []EvalResult
	for rows.Next() {
		var r EvalResult
		if err := rows.Scan(
			&r.ID, &r.RunID, &r.ScenarioName, &r.Status,
			&r.DurationMs, &r.ErrorMessage, &r.AssertionsJSON, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan eval_result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SetBaseline upserts an eval baseline. If ID is empty, a UUID is generated.
func (s *EvalStore) SetBaseline(baseline *EvalBaseline) error {
	if baseline.ID == "" {
		baseline.ID = uuid.New().String()
	}
	_, err := s.db.Conn().Exec(
		`INSERT INTO eval_baselines (id, source, target_name, suite_name, scenario_name, baseline_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(source, target_name, suite_name, scenario_name)
		 DO UPDATE SET baseline_json=excluded.baseline_json, updated_at=excluded.updated_at`,
		baseline.ID, baseline.Source, baseline.TargetName, baseline.SuiteName,
		baseline.ScenarioName, baseline.BaselineJSON, baseline.CreatedAt, baseline.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert eval_baseline: %w", err)
	}
	return nil
}

// GetBaseline retrieves a baseline by its composite key.
func (s *EvalStore) GetBaseline(source, targetName, suiteName, scenarioName string) (*EvalBaseline, error) {
	var b EvalBaseline
	err := s.db.Conn().QueryRow(
		`SELECT id, source, target_name, suite_name, scenario_name, baseline_json, created_at, updated_at
		 FROM eval_baselines WHERE source=? AND target_name=? AND suite_name=? AND scenario_name=?`,
		source, targetName, suiteName, scenarioName,
	).Scan(
		&b.ID, &b.Source, &b.TargetName, &b.SuiteName,
		&b.ScenarioName, &b.BaselineJSON, &b.CreatedAt, &b.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get eval_baseline: %w", err)
	}
	return &b, nil
}
