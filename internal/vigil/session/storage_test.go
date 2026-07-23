package session

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadSession(t *testing.T) {
	dir := t.TempDir()

	s := NewSession()
	s.GitSnapshot = &GitSnapshot{HeadSHA: "abc123", Branch: "main"}
	s.ToolRuns = append(s.ToolRuns, ToolRun{
		Tool: "claude-code", Command: "claude", StartTime: time.Now(),
	})

	if err := SaveSession(dir, s); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadSession(dir, s.ID)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.ID != s.ID {
		t.Errorf("expected ID %q, got %q", s.ID, loaded.ID)
	}
	if loaded.Status != StatusActive {
		t.Errorf("expected status active, got %q", loaded.Status)
	}
	if loaded.GitSnapshot.HeadSHA != "abc123" {
		t.Errorf("expected head sha 'abc123', got %q", loaded.GitSnapshot.HeadSHA)
	}
	if len(loaded.ToolRuns) != 1 {
		t.Errorf("expected 1 tool run, got %d", len(loaded.ToolRuns))
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()

	// Create 3 sessions with staggered start times.
	for i := 0; i < 3; i++ {
		s := NewSession()
		s.StartTime = time.Now().Add(time.Duration(i) * time.Minute)
		SaveSession(dir, s)
	}

	sessions, err := ListSessions(dir, 0)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// Should be newest first.
	if sessions[0].StartTime.Before(sessions[1].StartTime) {
		t.Error("expected sessions sorted newest-first")
	}
}

func TestListSessions_WithLimit(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 5; i++ {
		s := NewSession()
		SaveSession(dir, s)
	}

	sessions, err := ListSessions(dir, 2)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions with limit, got %d", len(sessions))
	}
}

func TestListSessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	sessions, err := ListSessions(dir, 0)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_NonexistentDir(t *testing.T) {
	sessions, err := ListSessions(filepath.Join(t.TempDir(), "nope"), 0)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil sessions, got %v", sessions)
	}
}

func TestLoadSession_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadSession(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}
