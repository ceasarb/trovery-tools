package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/servermgr"
)

// newTestServerForModel builds a server serving a specific model, so override
// validation can be exercised against both a permissive and a restrictive one.
func newTestServerForModel(model, response string) *Server {
	cfg := &agentcfg.AgentConfig{
		Name: "test-agent",
		Model: agentcfg.ModelConfig{
			Provider:  "anthropic",
			Model:     model,
			MaxTokens: 1024,
		},
		System:   "You are a test assistant.",
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

func float64Ptr(v float64) *float64 { return &v }
func intPtr(v int) *int             { return &v }

func TestApplyOverridesRejectsTemperatureOnRestrictiveModel(t *testing.T) {
	srv := newTestServerForModel("claude-opus-5", "hi")

	_, err := srv.applyOverrides(&InvokeRequest{
		Message:     "hello",
		Temperature: float64Ptr(0.7),
	})
	if err == nil {
		t.Fatal("expected an error overriding temperature on claude-opus-5")
	}
	if !strings.Contains(err.Error(), "claude-opus-5") {
		t.Errorf("error should name the model, got: %v", err)
	}
}

func TestApplyOverridesAllowsTemperatureOnPermissiveModel(t *testing.T) {
	srv := newTestServerForModel("claude-haiku-4-5", "hi")

	cfg, err := srv.applyOverrides(&InvokeRequest{
		Message:     "hello",
		Temperature: float64Ptr(0.7),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", cfg.Model.Temperature)
	}
}

// max_tokens is unaffected by the capability gate — it is accepted on every model, and
// narrowing the guard to temperature is the point.
func TestApplyOverridesMaxTokensUnaffected(t *testing.T) {
	srv := newTestServerForModel("claude-opus-5", "hi")

	cfg, err := srv.applyOverrides(&InvokeRequest{
		Message:   "hello",
		MaxTokens: intPtr(4096),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", cfg.Model.MaxTokens)
	}
}

// An override the model rejects must not reach the provider — the caller gets 400
// rather than a 500 surfacing the provider's own refusal.
func TestInvokeRejectsUnsupportedTemperatureWith400(t *testing.T) {
	srv := newTestServerForModel("claude-opus-5", "hi")
	srv.MarkReady()

	body := strings.NewReader(`{"message":"hello","temperature":0.7}`)
	req := httptest.NewRequest("POST", "/invoke", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestInvokeStreamRejectsUnsupportedTemperatureWith400(t *testing.T) {
	srv := newTestServerForModel("claude-opus-5", "hi")
	srv.MarkReady()

	body := strings.NewReader(`{"message":"hello","temperature":0.7}`)
	req := httptest.NewRequest("POST", "/invoke/stream", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.httpSrv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}
