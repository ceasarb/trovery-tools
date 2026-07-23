package session

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus represents the state of a session.
type SessionStatus string

const (
	StatusActive    SessionStatus = "active"
	StatusCompleted SessionStatus = "completed"
	StatusAborted   SessionStatus = "aborted"
)

// GitSnapshot captures git state at session start.
type GitSnapshot struct {
	HeadSHA    string   `json:"head_sha"`
	Branch     string   `json:"branch"`
	DirtyFiles []string `json:"dirty_files"`
}

// FileChange represents a single file modification during a session.
type FileChange struct {
	Path       string `json:"path"`
	ChangeType string `json:"change_type"`            // added | modified | deleted | renamed
	Source     string `json:"source,omitempty"`        // committed | staged | unstaged | untracked
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
}

// ToolRun records one invocation of a tool during a session.
type ToolRun struct {
	Tool      string    `json:"tool"`
	Command   string    `json:"command"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	ExitCode  int       `json:"exit_code"`
}

// PolicyViolation represents a single policy check failure.
type PolicyViolation struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"` // error | warning
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
}

// TokenUsage records token consumption for a tool run.
type TokenUsage struct {
	Tool             string  `json:"tool"`
	Model            string  `json:"model,omitempty"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// Session is the core data model for a governed work period.
type Session struct {
	ID              string            `json:"id"`
	Status          SessionStatus     `json:"status"`
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time,omitempty"`
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
	GitSnapshot     *GitSnapshot      `json:"git_snapshot,omitempty"`
	ToolRuns        []ToolRun         `json:"tool_runs"`
	FileChanges     []FileChange      `json:"file_changes"`
	Violations      []PolicyViolation `json:"violations"`
	TokenUsage      []TokenUsage      `json:"token_usage"`
	ConfigHash      string            `json:"config_hash,omitempty"`
}

// NewSession creates a new active session with a generated ID.
func NewSession() *Session {
	return &Session{
		ID:          uuid.New().String()[:12],
		Status:      StatusActive,
		StartTime:   time.Now(),
		ToolRuns:    []ToolRun{},
		FileChanges: []FileChange{},
		Violations:  []PolicyViolation{},
		TokenUsage:  []TokenUsage{},
	}
}

// HasErrors returns true if the session has any error-level violations.
func (s *Session) HasErrors() bool {
	for _, v := range s.Violations {
		if v.Severity == "error" {
			return true
		}
	}
	return false
}

// TotalCost returns the sum of all token usage costs.
func (s *Session) TotalCost() float64 {
	var total float64
	for _, u := range s.TokenUsage {
		total += u.EstimatedCostUSD
	}
	return total
}

// Finalize marks the session as completed and computes duration.
func (s *Session) Finalize() {
	s.Status = StatusCompleted
	s.EndTime = time.Now()
	s.DurationSeconds = s.EndTime.Sub(s.StartTime).Seconds()
}

// Abort marks the session as aborted.
func (s *Session) Abort() {
	s.Status = StatusAborted
	s.EndTime = time.Now()
	s.DurationSeconds = s.EndTime.Sub(s.StartTime).Seconds()
}
