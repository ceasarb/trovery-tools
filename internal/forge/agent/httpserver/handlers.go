package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/guardrails"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/runtime"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/delegation"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// InvokeRequest is the request body for /invoke and /invoke/stream.
type InvokeRequest struct {
	Message     string   `json:"message"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TimeoutSecs *int     `json:"timeout_secs,omitempty"`
	// OnBehalfOf is an opaque, per-request delegated-identity assertion (ADR-008).
	// Forge does not parse, validate, or trust it — it propagates the value to
	// every tool call as MCP `_meta`. Absent ⇒ today's behavior exactly.
	OnBehalfOf string `json:"on_behalf_of,omitempty"`
}

// InvokeResponse is the response body for /invoke.
type InvokeResponse struct {
	Response  string       `json:"response"`
	Usage     UsageInfo    `json:"usage"`
	ToolCalls int          `json:"tool_calls"`
	CostUSD   float64      `json:"cost_usd"`
	Model     string       `json:"model"`
}

// UsageInfo tracks token consumption in responses.
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// SSE event types for /invoke/stream.
const (
	sseText       = "text"
	sseToolStart  = "tool_start"
	sseToolResult = "tool_result"
	sseDone       = "done"
	sseError      = "error"
)

// handleHealth returns 200 OK unconditionally (liveness).
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleReady returns 200 if the server is initialized and ready, 503 otherwise.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

// handleInvoke processes a single message synchronously and returns the full response.
func (s *Server) handleInvoke(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Start OTel span if tracing is enabled
	ctx := r.Context()
	if s.otel != nil {
		var span trace.Span
		ctx, span = s.otel.StartSpan(ctx, "agent.invoke")
		defer span.End()
	}

	req, err := decodeInvokeRequest(r)
	if err != nil {
		s.recordRequestMetrics("/invoke", http.StatusBadRequest, start)
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check monthly budget before processing
	if rejected := s.checkMonthlyBudget(w); rejected {
		s.recordRequestMetrics("/invoke", http.StatusTooManyRequests, start)
		return
	}

	if s.metrics != nil {
		s.metrics.ActiveSessions.Inc()
		defer s.metrics.ActiveSessions.Dec()
	}

	cfg, err := s.applyOverrides(req)
	if err != nil {
		s.recordRequestMetrics("/invoke", http.StatusBadRequest, start)
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TimeoutSecs != nil && *req.TimeoutSecs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*req.TimeoutSecs)*time.Second)
		defer cancel()
	}

	// Carry the per-request delegated identity to every tool call (ADR-008).
	ctx = delegation.WithOnBehalfOf(ctx, req.OnBehalfOf)

	sess := runtime.NewSession(cfg, s.provider, s.serverMgr)
	sess.Output = runtime.SilentOutput()

	// Ledger the spend on every exit path, not just success. A timeout, a
	// provider error, or a tool failure still burned the tokens consumed up to
	// that point, and a monthly ceiling that only counts successful runs is not
	// a ceiling.
	defer s.recordCost(sess)

	// Wire per-request budget check
	s.wireBudgetCheck(sess)

	// Collect response text via output handler
	var responseText string
	sess.Output.OnText = func(text string) { responseText += text }

	if err := sess.SendMessage(ctx, req.Message); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			s.recordRequestMetrics("/invoke", http.StatusGatewayTimeout, start)
			writeJSONError(w, http.StatusGatewayTimeout, "request timeout exceeded")
			return
		}
		s.recordRequestMetrics("/invoke", http.StatusInternalServerError, start)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.recordUsageMetrics(sess)
	s.recordRequestMetrics("/invoke", http.StatusOK, start)

	// Add OTel attributes
	if s.otel != nil {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(
			attribute.Int("demi.tokens_in", sess.TotalInput),
			attribute.Int("demi.tokens_out", sess.TotalOutput),
			attribute.Int("demi.tool_calls", sess.ToolCalls),
			attribute.Float64("demi.cost_usd", sess.EstimatedCost()),
		)
	}

	setCostHeaders(w, sess)
	if sess.BudgetStopped {
		w.Header().Set("X-Demi-Budget-Exceeded", "true")
	}

	resp := InvokeResponse{
		Response:  responseText,
		Usage:     UsageInfo{InputTokens: sess.TotalInput, OutputTokens: sess.TotalOutput},
		ToolCalls: sess.ToolCalls,
		CostUSD:   sess.EstimatedCost(),
		Model:     cfg.Model.Model,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleInvokeStream processes a message with SSE streaming.
func (s *Server) handleInvokeStream(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ctx := r.Context()
	if s.otel != nil {
		var span trace.Span
		ctx, span = s.otel.StartSpan(ctx, "agent.invoke.stream")
		defer span.End()
	}

	req, err := decodeInvokeRequest(r)
	if err != nil {
		s.recordRequestMetrics("/invoke/stream", http.StatusBadRequest, start)
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check monthly budget before processing
	if rejected := s.checkMonthlyBudget(w); rejected {
		s.recordRequestMetrics("/invoke/stream", http.StatusTooManyRequests, start)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.recordRequestMetrics("/invoke/stream", http.StatusInternalServerError, start)
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	if s.metrics != nil {
		s.metrics.ActiveSessions.Inc()
		defer s.metrics.ActiveSessions.Dec()
	}

	cfg, err := s.applyOverrides(req)
	if err != nil {
		s.recordRequestMetrics("/invoke/stream", http.StatusBadRequest, start)
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TimeoutSecs != nil && *req.TimeoutSecs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*req.TimeoutSecs)*time.Second)
		defer cancel()
	}

	// Carry the per-request delegated identity to every tool call (ADR-008).
	ctx = delegation.WithOnBehalfOf(ctx, req.OnBehalfOf)

	// Set SSE headers before creating session
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sess := runtime.NewSession(cfg, s.provider, s.serverMgr)
	sess.Output = runtime.SilentOutput()

	// Ledger the spend on every exit path — including a client that disconnects
	// mid-stream, which surfaces here as a SendMessage error after tokens were
	// already spent.
	defer s.recordCost(sess)

	// Wire per-request budget check
	s.wireBudgetCheck(sess)

	var responseText string

	// Wire output to SSE events
	sess.Output.OnText = func(text string) {
		responseText += text
		writeSSE(w, flusher, sseText, map[string]string{"text": text})
	}
	sess.Output.OnToolStart = func(name string) {
		writeSSE(w, flusher, sseToolStart, map[string]string{"tool": name})
	}
	sess.Output.OnToolResult = func(name, summary string, elapsed time.Duration) {
		writeSSE(w, flusher, sseToolResult, map[string]string{"tool": name, "summary": summary})
	}

	if err := sess.SendMessage(ctx, req.Message); err != nil {
		writeSSE(w, flusher, sseError, map[string]string{"error": err.Error()})
		s.recordRequestMetrics("/invoke/stream", http.StatusInternalServerError, start)
		return
	}

	s.recordUsageMetrics(sess)
	s.recordRequestMetrics("/invoke/stream", http.StatusOK, start)

	// Final done event with usage summary
	doneData := map[string]any{
		"response":        responseText,
		"usage":           UsageInfo{InputTokens: sess.TotalInput, OutputTokens: sess.TotalOutput},
		"tool_calls":      sess.ToolCalls,
		"cost_usd":        sess.EstimatedCost(),
		"model":           cfg.Model.Model,
		"budget_exceeded": sess.BudgetStopped,
	}
	writeSSE(w, flusher, sseDone, doneData)
}

// decodeInvokeRequest parses and validates the request body.
func decodeInvokeRequest(r *http.Request) (*InvokeRequest, error) {
	var req InvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if req.Message == "" {
		return nil, fmt.Errorf("message is required")
	}
	// Clamp timeout to safe range [1, 3600]
	if req.TimeoutSecs != nil {
		if *req.TimeoutSecs < 1 {
			v := 1
			req.TimeoutSecs = &v
		}
		if *req.TimeoutSecs > 3600 {
			v := 3600
			req.TimeoutSecs = &v
		}
	}
	return &req, nil
}

// applyOverrides creates a copy of the agent config with request-level overrides applied.
//
// Returns an error when an override names a parameter the served model rejects, so the
// caller can answer 400 rather than forwarding a request the provider will refuse. The
// config-level equivalent is refused at startup (ADR-010 §5), but an override arrives
// per request and has to be caught here. Validation lives with application rather than
// in decodeInvokeRequest because only this function knows the effective model.
func (s *Server) applyOverrides(req *InvokeRequest) (*agentcfg.AgentConfig, error) {
	cfg := *s.cfg
	cfg.Model = s.cfg.Model

	if req.MaxTokens != nil {
		cfg.Model.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		if *req.Temperature > 0 && !runtime.SupportsTemperature(cfg.Model.Model) {
			return nil, fmt.Errorf(
				"model %q does not accept `temperature`: remove it from the request body",
				cfg.Model.Model)
		}
		cfg.Model.Temperature = *req.Temperature
	}
	return &cfg, nil
}

// setCostHeaders writes cost-tracking headers to the response.
func setCostHeaders(w http.ResponseWriter, sess *runtime.Session) {
	w.Header().Set("X-Demi-Cost", fmt.Sprintf("%.6f", sess.EstimatedCost()))
	w.Header().Set("X-Demi-Tokens-In", strconv.Itoa(sess.TotalInput))
	w.Header().Set("X-Demi-Tokens-Out", strconv.Itoa(sess.TotalOutput))
	w.Header().Set("X-Demi-Tool-Calls", strconv.Itoa(sess.ToolCalls))
}

// writeSSE writes a single SSE event to the response.
func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// checkMonthlyBudget returns true (and writes a 429 response) if the monthly cap is reached.
func (s *Server) checkMonthlyBudget(w http.ResponseWriter) bool {
	if s.budget == nil {
		return false
	}
	remaining, err := s.budget.CheckMonthlyBudget()
	if err == guardrails.ErrMonthlyCapReached {
		w.Header().Set("X-Demi-Monthly-Remaining", "0.000000")
		writeJSONError(w, http.StatusTooManyRequests, "monthly budget cap reached")
		return true
	}
	if err != nil {
		// Non-fatal: log but don't block
		return false
	}
	if remaining > 0 {
		w.Header().Set("X-Demi-Monthly-Remaining", fmt.Sprintf("%.6f", remaining))
	}
	return false
}

// wireBudgetCheck attaches a per-request budget checker to the session if configured.
func (s *Server) wireBudgetCheck(sess *runtime.Session) {
	if s.budget == nil || s.budget.PerRequest <= 0 {
		return
	}
	sess.BudgetCheck = s.budget.CheckRequestBudget
}

// recordCost persists the session cost for monthly tracking.
func (s *Server) recordCost(sess *runtime.Session) {
	if s.budget == nil {
		return
	}
	cost := sess.EstimatedCost()
	if cost > 0 {
		_ = s.budget.RecordCost(s.cfg.Name, cost)
	}
}

// recordRequestMetrics records request-level metrics if metrics are enabled.
func (s *Server) recordRequestMetrics(endpoint string, status int, start time.Time) {
	if s.metrics == nil {
		return
	}
	s.metrics.RecordRequest(endpoint, status)
	s.metrics.RequestDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
}

// recordUsageMetrics records token and cost metrics from a completed session.
func (s *Server) recordUsageMetrics(sess *runtime.Session) {
	if s.metrics == nil {
		return
	}
	s.metrics.RecordUsage(sess.TotalInput, sess.TotalOutput, sess.ToolCalls, sess.EstimatedCost())
}
