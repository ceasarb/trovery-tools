package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/eval"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/provider"
	anthropicProvider "github.com/ceasarb/demigo-tools/internal/forge/agent/provider/anthropic"
	ollamaProvider "github.com/ceasarb/demigo-tools/internal/forge/agent/provider/ollama"
	openaiProvider "github.com/ceasarb/demigo-tools/internal/forge/agent/provider/openai"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/runtime"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/env"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/storage"
	"github.com/ceasarb/demigo-tools/internal/forge/workspace"
	"github.com/spf13/cobra"
)

var (
	evalSuitePath      string
	evalRunsOverride   int
	evalReport         bool
	evalReportLast     bool
	evalFormat         string
	evalUpdateBaseline bool
)

var agentEvalCmd = &cobra.Command{
	Use:   "eval [agent-name]",
	Short: "Run eval suites against an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentEval,
}

func runAgentEval(cmd *cobra.Command, args []string) error {
	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	env.LoadDotenv()

	console.Header("Agent Eval: " + cfg.Name)
	console.Dim(fmt.Sprintf("  Model: %s/%s", cfg.Model.Provider, cfg.Model.Model))

	// Handle --report-last: generate report from last stored run without re-running
	if evalReportLast {
		ws, _ := workspace.Find(agentDir)
		var demiDir string
		if ws != nil {
			demiDir = filepath.Join(ws.Root, ".demi/forge")
		} else {
			demiDir = filepath.Join(agentDir, ".demi/forge")
		}
		dbPath := filepath.Join(demiDir, "eval.db")
		store, err := storage.NewEvalStore(dbPath)
		if err != nil {
			return fmt.Errorf("open eval store: %w", err)
		}
		defer store.Close()

		result, err := eval.LoadLastRun(store, cfg.Name)
		if err != nil {
			console.Error(err.Error())
			return err
		}

		console.Dim(fmt.Sprintf("  Loaded run %s (%s)", result.RunID[:8], result.SuiteName))
		fmt.Println()

		switch evalFormat {
		case "json":
			eval.WriteJSONReport(os.Stdout, result)
		default:
			eval.FormatTextReport(os.Stdout, result)
		}

		reportPath := filepath.Join(demiDir, fmt.Sprintf("%s-report.html", result.SuiteName))
		if err := eval.WriteHTMLReport(reportPath, result); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		console.Success("Report: " + reportPath)
		openBrowser(reportPath)
		return nil
	}

	// Discover eval suites
	suites, err := discoverSuites(agentDir)
	if err != nil {
		return err
	}

	if len(suites) == 0 {
		console.Warning("No eval suites found")
		console.Dim("  Create a suite at evals/<name>.eval.yaml")
		return nil
	}

	// A `max_cost_usd` assertion over a model with no pricing entry is an assertion that
	// cannot fail: every run prices at $0, so the check passes no matter what the suite
	// actually spent. That is worse than having no assertion, because the report says the
	// budget held. Same fail-closed rule as `agent serve` and `agent chat`.
	if err := requirePricedModelForSuites(cfg, suites); err != nil {
		return err
	}

	if err := requireSupportedTuningForSuites(cfg, suites); err != nil {
		return err
	}

	console.Dim(fmt.Sprintf("  Found %d suite(s)", len(suites)))
	fmt.Println()

	// Initialize provider
	prov, err := initProvider(cfg)
	if err != nil {
		console.Error(fmt.Sprintf("Provider init: %v", err))
		return err
	}

	// Start servers
	mgr := servermgr.NewManager()
	mgr.SetAgentToolWirer(newAgentToolWirer(providerFactoryFromFunc()))
	defer mgr.Close()

	ctx := context.Background()

	if len(cfg.Servers) > 0 {
		console.Dim(fmt.Sprintf("  Starting %d server(s)...", len(cfg.Servers)))
		for _, s := range cfg.Servers {
			serverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			ms, startErr := mgr.StartServer(serverCtx, s, agentDir)
			cancel()
			if startErr != nil {
				console.Error(fmt.Sprintf("  Failed to start %s: %v", s.Name, startErr))
				continue
			}
			console.Dim(fmt.Sprintf("  Started %s (%d tools)", s.Name, len(ms.Tools)))
		}
	}

	// Open eval store at workspace root
	ws, _ := workspace.Find(agentDir)
	var demiDir string
	if ws != nil {
		demiDir = filepath.Join(ws.Root, ".demi/forge")
	} else {
		demiDir = filepath.Join(agentDir, ".demi/forge")
	}
	dbPath := filepath.Join(demiDir, "eval.db")
	if err := os.MkdirAll(demiDir, 0o755); err != nil {
		return fmt.Errorf("create .demi/forge dir: %w", err)
	}

	store, err := storage.NewEvalStore(dbPath)
	if err != nil {
		return fmt.Errorf("open eval store: %w", err)
	}
	defer store.Close()

	engine := eval.New(store)
	engine.OnEvent = evalEventHandler

	// Build tool definitions from server manager
	tools := buildEvalToolDefs(mgr, cfg)

	// Run suites
	allPassed := true
	for _, suite := range suites {
		console.Header(fmt.Sprintf("Suite: %s", suite.Name))

		baseCaller := &eval.ServerManagerCaller{Mgr: mgr}

		result, runErr := engine.RunSuite(ctx, suite, cfg, prov, baseCaller, tools, evalRunsOverride)
		if runErr != nil {
			console.Error(fmt.Sprintf("Suite %s: %v", suite.Name, runErr))
			allPassed = false
			continue
		}

		// Output results
		switch evalFormat {
		case "json":
			eval.WriteJSONReport(os.Stdout, result)
		default:
			eval.FormatTextReport(os.Stdout, result)
		}

		if result.Status != "passed" {
			allPassed = false
		}

		// Generate HTML report
		if evalReport {
			reportPath := filepath.Join(agentDir, ".demi/forge", fmt.Sprintf("%s-report.html", suite.Name))
			if htmlErr := eval.WriteHTMLReport(reportPath, result); htmlErr != nil {
				console.Warning(fmt.Sprintf("Report write failed: %v", htmlErr))
			} else {
				console.Success("Report: " + reportPath)
			}
		}

		// Update baselines
		if evalUpdateBaseline {
			if blErr := engine.UpdateBaselines(suite, cfg.Name, result); blErr != nil {
				console.Warning(fmt.Sprintf("Baseline update failed: %v", blErr))
			} else {
				console.Success("Baselines updated")
			}
		}

		fmt.Println()
	}

	if !allPassed {
		return fmt.Errorf("one or more eval scenarios failed")
	}

	return nil
}

func evalEventHandler(e eval.Event) {
	switch e.Kind {
	case eval.EventScenarioStart:
		if e.TotalRuns > 1 {
			console.Dim(fmt.Sprintf("  ▸ %s (run %d/%d)", e.Scenario, e.Run, e.TotalRuns))
		} else {
			console.Dim(fmt.Sprintf("  ▸ %s", e.Scenario))
		}
	case eval.EventModelCall:
		console.Dim(fmt.Sprintf("    ↳ calling model... (%d tokens so far)", e.TokensUsed))
	case eval.EventToolCall:
		console.Dim(fmt.Sprintf("    ↳ tool: %s", e.ToolName))
	case eval.EventToolResult:
		if e.ToolError != "" {
			console.Dim(fmt.Sprintf("    ↳ tool: %s ✗ %dms (%s)", e.ToolName, e.ToolDuration.Milliseconds(), e.ToolError))
		} else {
			console.Dim(fmt.Sprintf("    ↳ tool: %s ✓ %dms", e.ToolName, e.ToolDuration.Milliseconds()))
		}
	case eval.EventScenarioEnd:
		if e.Passed {
			console.Success(fmt.Sprintf("  ✓ %s (%.1fs, %d tokens)", e.Scenario, e.Duration.Seconds(), e.TokensUsed))
		} else {
			console.Error(fmt.Sprintf("  ✗ %s (%.1fs, %d tokens)", e.Scenario, e.Duration.Seconds(), e.TokensUsed))
		}
	}
}

func discoverSuites(agentDir string) ([]*eval.Suite, error) {
	// If --suite flag is set, use that specific file
	if evalSuitePath != "" {
		suite, err := eval.LoadSuite(evalSuitePath)
		if err != nil {
			return nil, err
		}
		return []*eval.Suite{suite}, nil
	}

	// Look for *.eval.yaml in evals/
	evalsDir := filepath.Join(agentDir, "evals")
	entries, err := os.ReadDir(evalsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read evals dir: %w", err)
	}

	var suites []*eval.Suite
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".eval.yaml") {
			suite, loadErr := eval.LoadSuite(filepath.Join(evalsDir, e.Name()))
			if loadErr != nil {
				console.Warning(fmt.Sprintf("Skip %s: %v", e.Name(), loadErr))
				continue
			}
			suites = append(suites, suite)
		}
	}

	return suites, nil
}

func initProvider(cfg *agentcfg.AgentConfig) (provider.Provider, error) {
	switch cfg.Model.Provider {
	case "openai":
		return openaiProvider.New()
	case "anthropic":
		return anthropicProvider.New()
	case "ollama":
		return ollamaProvider.New()
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: anthropic, openai, ollama)", cfg.Model.Provider)
	}
}

func buildEvalToolDefs(mgr *servermgr.Manager, cfg *agentcfg.AgentConfig) []provider.ToolDef {
	nsTools := mgr.AllTools(cfg.Settings.Namespacing)
	defs := make([]provider.ToolDef, len(nsTools))
	for i, t := range nsTools {
		var schema interface{}
		if len(t.InputSchema) > 0 {
			json.Unmarshal(t.InputSchema, &schema)
		}
		defs[i] = provider.ToolDef{
			Name:        t.QualifiedName,
			Description: t.Description,
			InputSchema: schema,
		}
	}
	return defs
}

func init() {
	agentEvalCmd.Flags().StringVar(&evalSuitePath, "suite", "", "Path to specific eval suite YAML")
	agentEvalCmd.Flags().IntVar(&evalRunsOverride, "runs", 0, "Override multi-run count for all scenarios")
	agentEvalCmd.Flags().BoolVar(&evalReport, "report", false, "Generate HTML report")
	agentEvalCmd.Flags().BoolVar(&evalReportLast, "report-last", false, "Generate HTML report from last run (no re-run)")
	agentEvalCmd.Flags().StringVar(&evalFormat, "format", "text", "Output format: text or json")
	agentEvalCmd.Flags().BoolVar(&evalUpdateBaseline, "update-baselines", false, "Set current results as baselines")

	agentCmd.AddCommand(agentEvalCmd)
}

// requirePricedModelForSuites refuses to run when a suite asserts on cost but the model it
// would run has no pricing entry.
//
// Scoped to suites that actually assert on cost: an eval suite with no cost assertion is a
// correctness suite, and refusing to run it over an unpriced model would block work that
// never claimed to measure money. A suite-level `settings.model` override is checked too —
// it is the model that will really run.
func requirePricedModelForSuites(cfg *agentcfg.AgentConfig, suites []*eval.Suite) error {
	for _, suite := range suites {
		if !suiteAssertsCost(suite) {
			continue
		}
		model := cfg.Model.Model
		if suite.Settings.Model != "" {
			model = suite.Settings.Model
		}
		if runtime.KnownModel(model) {
			continue
		}
		return fmt.Errorf(
			"suite %q asserts max_cost_usd but model %q has no pricing entry: every run "+
				"would price at $0.0000 and the assertion could never fail.\nUse a priced "+
				"model id (an alias like claude-opus-5, not a dated snapshot), or drop the "+
				"cost assertion",
			suite.Name, model)
	}
	return nil
}

// requireSupportedTuningForSuites refuses to run when the agent's tuning parameters are
// rejected by the model a suite would actually run against.
//
// Unlike requirePricedModelForSuites this is not scoped to suites that opt in to
// anything: a rejected parameter 400s every request, so it breaks a correctness suite
// just as thoroughly as a cost one. The suite-level `settings.model` override matters
// here for the same reason it does there — it is the model that will really run, and it
// is the common way a suite ends up on a newer model than the agent config names.
//
// Note the temperature checked is always the agent config's. A suite's
// `settings.temperature` is parsed (eval.Settings) but never applied by the runner, so
// it cannot be what reaches the provider.
func requireSupportedTuningForSuites(cfg *agentcfg.AgentConfig, suites []*eval.Suite) error {
	for _, suite := range suites {
		model := cfg.Model.Model
		if suite.Settings.Model != "" {
			model = suite.Settings.Model
		}
		effective := cfg.Model
		effective.Model = model
		if err := requireSupportedTuning(&effective); err != nil {
			return fmt.Errorf("suite %q: %w", suite.Name, err)
		}
	}
	return nil
}

func suiteAssertsCost(suite *eval.Suite) bool {
	scenarios := suite.Scenarios
	scenarios = append(scenarios, suite.Cases...)
	for _, sc := range scenarios {
		for _, a := range sc.Assertions {
			if a.Type == "max_cost_usd" {
				return true
			}
		}
	}
	return false
}
