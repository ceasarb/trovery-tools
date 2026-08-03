package session

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/storage"
)

func newTestStore(t *testing.T) *storage.SessionStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test-sessions.db")
	store, err := storage.NewSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRecorderFullFlow(t *testing.T) {
	store := newTestStore(t)
	rec := New(store, "test-agent", "anthropic", "claude-sonnet-4-6")

	if !rec.IsEnabled() {
		t.Fatal("recorder should be enabled")
	}
	if rec.SessionID() == "" {
		t.Fatal("session ID should be set")
	}

	// Record user turn
	if err := rec.RecordUserTurn("Hello, what tools do you have?"); err != nil {
		t.Fatalf("RecordUserTurn: %v", err)
	}
	userTurnID := rec.CurrentTurnID()
	if userTurnID == "" {
		t.Fatal("current turn ID should be set after user turn")
	}

	// Record assistant turn with tool calls
	if err := rec.RecordAssistantTurn("Let me check my tools.", 100, 50, 0.001); err != nil {
		t.Fatalf("RecordAssistantTurn: %v", err)
	}
	assistantTurnID := rec.CurrentTurnID()
	if assistantTurnID == userTurnID {
		t.Fatal("assistant turn should have a different ID from user turn")
	}

	// Record a tool call on the assistant turn
	err := rec.RecordToolCall(assistantTurnID, ToolCallRecord{
		ToolName:   "weather.get_forecast",
		ServerName: "weather",
		Arguments:  map[string]any{"city": "London"},
		Result:     `{"temp": 15, "condition": "cloudy"}`,
		DurationMs: 230,
	})
	if err != nil {
		t.Fatalf("RecordToolCall: %v", err)
	}

	// Record a second tool call with an error
	err = rec.RecordToolCall(assistantTurnID, ToolCallRecord{
		ToolName:   "weather.get_alerts",
		ServerName: "weather",
		Error:      "timeout",
		DurationMs: 5000,
	})
	if err != nil {
		t.Fatalf("RecordToolCall error case: %v", err)
	}

	// Finish
	if err := rec.Finish("2 turns, 1 tool call"); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	// Verify via store queries
	sess, err := store.GetSession(rec.SessionID())
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess == nil {
		t.Fatal("session not found")
	}
	if sess.AgentName != "test-agent" {
		t.Errorf("agent name = %q, want test-agent", sess.AgentName)
	}
	if sess.FinishedAt == nil {
		t.Error("finished_at should be set after Finish()")
	}
	if sess.Summary == nil || *sess.Summary != "2 turns, 1 tool call" {
		t.Errorf("summary = %v, want '2 turns, 1 tool call'", sess.Summary)
	}
	if sess.TotalTokensIn != 100 {
		t.Errorf("total_tokens_in = %d, want 100", sess.TotalTokensIn)
	}
	if sess.TotalTokensOut != 50 {
		t.Errorf("total_tokens_out = %d, want 50", sess.TotalTokensOut)
	}

	// Verify turns
	turns, err := store.GetTurnsBySession(rec.SessionID())
	if err != nil {
		t.Fatalf("GetTurnsBySession: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(turns))
	}
	if turns[0].Role != "user" {
		t.Errorf("turn 0 role = %q, want user", turns[0].Role)
	}
	if turns[1].Role != "assistant" {
		t.Errorf("turn 1 role = %q, want assistant", turns[1].Role)
	}

	// Verify tool calls
	toolCalls, err := store.GetToolCallsByTurn(assistantTurnID)
	if err != nil {
		t.Fatalf("GetToolCallsByTurn: %v", err)
	}
	if len(toolCalls) != 2 {
		t.Fatalf("got %d tool calls, want 2", len(toolCalls))
	}
	if toolCalls[0].ToolName != "weather.get_forecast" {
		t.Errorf("tool call 0 name = %q", toolCalls[0].ToolName)
	}
	if toolCalls[1].Error == nil || *toolCalls[1].Error != "timeout" {
		t.Errorf("tool call 1 error = %v, want timeout", toolCalls[1].Error)
	}
}

func TestRecorderDisabled(t *testing.T) {
	rec := NewDisabled()

	if rec.IsEnabled() {
		t.Fatal("disabled recorder should not be enabled")
	}
	if rec.SessionID() != "" {
		t.Errorf("disabled recorder session ID should be empty, got %q", rec.SessionID())
	}
	if rec.CurrentTurnID() != "" {
		t.Errorf("disabled recorder turn ID should be empty, got %q", rec.CurrentTurnID())
	}

	// All methods should be no-ops returning nil
	if err := rec.RecordUserTurn("hello"); err != nil {
		t.Errorf("RecordUserTurn: %v", err)
	}
	if err := rec.RecordAssistantTurn("hi", 10, 5, 0.001); err != nil {
		t.Errorf("RecordAssistantTurn: %v", err)
	}
	if err := rec.RecordToolCall("turn-1", ToolCallRecord{ToolName: "test"}); err != nil {
		t.Errorf("RecordToolCall: %v", err)
	}
	if err := rec.Finish("done"); err != nil {
		t.Errorf("Finish: %v", err)
	}
}

func TestRecorderMultipleTurns(t *testing.T) {
	store := newTestStore(t)
	rec := New(store, "multi-turn", "openai", "gpt-5-mini")

	// Simulate a 3-turn conversation
	for i := 0; i < 3; i++ {
		if err := rec.RecordUserTurn("question"); err != nil {
			t.Fatalf("user turn %d: %v", i, err)
		}
		if err := rec.RecordAssistantTurn("answer", 50, 30, 0.0005); err != nil {
			t.Fatalf("assistant turn %d: %v", i, err)
		}
	}

	if err := rec.Finish("3 exchanges"); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	sess, _ := store.GetSession(rec.SessionID())
	if sess.TotalTurns != 6 { // 3 user + 3 assistant
		t.Errorf("total turns = %d, want 6", sess.TotalTurns)
	}
	if sess.TotalTokensIn != 150 {
		t.Errorf("total tokens in = %d, want 150", sess.TotalTokensIn)
	}
	if sess.TotalTokensOut != 90 {
		t.Errorf("total tokens out = %d, want 90", sess.TotalTokensOut)
	}
}

func TestRecorderConcurrent(t *testing.T) {
	store := newTestStore(t)
	rec := New(store, "concurrent", "anthropic", "claude-sonnet-4-6")

	// Record initial user turn
	if err := rec.RecordUserTurn("start"); err != nil {
		t.Fatalf("initial turn: %v", err)
	}

	// Record assistant turn so we have a turn ID for tool calls
	if err := rec.RecordAssistantTurn("thinking...", 10, 5, 0.0001); err != nil {
		t.Fatalf("assistant turn: %v", err)
	}
	turnID := rec.CurrentTurnID()

	// Record multiple tool calls concurrently
	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = rec.RecordToolCall(turnID, ToolCallRecord{
				ToolName:   "tool-" + string(rune('A'+idx)),
				ServerName: "test-server",
				DurationMs: int64(idx * 100),
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent tool call %d: %v", i, err)
		}
	}

	// Verify all tool calls were recorded
	calls, err := store.GetToolCallsByTurn(turnID)
	if err != nil {
		t.Fatalf("GetToolCallsByTurn: %v", err)
	}
	if len(calls) != 5 {
		t.Errorf("got %d tool calls, want 5", len(calls))
	}
}

func TestRecorderPruneOnFinish(t *testing.T) {
	store := newTestStore(t)

	// Create an "old" session directly in the store (more than 30 days ago)
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	oldSess := &storage.Session{
		AgentName: "old-agent",
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-6",
		StartedAt: oldTime,
	}
	if err := store.CreateSession(oldSess); err != nil {
		t.Fatalf("create old session: %v", err)
	}

	// Create a new recorder and finish it — should prune the old session
	rec := New(store, "new-agent", "anthropic", "claude-sonnet-4-6")
	if err := rec.RecordUserTurn("hello"); err != nil {
		t.Fatalf("RecordUserTurn: %v", err)
	}
	if err := rec.Finish("done"); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	// The old session should have been pruned (> 30 days old)
	old, err := store.GetSession(oldSess.ID)
	if err != nil {
		t.Fatalf("GetSession old: %v", err)
	}
	if old != nil {
		t.Error("old session should have been pruned")
	}

	// The new session should still exist
	current, err := store.GetSession(rec.SessionID())
	if err != nil {
		t.Fatalf("GetSession current: %v", err)
	}
	if current == nil {
		t.Error("current session should still exist")
	}
}

func TestRecorderToolCallNilArgs(t *testing.T) {
	store := newTestStore(t)
	rec := New(store, "nil-args", "anthropic", "claude-sonnet-4-6")

	if err := rec.RecordUserTurn("test"); err != nil {
		t.Fatalf("RecordUserTurn: %v", err)
	}
	if err := rec.RecordAssistantTurn("ok", 10, 5, 0.001); err != nil {
		t.Fatalf("RecordAssistantTurn: %v", err)
	}

	// Record tool call with nil arguments and empty result/error
	err := rec.RecordToolCall(rec.CurrentTurnID(), ToolCallRecord{
		ToolName:   "no-args-tool",
		ServerName: "srv",
	})
	if err != nil {
		t.Fatalf("RecordToolCall nil args: %v", err)
	}

	calls, _ := store.GetToolCallsByTurn(rec.CurrentTurnID())
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	if calls[0].ArgumentsJSON != nil {
		t.Errorf("arguments should be nil, got %v", calls[0].ArgumentsJSON)
	}
}
