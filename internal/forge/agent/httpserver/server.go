package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/guardrails"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/metrics"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/provider"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/auth"
	forgeotel "github.com/ceasarb/trovery-tools/internal/forge/shared/otel"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/ratelimit"
)

// Server is an HTTP API server for agent invocation.
type Server struct {
	cfg       *agentcfg.AgentConfig
	provider  provider.Provider
	serverMgr *servermgr.Manager
	budget    *guardrails.Budget
	metrics   *metrics.Metrics
	otel      *forgeotel.Provider
	agentDir  string
	host      string
	port      int
	ready     bool
	httpSrv   *http.Server
}

// Config holds HTTP server configuration.
type Config struct {
	AgentConfig *agentcfg.AgentConfig
	Provider    provider.Provider
	ServerMgr   *servermgr.Manager
	Budget      *guardrails.Budget    // optional, nil disables guardrails
	Metrics     *metrics.Metrics      // optional, nil disables metrics endpoint
	OTel        *forgeotel.Provider    // optional, nil disables tracing
	Auth        *auth.Config          // optional, nil disables auth
	AgentDir    string
	Host        string
	Port        int
}

// New creates a new HTTP server for agent invocation.
func New(cfg Config) *Server {
	s := &Server{
		cfg:       cfg.AgentConfig,
		provider:  cfg.Provider,
		serverMgr: cfg.ServerMgr,
		budget:    cfg.Budget,
		metrics:   cfg.Metrics,
		otel:      cfg.OTel,
		agentDir:  cfg.AgentDir,
		host:      cfg.Host,
		port:      cfg.Port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /invoke", s.handleInvoke)
	mux.HandleFunc("POST /invoke/stream", s.handleInvokeStream)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)

	// Register /metrics endpoint if metrics are enabled
	if s.metrics != nil {
		mux.Handle("GET /metrics", s.metrics.Handler())
	}

	var handler http.Handler = withCORS(mux)

	// Apply rate limiting (10 req/min for invoke endpoints)
	limiter := ratelimit.New(60, time.Minute)
	handler = limiter.Middleware()(handler)

	// Apply request body size limit (1MB)
	handler = ratelimit.MaxBodySize(1 << 20)(handler)

	// Apply auth middleware if configured
	if cfg.Auth != nil {
		handler = auth.Middleware(*cfg.Auth)(handler)
	}

	s.httpSrv = &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// MarkReady marks the server as ready to accept requests.
func (s *Server) MarkReady() {
	s.ready = true
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// Addr returns the address the server is configured to listen on.
func (s *Server) Addr() string {
	return s.httpSrv.Addr
}

// withCORS adds permissive CORS headers for local development.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "X-Trove-Cost, X-Trove-Tokens-In, X-Trove-Tokens-Out, X-Trove-Tool-Calls, X-Trove-Budget-Exceeded, X-Trove-Monthly-Remaining")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
