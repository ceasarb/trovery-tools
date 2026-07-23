package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/guardrails"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/provider"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/servermgr"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	response string
}

func (m *mockProvider) CreateMessage(msgs []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string) (*provider.Response, error) {
	return &provider.Response{
		Content:    []provider.Content{{Type: "text", Text: m.response}},
		Usage:      provider.Usage{InputTokens: 10, OutputTokens: 20},
		StopReason: "end_turn",
	}, nil
}

func (m *mockProvider) CreateMessageStream(msgs []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string, handler provider.StreamHandler) (*provider.Response, error) {
	handler(provider.StreamEvent{Type: "text", Text: m.response})
	handler(provider.StreamEvent{Type: "done"})
	return &provider.Response{
		Content:    []provider.Content{{Type: "text", Text: m.response}},
		Usage:      provider.Usage{InputTokens: 10, OutputTokens: 20},
		StopReason: "end_turn",
	}, nil
}

func newTestServer(response string) *Server {
	cfg := &agentcfg.AgentConfig{
		Name: "test-agent",
		Model: agentcfg.ModelConfig{
			Provider:  "openai",
			Model:     "gpt-4o-mini",
			MaxTokens: 1024,
		},
		System:  "You are a test assistant.",
		Settings: agentcfg.AgentSettings{MaxToolCalls: 10, Namespacing: "auto"},
	}

	return New(Config{
		AgentConfig: cfg,
		Provider:    &mockProvider{response: response},
		ServerMgr:   servermgr.NewManager(),
		AgentDir:    "/tmp/test-agent",
		Host:        "localhost",
		Port:        0,
	})
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer("hello")
	srv.MarkReady()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", body["status"])
	}
}

func TestReadyEndpointNotReady(t *testing.T) {
	srv := newTestServer("hello")
	// Not calling MarkReady

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "not_ready" {
		t.Fatalf("expected status not_ready, got %s", body["status"])
	}
}

func TestReadyEndpointReady(t *testing.T) {
	srv := newTestServer("hello")
	srv.MarkReady()

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInvokeEndpoint(t *testing.T) {
	srv := newTestServer("Hello from the agent!")
	srv.MarkReady()

	body := `{"message": "hi"}`
	req := httptest.NewRequest("POST", "/invoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp InvokeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Response != "Hello from the agent!" {
		t.Fatalf("expected response text, got %q", resp.Response)
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 20 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
	if resp.Model != "gpt-4o-mini" {
		t.Fatalf("expected model gpt-4o-mini, got %s", resp.Model)
	}

	// Verify cost headers
	if w.Header().Get("X-Demi-Tokens-In") != "10" {
		t.Fatalf("expected X-Demi-Tokens-In=10, got %s", w.Header().Get("X-Demi-Tokens-In"))
	}
	if w.Header().Get("X-Demi-Tokens-Out") != "20" {
		t.Fatalf("expected X-Demi-Tokens-Out=20, got %s", w.Header().Get("X-Demi-Tokens-Out"))
	}
	if w.Header().Get("X-Demi-Tool-Calls") != "0" {
		t.Fatalf("expected X-Demi-Tool-Calls=0, got %s", w.Header().Get("X-Demi-Tool-Calls"))
	}
	if w.Header().Get("X-Demi-Cost") == "" {
		t.Fatal("expected X-Demi-Cost header to be set")
	}
}

func TestInvokeEndpointMissingMessage(t *testing.T) {
	srv := newTestServer("hello")
	srv.MarkReady()

	body := `{"message": ""}`
	req := httptest.NewRequest("POST", "/invoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestInvokeEndpointInvalidJSON(t *testing.T) {
	srv := newTestServer("hello")
	srv.MarkReady()

	req := httptest.NewRequest("POST", "/invoke", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestInvokeEndpointWithOverrides(t *testing.T) {
	srv := newTestServer("overridden response")
	srv.MarkReady()

	body := `{"message": "hi", "max_tokens": 2048, "temperature": 0.5}`
	req := httptest.NewRequest("POST", "/invoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp InvokeResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Response != "overridden response" {
		t.Fatalf("expected overridden response, got %q", resp.Response)
	}
}

func TestInvokeStreamEndpoint(t *testing.T) {
	srv := newTestServer("streamed hello")
	srv.MarkReady()

	body := `{"message": "hi"}`
	req := httptest.NewRequest("POST", "/invoke/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}

	// Parse SSE events
	data := w.Body.String()
	if !strings.Contains(data, "event: text") {
		t.Fatal("expected text event in SSE stream")
	}
	if !strings.Contains(data, "event: done") {
		t.Fatal("expected done event in SSE stream")
	}
	if !strings.Contains(data, "streamed hello") {
		t.Fatalf("expected response text in stream, got:\n%s", data)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := newTestServer("hello")

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected CORS origin header")
	}
	if !strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), "X-Demi-Cost") {
		t.Fatal("expected X-Demi-Cost in exposed headers")
	}
}

func TestInvokeStreamDoneEventContainsUsage(t *testing.T) {
	srv := newTestServer("done check")
	srv.MarkReady()

	body := `{"message": "hi"}`
	req := httptest.NewRequest("POST", "/invoke/stream", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	// Extract the done event data
	lines := strings.Split(w.Body.String(), "\n")
	var doneData string
	for i, line := range lines {
		if line == "event: done" && i+1 < len(lines) {
			doneData = strings.TrimPrefix(lines[i+1], "data: ")
			break
		}
	}

	if doneData == "" {
		t.Fatalf("no done event found in:\n%s", w.Body.String())
	}

	var done map[string]any
	if err := json.Unmarshal([]byte(doneData), &done); err != nil {
		t.Fatalf("parse done data: %v", err)
	}

	if done["response"] != "done check" {
		t.Fatalf("expected response in done event, got %v", done["response"])
	}
	if done["model"] != "gpt-4o-mini" {
		t.Fatalf("expected model in done event, got %v", done["model"])
	}

	usage, ok := done["usage"].(map[string]any)
	if !ok {
		t.Fatal("expected usage object in done event")
	}
	if usage["input_tokens"] != float64(10) {
		t.Fatalf("expected input_tokens=10, got %v", usage["input_tokens"])
	}
}

func newTestServerWithBudget(response string, perReq, monthly float64) (*Server, func()) {
	dir := filepath.Join("/tmp", "demi-forge-test-budget")
	store, _ := guardrails.NewCostStore(filepath.Join(dir, "cost.db"))
	budget := guardrails.New(perReq, monthly, store)

	cfg := &agentcfg.AgentConfig{
		Name: "test-agent",
		Model: agentcfg.ModelConfig{
			Provider:  "openai",
			Model:     "gpt-4o-mini",
			MaxTokens: 1024,
		},
		System:   "You are a test assistant.",
		Settings: agentcfg.AgentSettings{MaxToolCalls: 10, Namespacing: "auto"},
	}

	srv := New(Config{
		AgentConfig: cfg,
		Provider:    &mockProvider{response: response},
		ServerMgr:   servermgr.NewManager(),
		Budget:      budget,
		AgentDir:    "/tmp/test-agent",
		Host:        "localhost",
		Port:        0,
	})
	srv.MarkReady()

	cleanup := func() { store.Close() }
	return srv, cleanup
}

func TestInvokeMonthlyBudgetExceeded(t *testing.T) {
	dir := t.TempDir()
	store, err := guardrails.NewCostStore(filepath.Join(dir, "cost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Pre-fill monthly spend to exceed cap
	store.RecordCost("test-agent", 100.0, time.Now())

	budget := guardrails.New(0, 10.0, store)

	cfg := &agentcfg.AgentConfig{
		Name: "test-agent",
		Model: agentcfg.ModelConfig{Provider: "openai", Model: "gpt-4o-mini", MaxTokens: 1024},
		Settings: agentcfg.AgentSettings{MaxToolCalls: 10, Namespacing: "auto"},
	}
	srv := New(Config{
		AgentConfig: cfg,
		Provider:    &mockProvider{response: "hello"},
		ServerMgr:   servermgr.NewManager(),
		Budget:      budget,
		AgentDir:    "/tmp/test-agent",
		Host:        "localhost",
		Port:        0,
	})
	srv.MarkReady()

	body := `{"message": "hi"}`
	req := httptest.NewRequest("POST", "/invoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}

	if w.Header().Get("X-Demi-Monthly-Remaining") != "0.000000" {
		t.Fatalf("expected X-Demi-Monthly-Remaining=0.000000, got %s", w.Header().Get("X-Demi-Monthly-Remaining"))
	}
}

func TestInvokeMonthlyBudgetRemainingHeader(t *testing.T) {
	dir := t.TempDir()
	store, err := guardrails.NewCostStore(filepath.Join(dir, "cost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Pre-fill some spend but under cap
	store.RecordCost("test-agent", 3.0, time.Now())

	budget := guardrails.New(0, 10.0, store)

	cfg := &agentcfg.AgentConfig{
		Name: "test-agent",
		Model: agentcfg.ModelConfig{Provider: "openai", Model: "gpt-4o-mini", MaxTokens: 1024},
		Settings: agentcfg.AgentSettings{MaxToolCalls: 10, Namespacing: "auto"},
	}
	srv := New(Config{
		AgentConfig: cfg,
		Provider:    &mockProvider{response: "hello"},
		ServerMgr:   servermgr.NewManager(),
		Budget:      budget,
		AgentDir:    "/tmp/test-agent",
		Host:        "localhost",
		Port:        0,
	})
	srv.MarkReady()

	body := `{"message": "hi"}`
	req := httptest.NewRequest("POST", "/invoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	remaining := w.Header().Get("X-Demi-Monthly-Remaining")
	if remaining == "" {
		t.Fatal("expected X-Demi-Monthly-Remaining header")
	}
}

