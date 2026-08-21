package storage

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	store, err := NewSessionStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSessionStoreIdempotentMigrations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.db")

	s1, err := NewSessionStore(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	s2, err := NewSessionStore(path)
	if err != nil {
		t.Fatalf("second open (idempotent): %v", err)
	}
	s2.Close()
}

func TestSessionCRUD(t *testing.T) {
	store := newTestSessionStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	sess := &Session{
		AgentName: "test-agent",
		Provider:  "openai",
		Model:     "gpt-4",
		StartedAt: now,
	}

	// Create
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected ID to be generated")
	}

	// Get
	got, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil {
		t.Fatal("GetSession returned nil")
	}
	if got.AgentName != "test-agent" {
		t.Errorf("AgentName = %q", got.AgentName)
	}
	if got.Provider != "openai" {
		t.Errorf("Provider = %q", got.Provider)
	}

	// Update
	finished := now.Add(30 * time.Second)
	summary := "test session complete"
	sess.FinishedAt = &finished
	sess.TotalTurns = 5
	sess.TotalTokensIn = 1000
	sess.TotalTokensOut = 500
	sess.TotalCostUSD = 0.05
	sess.Summary = &summary
	if err := store.UpdateSession(sess); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	got, _ = store.GetSession(sess.ID)
	if got.TotalTurns != 5 {
		t.Errorf("TotalTurns = %d, want 5", got.TotalTurns)
	}
	if got.TotalCostUSD != 0.05 {
		t.Errorf("TotalCostUSD = %f, want 0.05", got.TotalCostUSD)
	}
	if got.Summary == nil || *got.Summary != "test session complete" {
		t.Errorf("Summary = %v", got.Summary)
	}
}

func TestSessionGetNotFound(t *testing.T) {
	store := newTestSessionStore(t)
	got, err := store.GetSession("nonexistent")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestSessionListWithFilter(t *testing.T) {
	store := newTestSessionStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	sessions := []Session{
		{AgentName: "bot-a", Provider: "openai", Model: "gpt-4", StartedAt: now},
		{AgentName: "bot-a", Provider: "openai", Model: "gpt-4", StartedAt: now.Add(time.Second)},
		{AgentName: "bot-b", Provider: "anthropic", Model: "claude-3", StartedAt: now.Add(2 * time.Second)},
	}
	for i := range sessions {
		if err := store.CreateSession(&sessions[i]); err != nil {
			t.Fatalf("CreateSession[%d]: %v", i, err)
		}
	}

	// List all
	all, err := store.ListSessions("", 10, 0)
	if err != nil {
		t.Fatalf("ListSessions all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all = %d, want 3", len(all))
	}

	// Filter by agent
	botA, err := store.ListSessions("bot-a", 10, 0)
	if err != nil {
		t.Fatalf("ListSessions bot-a: %v", err)
	}
	if len(botA) != 2 {
		t.Errorf("bot-a = %d, want 2", len(botA))
	}

	// Pagination
	page1, err := store.ListSessions("", 2, 0)
	if err != nil {
		t.Fatalf("ListSessions page1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page1 = %d, want 2", len(page1))
	}

	page2, err := store.ListSessions("", 2, 2)
	if err != nil {
		t.Fatalf("ListSessions page2: %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2 = %d, want 1", len(page2))
	}
}

func TestSessionTurnCRUD(t *testing.T) {
	store := newTestSessionStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	sess := &Session{AgentName: "bot", Provider: "openai", Model: "gpt-4", StartedAt: now}
	store.CreateSession(sess)

	turns := []SessionTurn{
		{SessionID: sess.ID, TurnNumber: 1, Role: "user", Content: "hello", CreatedAt: now},
		{SessionID: sess.ID, TurnNumber: 2, Role: "assistant", Content: "hi there", TokensOut: 10, CreatedAt: now.Add(time.Second)},
	}
	for i := range turns {
		if err := store.CreateTurn(&turns[i]); err != nil {
			t.Fatalf("CreateTurn[%d]: %v", i, err)
		}
		if turns[i].ID == "" {
			t.Fatalf("turn[%d] ID not generated", i)
		}
	}

	got, err := store.GetTurnsBySession(sess.ID)
	if err != nil {
		t.Fatalf("GetTurnsBySession: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("turns = %d, want 2", len(got))
	}
	if got[0].Role != "user" {
		t.Errorf("turn[0].Role = %q", got[0].Role)
	}
	if got[1].Role != "assistant" {
		t.Errorf("turn[1].Role = %q", got[1].Role)
	}
}

func TestSessionTurnContentTruncation(t *testing.T) {
	store := newTestSessionStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	sess := &Session{AgentName: "bot", Provider: "openai", Model: "gpt-4", StartedAt: now}
	store.CreateSession(sess)

	// Create content just at the limit (should NOT be truncated)
	exactContent := strings.Repeat("a", maxContentBytes)
	turn1 := &SessionTurn{
		SessionID: sess.ID, TurnNumber: 1, Role: "user",
		Content: exactContent, CreatedAt: now,
	}
	if err := store.CreateTurn(turn1); err != nil {
		t.Fatalf("CreateTurn exact: %v", err)
	}

	turns, _ := store.GetTurnsBySession(sess.ID)
	if len(turns[0].Content) != maxContentBytes {
		t.Errorf("exact content len = %d, want %d", len(turns[0].Content), maxContentBytes)
	}

	// Create content over the limit (should be truncated)
	overContent := strings.Repeat("b", maxContentBytes+100)
	turn2 := &SessionTurn{
		SessionID: sess.ID, TurnNumber: 2, Role: "assistant",
		Content: overContent, CreatedAt: now.Add(time.Second),
	}
	if err := store.CreateTurn(turn2); err != nil {
		t.Fatalf("CreateTurn over: %v", err)
	}

	turns, _ = store.GetTurnsBySession(sess.ID)
	content := turns[1].Content
	if !strings.HasSuffix(content, "\n[truncated]") {
		t.Error("expected truncation marker")
	}
	expectedLen := maxContentBytes + len("\n[truncated]")
	if len(content) != expectedLen {
		t.Errorf("truncated content len = %d, want %d", len(content), expectedLen)
	}
}

func TestSessionToolCallCRUD(t *testing.T) {
	store := newTestSessionStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	sess := &Session{AgentName: "bot", Provider: "openai", Model: "gpt-4", StartedAt: now}
	store.CreateSession(sess)

	turn := &SessionTurn{SessionID: sess.ID, TurnNumber: 1, Role: "assistant", Content: "calling tool", CreatedAt: now}
	store.CreateTurn(turn)

	args := `{"city":"London"}`
	result := `{"temp":20}`
	dur := int64(200)
	call := &SessionToolCall{
		TurnID:        turn.ID,
		SessionID:     sess.ID,
		ToolName:      "get_weather",
		ServerName:    "weather-api",
		ArgumentsJSON: &args,
		ResultJSON:    &result,
		DurationMs:    &dur,
		CreatedAt:     now,
	}
	if err := store.CreateToolCall(call); err != nil {
		t.Fatalf("CreateToolCall: %v", err)
	}
	if call.ID == "" {
		t.Fatal("expected ID to be generated")
	}

	// By turn
	byTurn, err := store.GetToolCallsByTurn(turn.ID)
	if err != nil {
		t.Fatalf("GetToolCallsByTurn: %v", err)
	}
	if len(byTurn) != 1 {
		t.Fatalf("by turn = %d, want 1", len(byTurn))
	}
	if byTurn[0].ToolName != "get_weather" {
		t.Errorf("ToolName = %q", byTurn[0].ToolName)
	}
	if *byTurn[0].DurationMs != 200 {
		t.Errorf("DurationMs = %d", *byTurn[0].DurationMs)
	}

	// By session
	bySess, err := store.GetToolCallsBySession(sess.ID)
	if err != nil {
		t.Fatalf("GetToolCallsBySession: %v", err)
	}
	if len(bySess) != 1 {
		t.Fatalf("by session = %d, want 1", len(bySess))
	}
}

func TestSessionPruneSessions(t *testing.T) {
	store := newTestSessionStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	// Old session (48 hours ago)
	old := &Session{
		AgentName: "bot", Provider: "openai", Model: "gpt-4",
		StartedAt: now.Add(-48 * time.Hour),
	}
	store.CreateSession(old)

	oldTurn := &SessionTurn{
		SessionID: old.ID, TurnNumber: 1, Role: "user", Content: "old msg", CreatedAt: now.Add(-48 * time.Hour),
	}
	store.CreateTurn(oldTurn)

	oldCall := &SessionToolCall{
		TurnID: oldTurn.ID, SessionID: old.ID, ToolName: "tool", ServerName: "srv",
		CreatedAt: now.Add(-48 * time.Hour),
	}
	store.CreateToolCall(oldCall)

	// Recent session (1 hour ago)
	recent := &Session{
		AgentName: "bot", Provider: "openai", Model: "gpt-4",
		StartedAt: now.Add(-1 * time.Hour),
	}
	store.CreateSession(recent)

	// Prune sessions older than 24 hours
	count, err := store.PruneSessions(24 * time.Hour)
	if err != nil {
		t.Fatalf("PruneSessions: %v", err)
	}
	if count != 1 {
		t.Errorf("pruned = %d, want 1", count)
	}

	// Old session gone
	got, _ := store.GetSession(old.ID)
	if got != nil {
		t.Error("old session should be deleted")
	}

	// Recent session remains
	got, _ = store.GetSession(recent.ID)
	if got == nil {
		t.Error("recent session should remain")
	}

	// Old turns and tool calls also gone
	turns, _ := store.GetTurnsBySession(old.ID)
	if len(turns) != 0 {
		t.Errorf("old turns remaining = %d", len(turns))
	}
	calls, _ := store.GetToolCallsBySession(old.ID)
	if len(calls) != 0 {
		t.Errorf("old tool calls remaining = %d", len(calls))
	}
}
