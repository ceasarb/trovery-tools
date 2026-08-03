package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/guardrails"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/httpserver"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/metrics"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/runtime"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/internal/forge/server/sandbox"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/auth"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/console"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/container"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/env"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/logging"
	forgeotel "github.com/ceasarb/trovery-tools/internal/forge/shared/otel"
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

	if err := requireSupportedTuning(&cfg.Model); err != nil {
		return err
	}

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
	budget, cleanupBudget, err := initBudget(cfg)
	if err != nil {
		return err
	}
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

// requirePricedModel refuses a configured budget over a model with no pricing entry.
//
// Shared by `agent serve`, `agent chat` and the eval runner so all three fail the same way.
// `agent chat` is where an unpriced model bites hardest: it enforces no monthly ceiling and
// records no cost, so there an unpriced model is unmetered in every direction at once.
func requirePricedModel(cfg *agentcfg.AgentConfig) error {
	if runtime.KnownModel(cfg.Model.Model) {
		return nil
	}
	return fmt.Errorf(
		"model %q has no pricing entry, so budget_per_request/budget_monthly can never "+
			"trip: every run would estimate at $0 and the ceiling you configured would "+
			"never fire.\nUse a priced model id (an alias like claude-opus-5, not a dated "+
			"snapshot), or remove the budget settings to serve deliberately unbounded",
		cfg.Model.Model)
}

// requireSupportedTuning refuses a model tuning parameter the configured model rejects.
//
// Shared by `agent serve`, `agent chat` and the eval runner, alongside requirePricedModel
// (ADR-010 §5). Unlike the budget check it is unconditional: a rejected parameter is not a
// guardrail that quietly fails to fire, it is a 400 on every single request. Refusing at
// startup converts a permanent runtime failure into one boot-time error that names the
// line to delete.
//
// Only models positively known to reject a parameter are refused — see
// runtime.Capabilities. An unrecognised model is left alone, so a model newer than this
// binary behaves exactly as it does today rather than failing to boot.
func requireSupportedTuning(m *agentcfg.ModelConfig) error {
	if m.Temperature > 0 && !runtime.SupportsTemperature(m.Model) {
		return fmt.Errorf(
			"model %q does not accept `temperature` — it was removed on Claude Opus 4.7 "+
				"and later, and sending it returns HTTP 400 on every request.\nRemove "+
				"`temperature: %g` from the model block in agent.yaml",
			m.Model, m.Temperature)
	}
	return nil
}

func initBudget(cfg *agentcfg.AgentConfig) (*guardrails.Budget, func(), error) {
	perReq := cfg.Settings.BudgetPerRequest
	monthly := cfg.Settings.BudgetMonthly

	if perReq <= 0 && monthly <= 0 {
		return nil, nil, nil
	}

	// A budget over an unpriced model is inert: every run estimates at $0, so neither
	// ceiling can ever trip. Refuse to start.
	//
	// This was a warning until 2026-07-29, and a warning is the wrong shape for it. The
	// operator configured a budget, which is a statement that runs must be bounded; we
	// cannot honour it, so continuing serves unbounded spend to someone who believes they
	// capped it. A warning scrolls past on boot and the service looks healthy forever
	// after. Downstream (mc-web ADR-028) had to build an independent Postgres ceiling in
	// part because this could not be relied on — fail closed so the next integrator can.
	if err := requirePricedModel(cfg); err != nil {
		return nil, nil, err
	}

	// Open the built-in SQLite cost store for monthly tracking.
	//
	// ledger stays an untyped nil when the store can't be opened: assigning a
	// nil *CostStore to the interface would produce a non-nil Ledger holding a
	// nil pointer, and Budget would call through it and panic instead of
	// treating monthly tracking as disabled.
	var store *guardrails.CostStore
	var ledger guardrails.Ledger
	if monthly > 0 {
		troveDir := ".trove/forge"
		os.MkdirAll(troveDir, 0o755)
		dbPath := filepath.Join(troveDir, "cost.db")
		opened, err := guardrails.NewCostStore(dbPath)
		if err != nil {
			console.Warning(fmt.Sprintf("Cost tracking unavailable — monthly cap will not be enforced: %v", err))
		} else {
			store, ledger = opened, opened
		}
	}

	budget := guardrails.New(perReq, monthly, ledger)
	cleanup := func() {
		if store != nil {
			store.Close()
		}
	}
	return budget, cleanup, nil
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
