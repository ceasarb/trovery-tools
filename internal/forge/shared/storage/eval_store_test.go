package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestEvalStore(t *testing.T) *EvalStore {
	t.Helper()
	store, err := NewEvalStore(filepath.Join(t.TempDir(), "eval.db"))
	if err != nil {
		t.Fatalf("NewEvalStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestEvalStoreIdempotentMigrations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "eval.db")

	s1, err := NewEvalStore(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	s2, err := NewEvalStore(path)
	if err != nil {
		t.Fatalf("second open (idempotent): %v", err)
	}
	s2.Close()
}

func TestEvalRunCRUD(t *testing.T) {
	store := newTestEvalStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	run := &EvalRun{
		Source:     "server",
		TargetName: "weather-api",
		SuiteName:  "basic",
		StartedAt:  now,
		Status:     "running",
	}

	// Create
	if err := store.CreateRun(run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.ID == "" {
		t.Fatal("expected ID to be generated")
	}

	// Get
	got, err := store.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got == nil {
		t.Fatal("GetRun returned nil")
	}
	if got.Source != "server" {
		t.Errorf("Source = %q, want server", got.Source)
	}
	if got.TargetName != "weather-api" {
		t.Errorf("TargetName = %q", got.TargetName)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}

	// Update
	finished := now.Add(5 * time.Second)
	run.FinishedAt = &finished
	run.Status = "passed"
	run.TotalScenarios = 3
	run.Passed = 3
	if err := store.UpdateRun(run); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}

	got, _ = store.GetRun(run.ID)
	if got.Status != "passed" {
		t.Errorf("Status after update = %q, want passed", got.Status)
	}
	if got.Passed != 3 {
		t.Errorf("Passed = %d, want 3", got.Passed)
	}
}

func TestEvalRunGetNotFound(t *testing.T) {
	store := newTestEvalStore(t)
	got, err := store.GetRun("nonexistent")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent run")
	}
}

func TestEvalRunListWithFilter(t *testing.T) {
	store := newTestEvalStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	runs := []EvalRun{
		{Source: "server", TargetName: "api1", SuiteName: "s1", StartedAt: now, Status: "passed"},
		{Source: "server", TargetName: "api2", SuiteName: "s2", StartedAt: now.Add(time.Second), Status: "passed"},
		{Source: "agent", TargetName: "bot1", SuiteName: "s3", StartedAt: now.Add(2 * time.Second), Status: "failed"},
	}
	for i := range runs {
		if err := store.CreateRun(&runs[i]); err != nil {
			t.Fatalf("CreateRun[%d]: %v", i, err)
		}
	}

	// List all
	all, err := store.ListRuns("", 10)
	if err != nil {
		t.Fatalf("ListRuns all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListRuns all = %d, want 3", len(all))
	}

	// List by source
	servers, err := store.ListRuns("server", 10)
	if err != nil {
		t.Fatalf("ListRuns server: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("ListRuns server = %d, want 2", len(servers))
	}

	agents, err := store.ListRuns("agent", 10)
	if err != nil {
		t.Fatalf("ListRuns agent: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("ListRuns agent = %d, want 1", len(agents))
	}

	// Test limit
	limited, err := store.ListRuns("", 2)
	if err != nil {
		t.Fatalf("ListRuns limited: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("ListRuns limited = %d, want 2", len(limited))
	}
	// Should be most recent first
	if limited[0].TargetName != "bot1" {
		t.Errorf("first result = %q, want bot1 (most recent)", limited[0].TargetName)
	}
}

func TestEvalResultCRUD(t *testing.T) {
	store := newTestEvalStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	run := &EvalRun{
		Source: "server", TargetName: "api1", SuiteName: "basic",
		StartedAt: now, Status: "running",
	}
	store.CreateRun(run)

	dur := int64(150)
	errMsg := "assertion failed"
	assertions := `[{"name":"status","passed":false}]`

	result := &EvalResult{
		RunID:          run.ID,
		ScenarioName:   "test_get_weather",
		Status:         "failed",
		DurationMs:     &dur,
		ErrorMessage:   &errMsg,
		AssertionsJSON: &assertions,
		CreatedAt:      now,
	}
	if err := store.CreateResult(result); err != nil {
		t.Fatalf("CreateResult: %v", err)
	}
	if result.ID == "" {
		t.Fatal("expected ID to be generated")
	}

	results, err := store.GetResultsByRun(run.ID)
	if err != nil {
		t.Fatalf("GetResultsByRun: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].ScenarioName != "test_get_weather" {
		t.Errorf("ScenarioName = %q", results[0].ScenarioName)
	}
	if *results[0].DurationMs != 150 {
		t.Errorf("DurationMs = %d", *results[0].DurationMs)
	}
}

func TestEvalBaselineUpsert(t *testing.T) {
	store := newTestEvalStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	b := &EvalBaseline{
		Source:       "server",
		TargetName:   "api1",
		SuiteName:    "basic",
		ScenarioName: "test_get",
		BaselineJSON: `{"expected":"v1"}`,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Initial insert
	if err := store.SetBaseline(b); err != nil {
		t.Fatalf("SetBaseline insert: %v", err)
	}

	got, err := store.GetBaseline("server", "api1", "basic", "test_get")
	if err != nil {
		t.Fatalf("GetBaseline: %v", err)
	}
	if got == nil {
		t.Fatal("expected baseline")
	}
	if got.BaselineJSON != `{"expected":"v1"}` {
		t.Errorf("BaselineJSON = %q", got.BaselineJSON)
	}

	// Upsert with new data
	b2 := &EvalBaseline{
		Source:       "server",
		TargetName:   "api1",
		SuiteName:    "basic",
		ScenarioName: "test_get",
		BaselineJSON: `{"expected":"v2"}`,
		CreatedAt:    now,
		UpdatedAt:    now.Add(time.Minute),
	}
	if err := store.SetBaseline(b2); err != nil {
		t.Fatalf("SetBaseline upsert: %v", err)
	}

	got, _ = store.GetBaseline("server", "api1", "basic", "test_get")
	if got.BaselineJSON != `{"expected":"v2"}` {
		t.Errorf("BaselineJSON after upsert = %q, want v2", got.BaselineJSON)
	}
}

func TestEvalBaselineNotFound(t *testing.T) {
	store := newTestEvalStore(t)
	got, err := store.GetBaseline("x", "y", "z", "w")
	if err != nil {
		t.Fatalf("GetBaseline: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent baseline")
	}
}
