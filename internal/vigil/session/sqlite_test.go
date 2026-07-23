package session

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteIndex_OpenAndClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	if err := idx.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}
}

func TestSQLiteIndex_IndexAndQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer idx.Close()

	s := &Session{
		ID:              "test-123",
		Status:          StatusCompleted,
		StartTime:       time.Now().Add(-1 * time.Hour),
		EndTime:         time.Now(),
		DurationSeconds: 3600,
		ToolRuns: []ToolRun{
			{Tool: "claude-code", Command: "claude", StartTime: time.Now()},
		},
		FileChanges: []FileChange{
			{Path: "main.go", ChangeType: "modified"},
		},
		Violations: []PolicyViolation{
			{Rule: "secrets", Severity: "error", Message: "secret found"},
		},
		TokenUsage: []TokenUsage{
			{Tool: "claude-code", TotalTokens: 1000, EstimatedCostUSD: 0.05},
		},
	}

	if err := idx.Index(s); err != nil {
		t.Fatalf("failed to index: %v", err)
	}

	// Query all.
	results, err := idx.Query(QueryOpts{Limit: 10})
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "test-123" {
		t.Errorf("expected id test-123, got %q", results[0].ID)
	}
	if results[0].ViolationsCount != 1 {
		t.Errorf("expected 1 violation, got %d", results[0].ViolationsCount)
	}
	if results[0].TotalTokens != 1000 {
		t.Errorf("expected 1000 tokens, got %d", results[0].TotalTokens)
	}
}

func TestSQLiteIndex_QueryByTool(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer idx.Close()

	// Index two sessions with different tools.
	s1 := &Session{
		ID: "s1", Status: StatusCompleted, StartTime: time.Now(),
		ToolRuns: []ToolRun{{Tool: "claude-code"}},
	}
	s2 := &Session{
		ID: "s2", Status: StatusCompleted, StartTime: time.Now(),
		ToolRuns: []ToolRun{{Tool: "codex"}},
	}
	idx.Index(s1)
	idx.Index(s2)

	results, _ := idx.Query(QueryOpts{Tool: "claude-code", Limit: 10})
	if len(results) != 1 || results[0].ID != "s1" {
		t.Errorf("expected s1 for claude-code filter, got %v", results)
	}
}

func TestSQLiteIndex_QueryHasViolations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer idx.Close()

	clean := &Session{ID: "clean", Status: StatusCompleted, StartTime: time.Now()}
	dirty := &Session{
		ID: "dirty", Status: StatusCompleted, StartTime: time.Now(),
		Violations: []PolicyViolation{{Rule: "x", Severity: "error"}},
	}
	idx.Index(clean)
	idx.Index(dirty)

	results, _ := idx.Query(QueryOpts{HasViolations: true, Limit: 10})
	if len(results) != 1 || results[0].ID != "dirty" {
		t.Errorf("expected dirty session, got %v", results)
	}
}

func TestSQLiteIndex_Aggregate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer idx.Close()

	now := time.Now()
	idx.Index(&Session{
		ID: "s1", Status: StatusCompleted, StartTime: now,
		DurationSeconds: 100, FileChanges: []FileChange{{Path: "a.go"}},
		TokenUsage: []TokenUsage{{TotalTokens: 500, EstimatedCostUSD: 0.01}},
	})
	idx.Index(&Session{
		ID: "s2", Status: StatusCompleted, StartTime: now,
		DurationSeconds: 200, FileChanges: []FileChange{{Path: "b.go"}, {Path: "c.go"}},
		TokenUsage: []TokenUsage{{TotalTokens: 1000, EstimatedCostUSD: 0.05}},
	})

	stats, err := idx.Aggregate("status", now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("failed to aggregate: %v", err)
	}

	completed, ok := stats["completed"]
	if !ok {
		t.Fatal("expected 'completed' group")
	}
	if completed.Count != 2 {
		t.Errorf("expected count 2, got %d", completed.Count)
	}
	if completed.TotalTokens != 1500 {
		t.Errorf("expected 1500 tokens, got %d", completed.TotalTokens)
	}
}

func TestSQLiteIndex_Upsert(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer idx.Close()

	s := &Session{ID: "upsert-test", Status: StatusActive, StartTime: time.Now()}
	idx.Index(s)

	// Update.
	s.Status = StatusCompleted
	s.DurationSeconds = 60
	idx.Index(s)

	results, _ := idx.Query(QueryOpts{Limit: 10})
	if len(results) != 1 {
		t.Fatalf("expected 1 result after upsert, got %d", len(results))
	}
	if results[0].Status != "completed" {
		t.Errorf("expected completed after upsert, got %q", results[0].Status)
	}
}

func TestSQLiteIndex_RebuildIndex(t *testing.T) {
	dir := t.TempDir()

	// Create a session JSON file.
	s := NewSession()
	s.Finalize()
	SaveSession(dir, s)

	dbPath := filepath.Join(t.TempDir(), "rebuild.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer idx.Close()

	count, err := idx.RebuildIndex(dir)
	if err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 session rebuilt, got %d", count)
	}
}
