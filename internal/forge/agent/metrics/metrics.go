package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "trove"

// Metrics holds all Prometheus metrics for the agent HTTP server.
type Metrics struct {
	Requests         *prometheus.CounterVec
	ToolCalls        prometheus.Counter
	ToolCallErrors   prometheus.Counter
	TokensInput      prometheus.Counter
	TokensOutput     prometheus.Counter
	CostUSD          prometheus.Counter
	RequestDuration  *prometheus.HistogramVec
	ToolCallDuration prometheus.Histogram
	ActiveSessions   prometheus.Gauge
	BudgetRemaining  prometheus.Gauge

	registry *prometheus.Registry
}

// New creates and registers all metrics with a dedicated registry.
func New() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_total",
			Help:      "Total number of agent invocation requests",
		}, []string{"endpoint", "status"}),

		ToolCalls: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "tool_calls_total",
			Help:      "Total number of tool calls executed",
		}),

		ToolCallErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "tool_call_errors_total",
			Help:      "Total number of tool calls that returned errors",
		}),

		TokensInput: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "tokens_input_total",
			Help:      "Total input tokens consumed",
		}),

		TokensOutput: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "tokens_output_total",
			Help:      "Total output tokens consumed",
		}),

		CostUSD: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cost_usd_total",
			Help:      "Total estimated cost in USD",
		}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
		}, []string{"endpoint"}),

		ToolCallDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "tool_call_duration_seconds",
			Help:      "Individual tool call duration in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10},
		}),

		ActiveSessions: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_sessions",
			Help:      "Number of currently active agent sessions",
		}),

		BudgetRemaining: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "budget_remaining_usd",
			Help:      "Remaining monthly budget in USD",
		}),

		registry: reg,
	}

	reg.MustRegister(
		m.Requests,
		m.ToolCalls,
		m.ToolCallErrors,
		m.TokensInput,
		m.TokensOutput,
		m.CostUSD,
		m.RequestDuration,
		m.ToolCallDuration,
		m.ActiveSessions,
		m.BudgetRemaining,
	)

	return m
}

// Handler returns an http.Handler that serves the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// RecordRequest records a completed request with endpoint and HTTP status.
func (m *Metrics) RecordRequest(endpoint string, status int) {
	statusStr := "2xx"
	switch {
	case status >= 500:
		statusStr = "5xx"
	case status >= 400:
		statusStr = "4xx"
	case status >= 300:
		statusStr = "3xx"
	}
	m.Requests.WithLabelValues(endpoint, statusStr).Inc()
}

// RecordUsage records token consumption, tool calls, and cost from a completed request.
func (m *Metrics) RecordUsage(inputTokens, outputTokens, toolCalls int, costUSD float64) {
	m.TokensInput.Add(float64(inputTokens))
	m.TokensOutput.Add(float64(outputTokens))
	m.CostUSD.Add(costUSD)
	for range toolCalls {
		m.ToolCalls.Inc()
	}
}
