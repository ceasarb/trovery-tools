package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
)

func testConfig() *config.VigilConfig {
	cfg := config.DefaultConfig()
	cfg.Name = "test"
	return &cfg
}

func setupManagerWithGit(t *testing.T) (*SessionManager, string) {
	t.Helper()
	dir := t.TempDir()

	// Init git repo.
	for _, args := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.CombinedOutput()
	}
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.CombinedOutput()
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	cmd.CombinedOutput()

	cfg := testConfig()
	mgr := NewSessionManager(cfg, dir)
	return mgr, dir
}

func TestManager_StartStop(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	s, err := mgr.Start()
	if err != nil {
		t.Fatalf("start error: %v", err)
	}
	if s.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if s.Status != StatusActive {
		t.Errorf("expected active status, got %q", s.Status)
	}

	stopped, err := mgr.Stop(nil, nil)
	if err != nil {
		t.Fatalf("stop error: %v", err)
	}
	if stopped.Status != StatusCompleted {
		t.Errorf("expected completed status, got %q", stopped.Status)
	}
	if stopped.DurationSeconds <= 0 {
		t.Error("expected positive duration")
	}
}

func TestManager_DoubleStart(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	_, err := mgr.Start()
	if err != nil {
		t.Fatalf("first start error: %v", err)
	}

	_, err = mgr.Start()
	if err != ErrActiveSessionExists {
		t.Errorf("expected ErrActiveSessionExists, got %v", err)
	}
}

func TestManager_StopWithoutStart(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	_, err := mgr.Stop(nil, nil)
	if err != ErrNoActiveSession {
		t.Errorf("expected ErrNoActiveSession, got %v", err)
	}
}

func TestManager_Abort(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	mgr.Start()

	s, err := mgr.Abort()
	if err != nil {
		t.Fatalf("abort error: %v", err)
	}
	if s.Status != StatusAborted {
		t.Errorf("expected aborted status, got %q", s.Status)
	}

	// Should be able to start again after abort.
	_, err = mgr.Start()
	if err != nil {
		t.Fatalf("start after abort error: %v", err)
	}
}

func TestManager_ForceStop(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	mgr.Start()

	s, err := mgr.ForceStop()
	if err != nil {
		t.Fatalf("force stop error: %v", err)
	}
	if s == nil {
		t.Fatal("expected session from force stop")
	}
	if s.Status != StatusAborted {
		t.Errorf("expected aborted, got %q", s.Status)
	}
}

func TestManager_AddToolRun(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	mgr.Start()
	err := mgr.AddToolRun(ToolRun{Tool: "claude-code", Command: "claude"})
	if err != nil {
		t.Fatalf("add tool run error: %v", err)
	}

	active, _ := mgr.GetActive()
	if len(active.ToolRuns) != 1 {
		t.Errorf("expected 1 tool run, got %d", len(active.ToolRuns))
	}
}

func TestManager_ListSessions(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	// Start and stop 2 sessions.
	mgr.Start()
	mgr.Stop(nil, nil)

	mgr.Start()
	mgr.Stop(nil, nil)

	sessions, err := mgr.ListSessions(10)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestManager_GetSession(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	s, _ := mgr.Start()
	mgr.Stop(nil, nil)

	loaded, err := mgr.GetSession(s.ID)
	if err != nil {
		t.Fatalf("get session error: %v", err)
	}
	if loaded.ID != s.ID {
		t.Errorf("expected ID %q, got %q", s.ID, loaded.ID)
	}
}

func TestManager_GetSessionNotFound(t *testing.T) {
	mgr, _ := setupManagerWithGit(t)

	_, err := mgr.GetSession("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}
