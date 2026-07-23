package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
)

const (
	activeSessionFile = "active_session"
	staleThreshold    = 24 * time.Hour
)

var (
	ErrActiveSessionExists = errors.New("a session is already active — run `demi vigil stop` first")
	ErrNoActiveSession     = errors.New("no active session — run `demi vigil start` first")
	ErrSessionNotFound     = errors.New("session not found")
	ErrStaleSession        = errors.New("stale session detected (24h+) — use --force to clear")
)

// SessionManager manages the session lifecycle.
type SessionManager struct {
	Config      *config.VigilConfig
	ProjectRoot string
	sessionsDir string
}

// NewSessionManager creates a new manager.
func NewSessionManager(cfg *config.VigilConfig, projectRoot string) *SessionManager {
	return &SessionManager{
		Config:      cfg,
		ProjectRoot: projectRoot,
		sessionsDir: filepath.Join(projectRoot, cfg.Tracking.SessionDir),
	}
}

// Start creates a new active session.
func (m *SessionManager) Start() (*Session, error) {
	// Check for existing active session.
	existing, _ := m.GetActive()
	if existing != nil {
		return nil, ErrActiveSessionExists
	}

	s := NewSession()

	// Capture git snapshot.
	snap, err := Snapshot(m.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("capturing git snapshot: %w", err)
	}
	s.GitSnapshot = snap

	// Save session.
	if err := SaveSession(m.sessionsDir, s); err != nil {
		return nil, err
	}

	// Write active session marker.
	if err := m.writeActiveMarker(s.ID); err != nil {
		return nil, err
	}

	return s, nil
}

// GetActive returns the currently active session, or nil if none.
func (m *SessionManager) GetActive() (*Session, error) {
	id, err := m.readActiveMarker()
	if err != nil {
		return nil, nil
	}

	s, err := LoadSession(m.sessionsDir, id)
	if err != nil {
		// Corrupted active session — clean up.
		m.clearActiveMarker()
		return nil, nil
	}

	if s.Status != StatusActive {
		m.clearActiveMarker()
		return nil, nil
	}

	return s, nil
}

// Stop finalizes the active session with file changes and violations.
func (m *SessionManager) Stop(changes []FileChange, violations []PolicyViolation) (*Session, error) {
	s, err := m.GetActive()
	if err != nil || s == nil {
		return nil, ErrNoActiveSession
	}

	s.FileChanges = changes
	s.Violations = violations
	s.Finalize()

	if err := SaveSession(m.sessionsDir, s); err != nil {
		return nil, err
	}

	m.clearActiveMarker()
	return s, nil
}

// Abort force-stops the active session without policy checks.
func (m *SessionManager) Abort() (*Session, error) {
	s, err := m.GetActive()
	if err != nil || s == nil {
		return nil, ErrNoActiveSession
	}

	s.Abort()

	if err := SaveSession(m.sessionsDir, s); err != nil {
		return nil, err
	}

	m.clearActiveMarker()
	return s, nil
}

// ForceStop aborts a potentially stale session.
func (m *SessionManager) ForceStop() (*Session, error) {
	id, err := m.readActiveMarker()
	if err != nil {
		return nil, ErrNoActiveSession
	}

	s, err := LoadSession(m.sessionsDir, id)
	if err != nil {
		// Can't load — just clear the marker.
		m.clearActiveMarker()
		return nil, nil
	}

	s.Abort()
	SaveSession(m.sessionsDir, s)
	m.clearActiveMarker()
	return s, nil
}

// AddToolRun adds a tool run record to the active session.
func (m *SessionManager) AddToolRun(run ToolRun) error {
	s, err := m.GetActive()
	if err != nil || s == nil {
		return ErrNoActiveSession
	}

	s.ToolRuns = append(s.ToolRuns, run)
	return SaveSession(m.sessionsDir, s)
}

// ListSessions returns recent sessions.
func (m *SessionManager) ListSessions(limit int) ([]*Session, error) {
	return ListSessions(m.sessionsDir, limit)
}

// GetSession loads a specific session by ID.
func (m *SessionManager) GetSession(id string) (*Session, error) {
	s, err := LoadSession(m.sessionsDir, id)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	return s, nil
}

// IsStale checks if the active session has exceeded the stale threshold.
func (m *SessionManager) IsStale() bool {
	s, _ := m.GetActive()
	if s == nil {
		return false
	}
	return time.Since(s.StartTime) > staleThreshold
}

// SessionsDir returns the sessions directory path.
func (m *SessionManager) SessionsDir() string {
	return m.sessionsDir
}

func (m *SessionManager) activeMarkerPath() string {
	return filepath.Join(m.ProjectRoot, ".demi/vigil", activeSessionFile)
}

func (m *SessionManager) writeActiveMarker(id string) error {
	dir := filepath.Dir(m.activeMarkerPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(m.activeMarkerPath(), []byte(id), 0644)
}

func (m *SessionManager) readActiveMarker() (string, error) {
	data, err := os.ReadFile(m.activeMarkerPath())
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", fmt.Errorf("empty active session marker")
	}
	return id, nil
}

func (m *SessionManager) clearActiveMarker() {
	os.Remove(m.activeMarkerPath())
}
