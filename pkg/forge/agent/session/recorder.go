package session

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ceasarb/trovery-tools/pkg/forge/shared/storage"
	"github.com/google/uuid"
)

const defaultRetention = 30 * 24 * time.Hour // 30 days

// ToolCallRecord captures a single tool invocation for recording.
type ToolCallRecord struct {
	ToolName   string
	ServerName string
	Arguments  map[string]any
	Result     string
	Error      string
	DurationMs int64
}

// Recorder manages session recording to SQLite.
// When disabled (created via NewDisabled), all methods are no-ops.
type Recorder struct {
	mu          sync.Mutex
	store       *storage.SessionStore
	session     *storage.Session
	turnNum     int
	currentTurn string // ID of the current turn
	enabled     bool
}

// New creates an enabled Recorder that writes to the given SessionStore.
func New(store *storage.SessionStore, agentName, providerName, model string) *Recorder {
	sess := &storage.Session{
		ID:        uuid.New().String(),
		AgentName: agentName,
		Provider:  providerName,
		Model:     model,
		StartedAt: time.Now(),
	}
	// Best-effort create — if it fails, we'll catch it on the first record call.
	_ = store.CreateSession(sess)

	return &Recorder{
		store:   store,
		session: sess,
		enabled: true,
	}
}

// NewDisabled creates a no-op Recorder. All methods return nil.
func NewDisabled() *Recorder {
	return &Recorder{enabled: false}
}

// IsEnabled reports whether recording is active.
func (r *Recorder) IsEnabled() bool {
	return r.enabled
}

// SessionID returns the current session's ID, or empty if disabled.
func (r *Recorder) SessionID() string {
	if !r.enabled {
		return ""
	}
	return r.session.ID
}

// CurrentTurnID returns the ID of the most recently recorded turn.
func (r *Recorder) CurrentTurnID() string {
	if !r.enabled {
		return ""
	}
	return r.currentTurn
}

// RecordUserTurn records a user message as a new turn.
func (r *Recorder) RecordUserTurn(content string) error {
	if !r.enabled {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.turnNum++
	turn := &storage.SessionTurn{
		ID:         uuid.New().String(),
		SessionID:  r.session.ID,
		TurnNumber: r.turnNum,
		Role:       "user",
		Content:    content,
		CreatedAt:  time.Now(),
	}
	r.currentTurn = turn.ID

	if err := r.store.CreateTurn(turn); err != nil {
		return err
	}

	r.session.TotalTurns = r.turnNum
	return r.store.UpdateSession(r.session)
}

// RecordAssistantTurn records an assistant response with token/cost data.
func (r *Recorder) RecordAssistantTurn(content string, tokensIn, tokensOut int, costUSD float64) error {
	if !r.enabled {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.turnNum++
	turn := &storage.SessionTurn{
		ID:         uuid.New().String(),
		SessionID:  r.session.ID,
		TurnNumber: r.turnNum,
		Role:       "assistant",
		Content:    content,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		CostUSD:    costUSD,
		CreatedAt:  time.Now(),
	}
	r.currentTurn = turn.ID

	if err := r.store.CreateTurn(turn); err != nil {
		return err
	}

	// Update session totals
	r.session.TotalTurns = r.turnNum
	r.session.TotalTokensIn += tokensIn
	r.session.TotalTokensOut += tokensOut
	r.session.TotalCostUSD += costUSD
	return r.store.UpdateSession(r.session)
}

// RecordToolCall records a tool invocation linked to the current turn.
func (r *Recorder) RecordToolCall(turnID string, call ToolCallRecord) error {
	if !r.enabled {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var argsJSON *string
	if call.Arguments != nil {
		data, err := json.Marshal(call.Arguments)
		if err == nil {
			s := string(data)
			argsJSON = &s
		}
	}

	var resultJSON *string
	if call.Result != "" {
		resultJSON = &call.Result
	}

	var errStr *string
	if call.Error != "" {
		errStr = &call.Error
	}

	var durMs *int64
	if call.DurationMs > 0 {
		durMs = &call.DurationMs
	}

	tc := &storage.SessionToolCall{
		TurnID:        turnID,
		SessionID:     r.session.ID,
		ToolName:      call.ToolName,
		ServerName:    call.ServerName,
		ArgumentsJSON: argsJSON,
		ResultJSON:    resultJSON,
		DurationMs:    durMs,
		Error:         errStr,
		CreatedAt:     time.Now(),
	}

	return r.store.CreateToolCall(tc)
}

// Finish marks the session as complete, sets the summary, and prunes old sessions.
func (r *Recorder) Finish(summary string) error {
	if !r.enabled {
		return nil
	}

	now := time.Now()
	r.session.FinishedAt = &now
	if summary != "" {
		r.session.Summary = &summary
	}

	if err := r.store.UpdateSession(r.session); err != nil {
		return err
	}

	// Auto-prune old sessions (best-effort)
	_, _ = r.store.PruneSessions(defaultRetention)
	return nil
}
