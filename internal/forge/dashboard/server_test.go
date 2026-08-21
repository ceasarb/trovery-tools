package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ceasarb/trovery-tools/pkg/forge/shared/storage"
)

// testServer creates a dashboard Server backed by a temp workspace with
// pre-populated SQLite databases, server configs, and agent configs.
func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()

	// Create workspace structure
	troveDir := filepath.Join(dir, ".trove/forge")
	os.MkdirAll(troveDir, 0o755)
	os.MkdirAll(filepath.Join(dir, "servers", "my-server"), 0o755)
	os.MkdirAll(filepath.Join(dir, "agents", "my-agent"), 0o755)

	// Write .trove/forge.yaml
	os.WriteFile(filepath.Join(dir, ".trove/forge.yaml"), []byte(`[workspace]
name = "test-workspace"
`), 0o644)

	// Write server config
	os.WriteFile(filepath.Join(dir, "servers", "my-server", "trove.toml"), []byte(`[server]
name = "my-server"
entry = "main.py"
command = "python main.py"
transport = "stdio"

[testing]
fixtures = "tests/"
`), 0o644)

	// Write pyproject.toml so language detection works
	os.WriteFile(filepath.Join(dir, "servers", "my-server", "pyproject.toml"), []byte(`[project]
name = "my-server"
`), 0o644)

	// Write agent config
	os.WriteFile(filepath.Join(dir, "agents", "my-agent", "agent.yaml"), []byte(`name: my-agent
model:
  provider: anthropic
  model: claude-sonnet-4-20250514
system_prompt: You are a helpful assistant.
servers:
  - name: my-server
    path: ../../servers/my-server
settings:
  max_tool_calls: 10
`), 0o644)

	// Populate eval store
	evalStore, err := storage.NewEvalStore(filepath.Join(troveDir, "evals.db"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	finished := now.Add(5 * time.Second)
	evalRun := &storage.EvalRun{
		ID:             "eval-run-1",
		Source:         "server",
		TargetName:     "my-server",
		SuiteName:      "basic",
		StartedAt:      now,
		FinishedAt:     &finished,
		TotalScenarios: 2,
		Passed:         1,
		Failed:         1,
		Skipped:        0,
		Status:         "failed",
	}
	if err := evalStore.CreateRun(evalRun); err != nil {
		t.Fatal(err)
	}

	dur := int64(150)
	errMsg := "assertion failed"
	evalResult1 := &storage.EvalResult{
		ID:           "eval-result-1",
		RunID:        "eval-run-1",
		ScenarioName: "test_add",
		Status:       "passed",
		DurationMs:   &dur,
		CreatedAt:    now,
	}
	evalResult2 := &storage.EvalResult{
		ID:           "eval-result-2",
		RunID:        "eval-run-1",
		ScenarioName: "test_delete",
		Status:       "failed",
		DurationMs:   &dur,
		ErrorMessage: &errMsg,
		CreatedAt:    now,
	}
	if err := evalStore.CreateResult(evalResult1); err != nil {
		t.Fatal(err)
	}
	if err := evalStore.CreateResult(evalResult2); err != nil {
		t.Fatal(err)
	}
	evalStore.Close()

	// Populate session store
	sessStore, err := storage.NewSessionStore(filepath.Join(troveDir, "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}

	sessFinished := now.Add(30 * time.Second)
	session := &storage.Session{
		ID:             "sess-1",
		AgentName:      "my-agent",
		Provider:       "anthropic",
		Model:          "claude-sonnet-4-20250514",
		StartedAt:      now,
		FinishedAt:     &sessFinished,
		TotalTurns:     2,
		TotalTokensIn:  500,
		TotalTokensOut: 200,
		TotalCostUSD:   0.005,
	}
	if err := sessStore.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	turn1 := &storage.SessionTurn{
		ID:         "turn-1",
		SessionID:  "sess-1",
		TurnNumber: 1,
		Role:       "user",
		Content:    "Hello",
		TokensIn:   100,
		TokensOut:  0,
		CostUSD:    0.001,
		CreatedAt:  now,
	}
	turn2 := &storage.SessionTurn{
		ID:         "turn-2",
		SessionID:  "sess-1",
		TurnNumber: 2,
		Role:       "assistant",
		Content:    "Hi there!",
		TokensIn:   0,
		TokensOut:  50,
		CostUSD:    0.002,
		CreatedAt:  now.Add(time.Second),
	}
	if err := sessStore.CreateTurn(turn1); err != nil {
		t.Fatal(err)
	}
	if err := sessStore.CreateTurn(turn2); err != nil {
		t.Fatal(err)
	}

	callDur := int64(42)
	argsJSON := `{"x": 1}`
	resultJSON := `{"sum": 2}`
	toolCall := &storage.SessionToolCall{
		ID:            "call-1",
		TurnID:        "turn-2",
		SessionID:     "sess-1",
		ToolName:      "add",
		ServerName:    "my-server",
		ArgumentsJSON: &argsJSON,
		ResultJSON:    &resultJSON,
		DurationMs:    &callDur,
		CreatedAt:     now.Add(time.Second),
	}
	if err := sessStore.CreateToolCall(toolCall); err != nil {
		t.Fatal(err)
	}
	sessStore.Close()

	// Create the dashboard server pointing at the temp workspace
	srv, err := New(0, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		srv.evalStore.Close()
		srv.sessStore.Close()
	})

	return srv
}

func TestListServers(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/servers", nil)
	w := httptest.NewRecorder()
	srv.handleListServers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Fatalf("expected 1 server, got %d", resp.Total)
	}
}

func TestGetServer(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/servers/my-server", nil)
	req.SetPathValue("name", "my-server")
	w := httptest.NewRecorder()
	srv.handleGetServer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp detailResponse
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp.Data.(map[string]interface{})
	if data["name"] != "my-server" {
		t.Fatalf("expected name my-server, got %v", data["name"])
	}
	if data["language"] != "python" {
		t.Fatalf("expected language python, got %v", data["language"])
	}
}

func TestGetServerNotFound(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/servers/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	srv.handleGetServer(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListAgents(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()
	srv.handleListAgents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Fatalf("expected 1 agent, got %d", resp.Total)
	}
}

func TestGetAgent(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/agents/my-agent", nil)
	req.SetPathValue("name", "my-agent")
	w := httptest.NewRecorder()
	srv.handleGetAgent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp detailResponse
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp.Data.(map[string]interface{})
	if data["name"] != "my-agent" {
		t.Fatalf("expected name my-agent, got %v", data["name"])
	}
	if data["provider"] != "anthropic" {
		t.Fatalf("expected provider anthropic, got %v", data["provider"])
	}
}

func TestGetAgentNotFound(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/agents/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	srv.handleGetAgent(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListEvals(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/evals", nil)
	w := httptest.NewRecorder()
	srv.handleListEvals(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Fatalf("expected 1 eval run, got %d", resp.Total)
	}
}

func TestListEvalsWithSourceFilter(t *testing.T) {
	srv := testServer(t)

	// Filter by server — should return 1
	req := httptest.NewRequest("GET", "/api/evals?source=server", nil)
	w := httptest.NewRecorder()
	srv.handleListEvals(w, req)

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Fatalf("expected 1 eval run for source=server, got %d", resp.Total)
	}

	// Filter by agent — should return 0
	req = httptest.NewRequest("GET", "/api/evals?source=agent", nil)
	w = httptest.NewRecorder()
	srv.handleListEvals(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 0 {
		t.Fatalf("expected 0 eval runs for source=agent, got %d", resp.Total)
	}
}

func TestGetEval(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/evals/eval-run-1", nil)
	req.SetPathValue("id", "eval-run-1")
	w := httptest.NewRecorder()
	srv.handleGetEval(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetEvalNotFound(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/evals/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	srv.handleGetEval(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListSessions(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.handleListSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Fatalf("expected 1 session, got %d", resp.Total)
	}
}

func TestListSessionsWithAgentFilter(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/sessions?agent=my-agent", nil)
	w := httptest.NewRecorder()
	srv.handleListSessions(w, req)

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Fatalf("expected 1 session for agent=my-agent, got %d", resp.Total)
	}

	req = httptest.NewRequest("GET", "/api/sessions?agent=other", nil)
	w = httptest.NewRecorder()
	srv.handleListSessions(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 0 {
		t.Fatalf("expected 0 sessions for agent=other, got %d", resp.Total)
	}
}

func TestGetSession(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/sessions/sess-1", nil)
	req.SetPathValue("id", "sess-1")
	w := httptest.NewRecorder()
	srv.handleGetSession(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSessionNotFound(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/sessions/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	srv.handleGetSession(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestExportSession(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/sessions/sess-1/export", nil)
	req.SetPathValue("id", "sess-1")
	w := httptest.NewRecorder()
	srv.handleExportSession(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	disp := w.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "sess-1") {
		t.Fatalf("expected Content-Disposition with session ID, got %q", disp)
	}
}

func TestAnalyticsCost(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/analytics/cost?days=30", nil)
	w := httptest.NewRecorder()
	srv.handleAnalyticsCost(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// Should have at least 1 data point since we inserted a session for today
	if resp.Total < 1 {
		t.Fatalf("expected at least 1 cost data point, got %d", resp.Total)
	}
}

func TestAnalyticsTokens(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/analytics/tokens", nil)
	w := httptest.NewRecorder()
	srv.handleAnalyticsTokens(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAnalyticsErrors(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/analytics/errors", nil)
	w := httptest.NewRecorder()
	srv.handleAnalyticsErrors(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAnalyticsLatency(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/analytics/latency", nil)
	w := httptest.NewRecorder()
	srv.handleAnalyticsLatency(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAnalyticsUsage(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/analytics/usage", nil)
	w := httptest.NewRecorder()
	srv.handleAnalyticsUsage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp listResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// Should have at least 1 usage point for the "add" tool call
	if resp.Total < 1 {
		t.Fatalf("expected at least 1 usage data point, got %d", resp.Total)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Regular request
	req := httptest.NewRequest("GET", "/api/servers", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS origin header")
	}

	// Preflight OPTIONS request
	req = httptest.NewRequest("OPTIONS", "/api/servers", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
}

func TestJSONContentTypeMiddleware(t *testing.T) {
	handler := jsonContentType(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// API route should get JSON content type
	req := httptest.NewRequest("GET", "/api/servers", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	// Non-API route should NOT get JSON content type
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	ct = w.Header().Get("Content-Type")
	if ct == "application/json" {
		t.Fatal("non-API route should not have JSON content type")
	}
}

func TestListToolsNoServerParam(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("GET", "/api/tools", nil)
	w := httptest.NewRecorder()
	srv.handleListTools(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCallToolInvalidBody(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/api/tools/call", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()
	srv.handleCallTool(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCallToolMissingFields(t *testing.T) {
	srv := testServer(t)

	req := httptest.NewRequest("POST", "/api/tools/call", strings.NewReader(`{"server":"","tool":""}`))
	w := httptest.NewRecorder()
	srv.handleCallTool(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
