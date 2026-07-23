package reporting

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ceasarb/demigo-tools/internal/vigil/session"
)

func setupTestIndex(t *testing.T) *session.SQLiteIndex {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := session.OpenSQLiteIndex(dbPath)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	now := time.Now()
	idx.Index(&session.Session{
		ID: "s1", Status: session.StatusCompleted, StartTime: now.Add(-1 * time.Hour),
		DurationSeconds: 3600,
		ToolRuns:        []session.ToolRun{{Tool: "claude-code"}},
		FileChanges:     []session.FileChange{{Path: "a.go"}},
		TokenUsage:      []session.TokenUsage{{TotalTokens: 1000, EstimatedCostUSD: 0.05}},
	})
	idx.Index(&session.Session{
		ID: "s2", Status: session.StatusCompleted, StartTime: now.Add(-2 * time.Hour),
		DurationSeconds: 1800,
		ToolRuns:        []session.ToolRun{{Tool: "codex"}},
		FileChanges:     []session.FileChange{{Path: "b.go"}, {Path: "c.go"}},
		TokenUsage:      []session.TokenUsage{{TotalTokens: 500, EstimatedCostUSD: 0.02}},
	})

	return idx
}

func TestReportGenerator_Terminal(t *testing.T) {
	idx := setupTestIndex(t)
	defer idx.Close()

	gen := NewReportGenerator(idx)
	output, err := gen.Generate(ReportOptions{Days: 30, GroupBy: "status", Format: "terminal"})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(output, "Usage Report") {
		t.Errorf("expected report header, got: %s", output)
	}
	if !strings.Contains(output, "Sessions:") {
		t.Errorf("expected sessions count in output")
	}
}

func TestReportGenerator_JSON(t *testing.T) {
	idx := setupTestIndex(t)
	defer idx.Close()

	gen := NewReportGenerator(idx)
	output, err := gen.Generate(ReportOptions{Days: 30, Format: "json"})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(output, `"generated_at"`) {
		t.Errorf("expected JSON output, got: %s", output)
	}
}

func TestReportGenerator_CSV(t *testing.T) {
	idx := setupTestIndex(t)
	defer idx.Close()

	gen := NewReportGenerator(idx)
	output, err := gen.Generate(ReportOptions{Days: 30, Format: "csv"})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(output, "Group,Sessions") {
		t.Errorf("expected CSV header, got: %s", output)
	}
}

func TestReportGenerator_EmptyData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	idx, _ := session.OpenSQLiteIndex(dbPath)
	defer idx.Close()

	gen := NewReportGenerator(idx)
	output, err := gen.Generate(ReportOptions{Days: 30, Format: "terminal"})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(output, "No sessions found") {
		t.Errorf("expected empty message, got: %s", output)
	}
}
