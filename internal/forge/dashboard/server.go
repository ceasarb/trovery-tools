package dashboard

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"

	"time"

	"github.com/ceasarb/demigo-tools/internal/forge/dashboard/ws"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/auth"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/ratelimit"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/storage"
)

// Server is the dashboard HTTP server providing API endpoints and a static SPA.
type Server struct {
	port       int
	evalStore  *storage.EvalStore
	sessStore  *storage.SessionStore
	hub        *ws.Hub
	logger     *slog.Logger
	authCfg    *auth.Config
	workDir    string
	httpServer *http.Server
}

// New creates a dashboard server rooted at workDir. It opens the eval and session
// SQLite databases from the .demi/forge/ directory.
// Option configures the dashboard server.
type Option func(*Server)

// WithAuth sets the auth config for the dashboard.
func WithAuth(cfg *auth.Config) Option {
	return func(s *Server) { s.authCfg = cfg }
}

func New(port int, workDir string, opts ...Option) (*Server, error) {
	demiDir := filepath.Join(workDir, ".demi/forge")

	evalStore, err := storage.NewEvalStore(filepath.Join(demiDir, "evals.db"))
	if err != nil {
		return nil, fmt.Errorf("open eval store: %w", err)
	}

	sessStore, err := storage.NewSessionStore(filepath.Join(demiDir, "sessions.db"))
	if err != nil {
		evalStore.Close()
		return nil, fmt.Errorf("open session store: %w", err)
	}

	logger := slog.Default().With("component", "dashboard")
	hub := ws.NewHub(logger)
	go hub.Run()

	s := &Server{
		port:      port,
		evalStore: evalStore,
		sessStore: sessStore,
		hub:       hub,
		logger:    logger,
		workDir:   workDir,
	}

	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	var handler http.Handler = mux
	handler = jsonContentType(handler)
	handler = cors(handler)
	limiter := ratelimit.New(100, time.Minute)
	handler = limiter.Middleware()(handler)
	handler = ratelimit.MaxBodySize(1 << 20)(handler)
	if s.authCfg != nil {
		handler = auth.Middleware(*s.authCfg)(handler)
	}
	handler = requestLogger(handler)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	return s, nil
}

// Start begins listening and serving. It blocks until the server is shut down.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Hub returns the event hub for external publishers (e.g., eval runner).
func (s *Server) Hub() *ws.Hub {
	return s.hub
}

// Shutdown gracefully shuts down the server and closes database connections.
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	s.evalStore.Close()
	s.sessStore.Close()
	return err
}

// registerRoutes wires all API and static routes onto the mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// API routes — servers
	mux.HandleFunc("GET /api/servers", s.handleListServers)
	mux.HandleFunc("GET /api/servers/{name}", s.handleGetServer)

	// API routes — agents
	mux.HandleFunc("GET /api/agents", s.handleListAgents)
	mux.HandleFunc("GET /api/agents/{name}", s.handleGetAgent)

	// API routes — evals
	mux.HandleFunc("GET /api/evals", s.handleListEvals)
	mux.HandleFunc("GET /api/evals/{id}", s.handleGetEval)

	// API routes — sessions
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("GET /api/sessions/{id}/export", s.handleExportSession)

	// API routes — analytics
	mux.HandleFunc("GET /api/analytics/cost", s.handleAnalyticsCost)
	mux.HandleFunc("GET /api/analytics/tokens", s.handleAnalyticsTokens)
	mux.HandleFunc("GET /api/analytics/errors", s.handleAnalyticsErrors)
	mux.HandleFunc("GET /api/analytics/latency", s.handleAnalyticsLatency)
	mux.HandleFunc("GET /api/analytics/usage", s.handleAnalyticsUsage)

	// API routes — live tools
	mux.HandleFunc("GET /api/tools", s.handleListTools)
	mux.HandleFunc("POST /api/tools/call", s.handleCallTool)

	// WebSocket routes — real-time
	mux.HandleFunc("GET /ws/events", s.handleWSEvents)
	mux.HandleFunc("GET /ws/chat", s.handleWSChat)
	mux.HandleFunc("GET /ws/eval", s.handleWSEval)

	// Static SPA — serve embedded files, fallback to index.html
	staticSub, _ := fs.Sub(staticFS, "static")
	fileServer := http.FileServer(http.FS(staticSub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try serving the exact file first
		if r.URL.Path != "/" {
			f, err := staticSub.Open(r.URL.Path[1:]) // strip leading /
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
