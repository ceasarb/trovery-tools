package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRegistersAllMetrics(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}

	// Verify all fields are non-nil
	if m.Requests == nil {
		t.Error("Requests counter is nil")
	}
	if m.ToolCalls == nil {
		t.Error("ToolCalls counter is nil")
	}
	if m.ToolCallErrors == nil {
		t.Error("ToolCallErrors counter is nil")
	}
	if m.TokensInput == nil {
		t.Error("TokensInput counter is nil")
	}
	if m.TokensOutput == nil {
		t.Error("TokensOutput counter is nil")
	}
	if m.CostUSD == nil {
		t.Error("CostUSD counter is nil")
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration histogram is nil")
	}
	if m.ToolCallDuration == nil {
		t.Error("ToolCallDuration histogram is nil")
	}
	if m.ActiveSessions == nil {
		t.Error("ActiveSessions gauge is nil")
	}
	if m.BudgetRemaining == nil {
		t.Error("BudgetRemaining gauge is nil")
	}
}

func TestMetricsHandler(t *testing.T) {
	m := New()

	// Record some data
	m.RecordRequest("/invoke", 200)
	m.RecordRequest("/invoke", 500)
	m.RecordUsage(100, 50, 3, 0.01)
	m.ActiveSessions.Set(2)
	m.BudgetRemaining.Set(99.5)
	m.RequestDuration.WithLabelValues("/invoke").Observe(1.5)
	m.ToolCallDuration.Observe(0.2)

	// Serve metrics
	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	output := string(body)

	// Verify all metric families appear
	expectedMetrics := []string{
		"trove_requests_total",
		"trove_tool_calls_total",
		"trove_tool_call_errors_total",
		"trove_tokens_input_total",
		"trove_tokens_output_total",
		"trove_cost_usd_total",
		"trove_request_duration_seconds",
		"trove_tool_call_duration_seconds",
		"trove_active_sessions",
		"trove_budget_remaining_usd",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(output, metric) {
			t.Errorf("expected %q in metrics output", metric)
		}
	}
}

func TestRecordRequest_StatusBuckets(t *testing.T) {
	m := New()

	m.RecordRequest("/invoke", 200)
	m.RecordRequest("/invoke", 201)
	m.RecordRequest("/invoke", 400)
	m.RecordRequest("/invoke", 404)
	m.RecordRequest("/invoke", 500)
	m.RecordRequest("/invoke", 503)
	m.RecordRequest("/invoke", 302)

	// Serve and verify status labels exist
	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	output := string(body)

	for _, status := range []string{"2xx", "4xx", "5xx", "3xx"} {
		if !strings.Contains(output, status) {
			t.Errorf("expected status label %q in output", status)
		}
	}
}

func TestRecordUsage(t *testing.T) {
	m := New()

	m.RecordUsage(500, 200, 5, 0.05)
	m.RecordUsage(300, 100, 2, 0.02)

	// Serve metrics
	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Result().Body)
	output := string(body)

	// Tokens should be accumulated (800 input, 300 output)
	if !strings.Contains(output, "trove_tokens_input_total 800") {
		t.Errorf("expected tokens_input_total 800 in output")
	}
	if !strings.Contains(output, "trove_tokens_output_total 300") {
		t.Errorf("expected tokens_output_total 300 in output")
	}
}
