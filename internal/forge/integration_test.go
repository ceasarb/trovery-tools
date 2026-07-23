package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/session"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/storage"
)

// Integration tests covering the full CRAWL workflow.
// These tests build and run the actual binary.

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all integration tests
	dir, err := os.MkdirTemp("", "demi-forge-integration-*")
	if err != nil {
		panic(err)
	}
	binaryPath = filepath.Join(dir, "demi-forge")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/demi-forge")
	cmd.Dir = findRepoRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func findRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find repo root")
		}
		dir = parent
	}
}

func runAjnt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("demi-forge %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func runAjntExpectFail(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// --- Integration Tests ---

func TestIntegrationVersion(t *testing.T) {
	out := runAjnt(t, t.TempDir(), "--version")
	if !strings.Contains(out, "Demigo Forge") {
		t.Errorf("version output missing brand: %s", out)
	}
}

func TestIntegrationInitWorkspace(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "my-project")

	out := runAjnt(t, dir, "init", "my-project")
	if !strings.Contains(out, "Workspace created") {
		t.Errorf("missing success message: %s", out)
	}

	// Verify structure
	for _, f := range []string{".demi/forge.yaml", "agents", "servers", "README.md", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(wsDir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
}

func TestIntegrationInitNoServers(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "agent-only")

	runAjnt(t, dir, "init", "agent-only", "--no-servers")

	if _, err := os.Stat(filepath.Join(wsDir, "servers")); !os.IsNotExist(err) {
		t.Error("servers/ should not exist with --no-servers")
	}

	if _, err := os.Stat(filepath.Join(wsDir, "agents")); err != nil {
		t.Error("agents/ should exist")
	}
}

func TestIntegrationServerCreate(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")

	out := runAjnt(t, wsDir, "server", "create", "--name", "weather", "--language", "python", "--transport", "stdio")
	if !strings.Contains(out, "Server created") {
		t.Errorf("missing success message: %s", out)
	}

	serverDir := filepath.Join(wsDir, "servers", "weather")
	for _, f := range []string{"demi.toml", "pyproject.toml", "src/weather/server.py"} {
		if _, err := os.Stat(filepath.Join(serverDir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
}

func TestIntegrationServerCreateTypescript(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")

	out := runAjnt(t, wsDir, "server", "create", "--name", "api", "--language", "typescript", "--transport", "stdio")
	if !strings.Contains(out, "Server created") {
		t.Errorf("missing success: %s", out)
	}

	serverDir := filepath.Join(wsDir, "servers", "api")
	for _, f := range []string{"demi.toml", "package.json", "tsconfig.json", "src/server.ts"} {
		if _, err := os.Stat(filepath.Join(serverDir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
}

func TestIntegrationServerList(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")
	runAjnt(t, wsDir, "server", "create", "--name", "alpha", "--language", "python", "--transport", "stdio")
	runAjnt(t, wsDir, "server", "create", "--name", "beta", "--language", "python", "--transport", "stdio")

	out := runAjnt(t, wsDir, "server", "list")
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Errorf("server list missing entries: %s", out)
	}
}

func TestIntegrationServerCreateDuplicate(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")
	runAjnt(t, wsDir, "server", "create", "--name", "dupe", "--language", "python", "--transport", "stdio")

	out := runAjntExpectFail(t, wsDir, "server", "create", "--name", "dupe", "--language", "python", "--transport", "stdio")
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected duplicate error: %s", out)
	}
}

func TestIntegrationAgentCreate(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")

	out := runAjnt(t, wsDir, "agent", "create", "assistant", "--template", "single-agent")
	if !strings.Contains(out, "Agent created") {
		t.Errorf("missing success: %s", out)
	}

	agentDir := filepath.Join(wsDir, "agents", "assistant")
	if _, err := os.Stat(filepath.Join(agentDir, "agent.yaml")); err != nil {
		t.Error("missing agent.yaml")
	}
}

func TestIntegrationAgentList(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")
	runAjnt(t, wsDir, "agent", "create", "bot-a", "--template", "single-agent")
	runAjnt(t, wsDir, "agent", "create", "bot-b", "--template", "researcher")

	out := runAjnt(t, wsDir, "agent", "list")
	if !strings.Contains(out, "bot-a") || !strings.Contains(out, "bot-b") {
		t.Errorf("agent list missing entries: %s", out)
	}
}

func TestIntegrationAgentInspect(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")
	runAjnt(t, wsDir, "agent", "create", "mybot", "--template", "single-agent")

	out := runAjnt(t, wsDir, "agent", "inspect", "mybot")
	if !strings.Contains(out, "mybot") {
		t.Errorf("inspect missing agent name: %s", out)
	}
	if !strings.Contains(out, "anthropic") {
		t.Errorf("inspect missing provider: %s", out)
	}
}

func TestIntegrationAgentNotFound(t *testing.T) {
	dir := t.TempDir()
	runAjnt(t, dir, "init", "ws")

	wsDir := filepath.Join(dir, "ws")
	runAjnt(t, wsDir, "agent", "create", "real-agent", "--template", "custom")

	out := runAjntExpectFail(t, wsDir, "agent", "chat", "typo-agent")
	if !strings.Contains(out, "Agent not found") {
		t.Errorf("expected not found error: %s", out)
	}
	if !strings.Contains(out, "real-agent") {
		t.Errorf("expected available agents hint: %s", out)
	}
}

func TestIntegrationBinarySize(t *testing.T) {
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatal(err)
	}

	sizeMB := float64(info.Size()) / (1024 * 1024)
	if sizeMB > 50 {
		t.Errorf("binary size %.1fMB exceeds 50MB limit", sizeMB)
	}
	t.Logf("binary size: %.1fMB", sizeMB)
}

func TestIntegrationFullWorkflow(t *testing.T) {
	// Full CRAWL smoke test: init → server create → agent create → agent add-server → inspect
	// (skips server test/chat which need Python/API key)
	dir := t.TempDir()

	// Init workspace
	runAjnt(t, dir, "init", "smoke-test")
	wsDir := filepath.Join(dir, "smoke-test")

	// Create server
	runAjnt(t, wsDir, "server", "create", "--name", "weather", "--language", "python", "--transport", "stdio")

	// Create agent
	runAjnt(t, wsDir, "agent", "create", "assistant", "--template", "single-agent")

	// List both
	serverOut := runAjnt(t, wsDir, "server", "list")
	if !strings.Contains(serverOut, "weather") {
		t.Error("server list missing weather")
	}

	agentOut := runAjnt(t, wsDir, "agent", "list")
	if !strings.Contains(agentOut, "assistant") {
		t.Error("agent list missing assistant")
	}

	// Inspect agent
	inspectOut := runAjnt(t, wsDir, "agent", "inspect", "assistant")
	if !strings.Contains(inspectOut, "claude-haiku") {
		t.Error("inspect should show haiku model")
	}
}

// --- Server Lifecycle Integration ---

func TestServerLifecycle(t *testing.T) {
	dir := t.TempDir()

	// Create workspace
	runAjnt(t, dir, "init", "lifecycle")
	wsDir := filepath.Join(dir, "lifecycle")

	// Scaffold a Python server
	runAjnt(t, wsDir, "server", "create", "--name", "notes", "--language", "python", "--transport", "stdio")

	serverDir := filepath.Join(wsDir, "servers", "notes")

	// Verify demi.toml exists and is loadable
	tomlPath := filepath.Join(serverDir, "demi.toml")
	if _, err := os.Stat(tomlPath); err != nil {
		t.Fatalf("missing demi.toml: %v", err)
	}

	// Verify test fixtures exist
	testDir := filepath.Join(serverDir, "tests")
	if _, err := os.Stat(testDir); err != nil {
		t.Fatalf("missing tests/ directory: %v", err)
	}

	// Verify server source
	srcPath := filepath.Join(serverDir, "src", "notes", "server.py")
	if _, err := os.Stat(srcPath); err != nil {
		t.Fatalf("missing server.py: %v", err)
	}

	// Scaffold a TypeScript server in the same workspace
	runAjnt(t, wsDir, "server", "create", "--name", "calendar", "--language", "typescript", "--transport", "stdio")

	calDir := filepath.Join(wsDir, "servers", "calendar")
	for _, f := range []string{"demi.toml", "package.json", "src/server.ts"} {
		if _, err := os.Stat(filepath.Join(calDir, f)); err != nil {
			t.Errorf("calendar server missing %s: %v", f, err)
		}
	}

	// List should show both
	listOut := runAjnt(t, wsDir, "server", "list")
	if !strings.Contains(listOut, "notes") || !strings.Contains(listOut, "calendar") {
		t.Errorf("server list should contain both servers: %s", listOut)
	}
}

// --- Agent Lifecycle Integration ---

func TestAgentLifecycle(t *testing.T) {
	dir := t.TempDir()

	// Create workspace
	runAjnt(t, dir, "init", "agent-lifecycle")
	wsDir := filepath.Join(dir, "agent-lifecycle")

	// Scaffold agent
	runAjnt(t, wsDir, "agent", "create", "helper", "--template", "single-agent")

	agentDir := filepath.Join(wsDir, "agents", "helper")

	// Verify agent.yaml exists
	yamlPath := filepath.Join(agentDir, "agent.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("missing agent.yaml: %v", err)
	}

	// Load config and verify it parses correctly
	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		t.Fatalf("failed to load agent config: %v", err)
	}

	if cfg.Name != "helper" {
		t.Errorf("agent name = %q, want helper", cfg.Name)
	}
	if cfg.Model.Provider == "" {
		t.Error("provider should not be empty")
	}
	if cfg.Model.Model == "" {
		t.Error("model should not be empty")
	}

	// Create a second agent with a different template
	runAjnt(t, wsDir, "agent", "create", "researcher", "--template", "researcher")

	// List should show both
	listOut := runAjnt(t, wsDir, "agent", "list")
	if !strings.Contains(listOut, "helper") || !strings.Contains(listOut, "researcher") {
		t.Errorf("agent list should contain both agents: %s", listOut)
	}

	// Inspect should show config details
	inspectOut := runAjnt(t, wsDir, "agent", "inspect", "helper")
	if !strings.Contains(inspectOut, "helper") {
		t.Error("inspect should show agent name")
	}
}

// --- Storage Integration ---

func TestStorageIntegration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test-storage.db")

	// Open session store
	store, err := storage.NewSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	defer store.Close()

	// Create a session
	sess := &storage.Session{
		AgentName: "test-agent",
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-6",
		StartedAt: time.Now(),
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("session ID should be auto-generated")
	}

	// Create turns
	userTurn := &storage.SessionTurn{
		SessionID:  sess.ID,
		TurnNumber: 1,
		Role:       "user",
		Content:    "Hello!",
		CreatedAt:  time.Now(),
	}
	if err := store.CreateTurn(userTurn); err != nil {
		t.Fatalf("create user turn: %v", err)
	}

	assistantTurn := &storage.SessionTurn{
		SessionID:  sess.ID,
		TurnNumber: 2,
		Role:       "assistant",
		Content:    "Hi there!",
		TokensIn:   50,
		TokensOut:  30,
		CostUSD:    0.0005,
		CreatedAt:  time.Now(),
	}
	if err := store.CreateTurn(assistantTurn); err != nil {
		t.Fatalf("create assistant turn: %v", err)
	}

	// Create tool call on assistant turn
	argsJSON := `{"city":"London"}`
	resultJSON := `{"temp":15}`
	durMs := int64(120)
	toolCall := &storage.SessionToolCall{
		TurnID:        assistantTurn.ID,
		SessionID:     sess.ID,
		ToolName:      "weather.get_forecast",
		ServerName:    "weather",
		ArgumentsJSON: &argsJSON,
		ResultJSON:    &resultJSON,
		DurationMs:    &durMs,
		CreatedAt:     time.Now(),
	}
	if err := store.CreateToolCall(toolCall); err != nil {
		t.Fatalf("create tool call: %v", err)
	}

	// Query back and verify
	gotSess, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if gotSess.AgentName != "test-agent" {
		t.Errorf("agent name = %q", gotSess.AgentName)
	}

	turns, err := store.GetTurnsBySession(sess.ID)
	if err != nil {
		t.Fatalf("get turns: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(turns))
	}

	calls, err := store.GetToolCallsBySession(sess.ID)
	if err != nil {
		t.Fatalf("get tool calls: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(calls))
	}
	if calls[0].ToolName != "weather.get_forecast" {
		t.Errorf("tool name = %q", calls[0].ToolName)
	}

	// List sessions should include ours
	sessions, err := store.ListSessions("", 100, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.ID == sess.ID {
			found = true
		}
	}
	if !found {
		t.Error("session not found in list")
	}

	// Filter by agent name
	filtered, err := store.ListSessions("test-agent", 100, 0)
	if err != nil {
		t.Fatalf("list sessions filtered: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("filtered count = %d, want 1", len(filtered))
	}
}

// --- Session Recording Integration ---

func TestSessionRecording(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "rec-test.db")
	store, err := storage.NewSessionStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// Create recorder
	rec := session.New(store, "my-agent", "openai", "gpt-5-mini")
	if !rec.IsEnabled() {
		t.Fatal("recorder should be enabled")
	}

	sessionID := rec.SessionID()
	if sessionID == "" {
		t.Fatal("session ID should be set")
	}

	// Simulate a conversation: user -> assistant (with tool call) -> user -> assistant
	if err := rec.RecordUserTurn("What's the weather?"); err != nil {
		t.Fatalf("user turn 1: %v", err)
	}

	if err := rec.RecordAssistantTurn("Let me check...", 80, 20, 0.0003); err != nil {
		t.Fatalf("assistant turn 1: %v", err)
	}
	turn1ID := rec.CurrentTurnID()

	// Record tool call
	err = rec.RecordToolCall(turn1ID, session.ToolCallRecord{
		ToolName:   "weather.get_forecast",
		ServerName: "weather",
		Arguments:  map[string]any{"city": "SF"},
		Result:     "Sunny, 72F",
		DurationMs: 150,
	})
	if err != nil {
		t.Fatalf("tool call: %v", err)
	}

	// Second exchange
	if err := rec.RecordUserTurn("Thanks!"); err != nil {
		t.Fatalf("user turn 2: %v", err)
	}
	if err := rec.RecordAssistantTurn("You're welcome!", 40, 10, 0.0001); err != nil {
		t.Fatalf("assistant turn 2: %v", err)
	}

	// Finish
	if err := rec.Finish("Weather chat completed"); err != nil {
		t.Fatalf("finish: %v", err)
	}

	// Verify everything via direct store queries
	sess, err := store.GetSession(sessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.TotalTurns != 4 {
		t.Errorf("total turns = %d, want 4", sess.TotalTurns)
	}
	if sess.TotalTokensIn != 120 {
		t.Errorf("total tokens in = %d, want 120", sess.TotalTokensIn)
	}
	if sess.FinishedAt == nil {
		t.Error("session should be finished")
	}
	if sess.Summary == nil || *sess.Summary != "Weather chat completed" {
		t.Error("summary should be set")
	}

	turns, _ := store.GetTurnsBySession(sessionID)
	if len(turns) != 4 {
		t.Fatalf("got %d turns, want 4", len(turns))
	}

	// Tool calls should be on the first assistant turn
	calls, _ := store.GetToolCallsByTurn(turn1ID)
	if len(calls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(calls))
	}
	if calls[0].ServerName != "weather" {
		t.Errorf("server name = %q, want weather", calls[0].ServerName)
	}
}
