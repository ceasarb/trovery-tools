package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/guardrails"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/httpserver"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/metrics"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/demigo-tools/internal/forge/server/sandbox"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/auth"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/container"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/env"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/logging"
	forgeotel "github.com/ceasarb/demigo-tools/internal/forge/shared/otel"
	"github.com/spf13/cobra"
)

var (
	servePort    int
	serveHost    string
	serveToken   string
	serveNoAuth  bool
	serveSandbox bool
	servePolicy  string
	serveRuntime string
	logLevel     string
	logFormat    string
)

var agentServeCmd = &cobra.Command{
	Use:   "serve [agent-name]",
	Short: "Start an HTTP API server for agent invocation",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentServe,
}

func runAgentServe(cmd *cobra.Command, args []string) error {
	// Initialize structured logging
	logging.Init(logging.Config{
		Level:  logLevel,
		Format: logging.Format(logFormat),
	})
	log := logging.For(logging.ComponentHTTPServer)

	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	env.LoadDotenv()

	console.Header("Agent Serve: " + cfg.Name)
	console.Dim(fmt.Sprintf("  Model: %s/%s", cfg.Model.Provider, cfg.Model.Model))

	// Initialize provider
	prov, err := initProvider(cfg)
	if err != nil {
		console.Error(fmt.Sprintf("Provider init: %v", err))
		return err
	}

	// Resolve sandbox configuration if enabled
	var sbxCfg *servermgr.SandboxConfig
	if serveSandbox {
		rt, rtErr := resolveSandboxRuntime(serveRuntime)
		if rtErr != nil {
			console.Error(fmt.Sprintf("Sandbox runtime: %v", rtErr))
			return rtErr
		}

		policy, polErr := sandbox.ResolvePolicy(servePolicy)
		if polErr != nil {
			console.Error(fmt.Sprintf("Sandbox policy: %v", polErr))
			return polErr
		}

		sbxCfg = &servermgr.SandboxConfig{
			Runtime: rt,
			Policy:  policy,
		}
		console.Dim(fmt.Sprintf("  Sandbox: %s policy, %s runtime", policy.Name, rt.Name()))
		console.Dim(fmt.Sprintf("  Sandbox: memory=%dMB, cpus=%.1f, pids=%d, readonly=%v, network=%v",
			policy.MemoryMB, policy.CPUs, policy.PidsLimit, policy.ReadOnly, policy.Network))
	}

	// Start MCP servers
	mgr := servermgr.NewManager()
	mgr.SetAgentToolWirer(newAgentToolWirer(providerFactoryFromFunc()))
	defer mgr.Close()

	ctx := context.Background()

	if len(cfg.Servers) > 0 {
		if sbxCfg != nil {
			console.Dim(fmt.Sprintf("  Starting %d server(s) in sandbox...", len(cfg.Servers)))
		} else {
			console.Dim(fmt.Sprintf("  Starting %d server(s)...", len(cfg.Servers)))
		}
		if err := startServersWithSandbox(ctx, mgr, cfg, agentDir, sbxCfg); err != nil {
			return err
		}
	}

	tools := mgr.AllTools(cfg.Settings.Namespacing)
	if len(tools) > 0 {
		console.Dim(fmt.Sprintf("  %d tool(s) available", len(tools)))
	}
	fmt.Println()

	// Initialize budget guardrails if configured
	budget, cleanupBudget := initBudget(cfg)
	if cleanupBudget != nil {
		defer cleanupBudget()
	}

	if budget != nil {
		if cfg.Settings.BudgetPerRequest > 0 {
			console.Dim(fmt.Sprintf("  Budget: $%.2f/request", cfg.Settings.BudgetPerRequest))
		}
		if cfg.Settings.BudgetMonthly > 0 {
			remaining := budget.MonthlyRemaining()
			console.Dim(fmt.Sprintf("  Budget: $%.2f/month ($%.2f remaining)", cfg.Settings.BudgetMonthly, remaining))
		}
	}

	// Initialize authentication
	authCfg := resolveAuthConfig(serveToken, serveNoAuth, serveHost)

	// Initialize Prometheus metrics
	m := metrics.New()
	log.Info("metrics enabled", "endpoint", "/metrics")

	// Initialize OTel tracing
	var otelProvider *forgeotel.Provider
	if cfg.OTel != nil && cfg.OTel.Enabled {
		otelProvider, err = forgeotel.Init(ctx, forgeotel.Config{
			Enabled:  cfg.OTel.Enabled,
			Endpoint: cfg.OTel.Endpoint,
			Protocol: cfg.OTel.Protocol,
			Insecure: cfg.OTel.Insecure,
		})
		if err != nil {
			console.Warning(fmt.Sprintf("OTel init failed: %v", err))
			log.Warn("otel initialization failed", "error", err)
		} else {
			defer func() {
				shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				otelProvider.Shutdown(shutCtx)
			}()
			console.Dim(fmt.Sprintf("  OTel: tracing to %s (%s)", cfg.OTel.Endpoint, cfg.OTel.Protocol))
		}
	}

	// Create HTTP server
	srv := httpserver.New(httpserver.Config{
		AgentConfig: cfg,
		Provider:    prov,
		ServerMgr:   mgr,
		Budget:      budget,
		Metrics:     m,
		OTel:        otelProvider,
		Auth:        authCfg,
		AgentDir:    agentDir,
		Host:        serveHost,
		Port:        servePort,
	})
	srv.MarkReady()

	// Print endpoint info
	addr := fmt.Sprintf("http://%s", srv.Addr())
	console.Success(fmt.Sprintf("Listening on %s", addr))
	fmt.Println()
	console.Dim("  Endpoints:")
	console.Dim(fmt.Sprintf("    POST %s/invoke          Synchronous invocation", addr))
	console.Dim(fmt.Sprintf("    POST %s/invoke/stream    SSE streaming", addr))
	console.Dim(fmt.Sprintf("    GET  %s/health           Liveness check", addr))
	console.Dim(fmt.Sprintf("    GET  %s/ready            Readiness check", addr))
	console.Dim(fmt.Sprintf("    GET  %s/metrics          Prometheus metrics", addr))
	fmt.Println()
	console.Dim("  Press Ctrl+C to stop")
	fmt.Println()

	log.Info("server started", "addr", addr, "agent", cfg.Name)

	// Graceful shutdown on interrupt
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		console.Error(fmt.Sprintf("Server error: %v", err))
		log.Error("server error", "error", err)
		return err
	case <-stop:
		fmt.Println()
		console.Dim("Shutting down...")
		log.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			console.Error(fmt.Sprintf("Shutdown error: %v", err))
			log.Error("shutdown error", "error", err)
		}
		console.Success("Server stopped")
	}

	return nil
}

// startServersWithSandbox starts MCP servers, using sandbox containers for
// bundled servers (path/command) when sandbox config is provided. Remote (url)
// and agent-as-tool servers always start normally.
func startServersWithSandbox(ctx context.Context, mgr *servermgr.Manager, cfg *agentcfg.AgentConfig, agentDir string, sbx *servermgr.SandboxConfig) error {
	for _, s := range cfg.Servers {
		// Use longer timeout for sandbox (image build can take time)
		timeout := 30 * time.Second
		if sbx != nil && !s.IsAgentRef() && s.URL == "" {
			timeout = 120 * time.Second
		}

		serverCtx, cancel := context.WithTimeout(ctx, timeout)
		var ms *servermgr.ManagedServer
		var err error

		if sbx != nil && !s.IsAgentRef() && s.URL == "" {
			// Sandbox: run bundled server in a container
			ms, err = mgr.StartServerSandboxed(serverCtx, s, agentDir, sbx)
			if err == nil {
				console.Dim(fmt.Sprintf("  ✓ %s (%d tools) [sandboxed: %s]", s.Name, len(ms.Tools), sbx.Policy.Name))
			}
		} else {
			// Normal: direct subprocess or remote connection
			ms, err = mgr.StartServer(serverCtx, s, agentDir)
			if err == nil {
				label := ""
				if s.URL != "" {
					label = " [remote]"
				} else if s.IsAgentRef() {
					label = " [agent]"
				}
				console.Dim(fmt.Sprintf("  ✓ %s (%d tools)%s", s.Name, len(ms.Tools), label))
			}
		}
		cancel()

		if err != nil {
			console.Error(fmt.Sprintf("  Failed to start %s: %v", s.Name, err))
			continue
		}
	}
	return nil
}

// resolveSandboxRuntime detects or resolves the container runtime for sandbox mode.
func resolveSandboxRuntime(override string) (container.Runtime, error) {
	if override != "" {
		return container.RuntimeByName(override)
	}
	return container.DetectRuntime()
}

func initBudget(cfg *agentcfg.AgentConfig) (*guardrails.Budget, func()) {
	perReq := cfg.Settings.BudgetPerRequest
	monthly := cfg.Settings.BudgetMonthly

	if perReq <= 0 && monthly <= 0 {
		return nil, nil
	}

	// Open cost store for monthly tracking
	var store *guardrails.CostStore
	if monthly > 0 {
		demiDir := ".demi/forge"
		os.MkdirAll(demiDir, 0o755)
		dbPath := filepath.Join(demiDir, "cost.db")
		var err error
		store, err = guardrails.NewCostStore(dbPath)
		if err != nil {
			console.Warning(fmt.Sprintf("Cost tracking unavailable: %v", err))
		}
	}

	budget := guardrails.New(perReq, monthly, store)
	cleanup := func() {
		if store != nil {
			store.Close()
		}
	}
	return budget, cleanup
}

func resolveAuthConfig(token string, noAuth bool, host string) *auth.Config {
	if noAuth {
		console.Warning("Running without authentication — do not expose to network")
		return &auth.Config{Disabled: true}
	}

	// Check flag, then env var
	if token == "" {
		token = os.Getenv("AJNT_AUTH_TOKEN")
	}

	// Auto-generate if not provided
	if token == "" {
		var err error
		token, err = auth.GenerateToken()
		if err != nil {
			console.Warning(fmt.Sprintf("Could not generate auth token: %v", err))
			return &auth.Config{Disabled: true}
		}
		console.Info("Auth token (auto-generated):")
		console.Dim("  " + token)
		fmt.Println()
	}

	// Warn if binding to non-localhost
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		console.Warning(fmt.Sprintf("Binding to %s — ensure auth token is set", host))
	}

	return &auth.Config{Token: token}
}

func init() {
	agentServeCmd.Flags().IntVar(&servePort, "port", 8080, "Port to listen on")
	agentServeCmd.Flags().StringVar(&serveHost, "host", "localhost", "Host to bind to")
	agentServeCmd.Flags().StringVar(&serveToken, "token", "", "Auth bearer token (default: auto-generated)")
	agentServeCmd.Flags().BoolVar(&serveNoAuth, "no-auth", false, "Disable authentication (dev only)")
	agentServeCmd.Flags().BoolVar(&serveSandbox, "sandbox", false, "Run bundled MCP servers in sandboxed containers")
	agentServeCmd.Flags().StringVar(&servePolicy, "policy", "standard", "Sandbox security policy: strict, standard, permissive, or path to YAML")
	agentServeCmd.Flags().StringVar(&serveRuntime, "runtime", "", "Container runtime override: docker, podman, or nerdctl (default: auto-detect)")
	agentServeCmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	agentServeCmd.Flags().StringVar(&logFormat, "log-format", "text", "Log format: json, text")

	agentCmd.AddCommand(agentServeCmd)
}
