package session

import (
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkSaveSession(b *testing.B) {
	dir := b.TempDir()
	s := NewSession()
	s.ToolRuns = []ToolRun{{Tool: "claude-code", Command: "claude", StartTime: time.Now()}}
	s.FileChanges = []FileChange{{Path: "main.go", ChangeType: "modified", Additions: 10, Deletions: 3}}
	s.Finalize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ID = "bench-" + string(rune(i%1000000+'0'))
		SaveSession(dir, s)
	}
}

func BenchmarkLoadSession(b *testing.B) {
	dir := b.TempDir()
	s := NewSession()
	s.Finalize()
	SaveSession(dir, s)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadSession(dir, s.ID)
	}
}

func BenchmarkSQLiteIndex(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer idx.Close()

	s := &Session{
		ID: "bench", Status: StatusCompleted, StartTime: time.Now(),
		ToolRuns:    []ToolRun{{Tool: "claude-code"}},
		FileChanges: []FileChange{{Path: "a.go"}},
		TokenUsage:  []TokenUsage{{TotalTokens: 1000}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ID = "bench-" + string(rune(i%1000000+'0'))
		idx.Index(s)
	}
}

func BenchmarkSQLiteQuery(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	idx, err := OpenSQLiteIndex(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer idx.Close()

	// Seed with 100 sessions.
	for i := 0; i < 100; i++ {
		s := &Session{
			ID: "s-" + string(rune(i+'0')), Status: StatusCompleted,
			StartTime: time.Now().Add(-time.Duration(i) * time.Hour),
		}
		idx.Index(s)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Query(QueryOpts{Limit: 20})
	}
}
