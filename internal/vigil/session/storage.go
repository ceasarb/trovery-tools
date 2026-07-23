package session

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SaveSession writes a session to a JSON file.
func SaveSession(sessionsDir string, s *Session) error {
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("creating sessions directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	path := filepath.Join(sessionsDir, s.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	return nil
}

// LoadSession reads a session from its JSON file.
func LoadSession(sessionsDir, id string) (*Session, error) {
	path := filepath.Join(sessionsDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading session %s: %w", id, err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing session %s: %w", id, err)
	}

	return &s, nil
}

// ListSessions returns recent sessions sorted newest-first.
func ListSessions(sessionsDir string, limit int) ([]*Session, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		s, err := LoadSession(sessionsDir, id)
		if err != nil {
			slog.Warn("skipping corrupted session", "id", id, "error", err)
			continue
		}
		sessions = append(sessions, s)
	}

	// Sort newest first.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}
