package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/guardrails"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/orchestrator"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/palette"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/provider"
	paletteclient "github.com/ceasarb/trovery-tools/internal/forge/palette/go-client"
	anthropicProvider "github.com/ceasarb/trovery-tools/internal/forge/agent/provider/anthropic"
	ollamaProvider "github.com/ceasarb/trovery-tools/internal/forge/agent/provider/ollama"
	openaiProvider "github.com/ceasarb/trovery-tools/internal/forge/agent/provider/openai"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/runtime"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/session"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/console"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/env"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/storage"
	"github.com/spf13/cobra"
)

var (
	chatMessage string
	chatSave    bool
	chatVerbose bool
	chatTrace   bool
)

var agentChatCmd = &cobra.Command{
	Use:   "chat [agent-name]",
	Short: "Start an interactive agent session",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentChat,
}

func runAgentChat(cmd *cobra.Command, args []string) error {
	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	// Load .env from workspace root / CWD
	env.LoadDotenv()

	if err := requireSupportedTuning(&cfg.Model); err != nil {
		return err
	}

	console.Header("Agent: " + cfg.Name)
	console.Dim(fmt.Sprintf("  Model: %s/%s", cfg.Model.Provider, cfg.Model.Model))

	// Initialize provider based on agent config
	var prov provider.Provider
	switch cfg.Model.Provider {
	case "openai":
		prov, err = openaiProvider.New()
	case "anthropic":
		prov, err = anthropicProvider.New()
	case "ollama":
		prov, err = ollamaProvider.New()
	default:
		err = fmt.Errorf("unsupported provider: %s (supported: anthropic, openai, ollama)", cfg.Model.Provider)
	}
	if err != nil {
		console.Error(fmt.Sprintf("Provider init: %v", err))
		return err
	}

	// If orchestrator, use orchestrator engine
	if cfg.IsOrchestrator() {
		return runOrchestratorChat(cfg, prov)
	}

	// Start servers
	mgr := servermgr.NewManager()
	wirer := newAgentToolWirer(providerFactoryFromFunc())
	mgr.SetAgentToolWirer(wirer)
	defer mgr.Close()

	ctx := context.Background()

	// Start Palette first so activity sink is wired before agent-as-tool servers
	var paletteSession *palette.Session
	if cfg.PaletteEnabled() {
		console.Dim("  Starting Palette viewer...")
		paletteServerDir := findPaletteServerDir()
		if paletteServerDir == "" {
			console.Warning("Palette enabled but palette/server/ not found — skipping")
		} else {
			ps, err := palette.StartSession(ctx, cfg.Palette, paletteServerDir)
			if err != nil {
				console.Error(fmt.Sprintf("  Palette: %v", err))
			} else {
				paletteSession = ps
				defer paletteSession.Close()
				mgr.RegisterToolProvider("palette", paletteSession.Provider.Tools(), paletteSession.Provider)
				// Wire activity sink so child agent events flow to the viewer
				wirer.activitySink = func(event, content string) {
					paletteSession.Client.SendActivity(event, content)
				}
				console.Dim(fmt.Sprintf("  ✓ Palette viewer on port %d (12 tools)", cfg.Palette.EffectivePort()))
			}
		}
	}

	if len(cfg.Servers) > 0 {
		console.Dim(fmt.Sprintf("  Starting %d server(s)...", len(cfg.Servers)))
		var serverErrors []string
		for _, s := range cfg.Servers {
			serverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			ms, err := mgr.StartServer(serverCtx, s, agentDir)
			cancel()
			if err != nil {
				console.Error(fmt.Sprintf("  Failed to start %s: %v", s.Name, err))
				serverErrors = append(serverErrors, s.Name)
				continue
			}
			console.Dim(fmt.Sprintf("  ✓ %s (%d tools)", s.Name, len(ms.Tools)))
		}
		if len(serverErrors) == len(cfg.Servers) {
			return fmt.Errorf("all servers failed to start")
		}
	}

	// Show available tools
	tools := mgr.AllTools(cfg.Settings.Namespacing)
	if len(tools) > 0 {
		console.Dim(fmt.Sprintf("  %d tool(s) available", len(tools)))
	}

	// Send agent identity to Palette viewer
	if paletteSession != nil {
		var subAgents []paletteclient.SubAgentInfo
		for _, s := range cfg.Servers {
			if s.IsAgentRef() {
				desc := s.Name
				// Try to load child agent description
				childDir := s.Agent
				if !filepath.IsAbs(childDir) {
					childDir = filepath.Join(agentDir, childDir)
				}
				if childCfg, err := agentcfg.Load(childDir); err == nil && childCfg.Expose != nil && childCfg.Expose.Description != "" {
					desc = childCfg.Expose.Description
				}
				subAgents = append(subAgents, paletteclient.SubAgentInfo{
					Name:        s.Name,
					Description: desc,
				})
			}
		}
		paletteSession.Client.SendAgentInfo(
			cfg.Name,
			cfg.Description,
			fmt.Sprintf("%s/%s", cfg.Model.Provider, cfg.Model.Model),
			subAgents,
		)
		// Open browser after agent info is cached so the viewer has metadata on first load
		paletteSession.OpenBrowserIfEnabled()
	}

	// Initialize session recorder
	rec, cleanupStore := initRecorder(cfg)
	if cleanupStore != nil {
		defer cleanupStore()
	}

	if rec.IsEnabled() {
		console.Dim(fmt.Sprintf("  Session recording: %s", rec.SessionID()))
	}

	fmt.Println()

	// Create runtime session
	sess := runtime.NewSession(cfg, prov, mgr)

	// Wire per-request budget if configured. Same fail-closed rule as `agent serve`: a
	// budget over an unpriced model can never trip, and this is the path where that costs
	// the most — chat enforces no monthly ceiling and records no cost, so an unpriced model
	// here is unmetered in every direction.
	if cfg.Settings.BudgetPerRequest > 0 {
		if err := requirePricedModel(cfg); err != nil {
			return err
		}
		budget := guardrails.New(cfg.Settings.BudgetPerRequest, 0, nil)
		sess.BudgetCheck = budget.CheckRequestBudget
		console.Dim(fmt.Sprintf("  Budget: $%.2f/request", cfg.Settings.BudgetPerRequest))
	}

	// Helper to send palette activity events (no-op when palette is off)
	sendActivity := func(event, content string) {
		if paletteSession != nil {
			paletteSession.Client.SendActivity(event, content)
		}
	}

	// Wire output hooks — forward real-time events to palette viewer
	defaultOutput := runtime.DefaultOutput()
	var initialThinking bool // tracks the manual "Agent is thinking..." sent before SendMessage
	sess.Output = runtime.Output{
		OnText: func(text string) {
			defaultOutput.OnText(text)
			// Resolve the initial thinking indicator on first stream output
			if initialThinking {
				initialThinking = false
				sendActivity("processing_stop", "Agent responded")
			}
		},
		OnToolStart: func(name string) {
			defaultOutput.OnToolStart(name)
			// Resolve the initial thinking indicator when a tool call starts
			if initialThinking {
				initialThinking = false
				sendActivity("processing_stop", "Agent responded")
			}
			sendActivity("tool_start", name)
		},
		OnToolResult: func(name, summary string, elapsed time.Duration) {
			defaultOutput.OnToolResult(name, summary, elapsed)
			sendActivity("tool_result", fmt.Sprintf("%s (%s)", name, elapsed.Round(time.Millisecond)))
		},
		OnDone: func() {
			defaultOutput.OnDone()
		},
		OnMaxTools: func() {
			defaultOutput.OnMaxTools()
			sendActivity("error", "Max tool calls reached")
		},
		OnProcessingStart: func(label string) {
			defaultOutput.OnProcessingStart(label)
			sendActivity("processing_start", label)
		},
		OnProcessingStop: func(label string, elapsed time.Duration, cost float64) {
			defaultOutput.OnProcessingStop(label, elapsed, cost)
			costStr := ""
			if cost > 0 {
				costStr = fmt.Sprintf(", ~$%.4f", cost)
			}
			sendActivity("processing_stop", fmt.Sprintf("%s (%s%s)", label, elapsed.Round(time.Millisecond), costStr))
		},
	}

	// Wire hooks for verbose, trace, and recording
	turnCount := 0
	sess.Hooks = runtime.Hooks{
		OnAssistantTurn: func(event runtime.TurnEvent) {
			turnCount++

			if chatVerbose {
				console.Dim(fmt.Sprintf("  [tokens] in=%d out=%d", event.TokensIn, event.TokensOut))
			}

			if chatTrace {
				cost := sess.EstimatedCost()
				console.Dim(fmt.Sprintf("  [TRACE] Turn %d: %d in, %d out, $%.4f", turnCount, event.TokensIn, event.TokensOut, cost))
			}

			if rec.IsEnabled() {
				_ = rec.RecordAssistantTurn(event.Text, event.TokensIn, event.TokensOut, sess.EstimatedCost())
			}
		},
		OnToolCall: func(event runtime.ToolCallEvent) {
			if chatVerbose {
				if event.Arguments != nil {
					data, _ := json.MarshalIndent(event.Arguments, "    ", "  ")
					console.Dim(fmt.Sprintf("    args: %s", string(data)))
				}
				console.Dim(fmt.Sprintf("    result: %s", event.Result))
			}

			if chatTrace {
				console.Dim(fmt.Sprintf("  [TRACE] Provider selected tool: %s", event.ToolName))
				console.Dim(fmt.Sprintf("  [TRACE] Tool call completed: %s in %dms", event.ToolName, event.Duration.Milliseconds()))
			}

			if rec.IsEnabled() {
				_ = rec.RecordToolCall(rec.CurrentTurnID(), session.ToolCallRecord{
					ToolName:   event.ToolName,
					ServerName: event.ServerName,
					Arguments:  event.Arguments,
					Result:     event.Result,
					Error:      event.Error,
					DurationMs: event.Duration.Milliseconds(),
				})
			}
		},
	}

	// Single message mode
	if chatMessage != "" {
		sendActivity("user_message", chatMessage)
		if rec.IsEnabled() {
			_ = rec.RecordUserTurn(chatMessage)
		}
		initialThinking = true
		sendActivity("processing_start", "Agent is thinking...")
		if err := sess.SendMessage(ctx, chatMessage); err != nil {
			return err
		}
		fmt.Println()
		finishSession(sess, rec)
		return nil
	}

	// Wire Palette bidirectional chat — messages from viewer inject into the agent
	paletteMessages := make(chan string, 10)
	if paletteSession != nil {
		paletteSession.Client.OnMessage(func(msg *paletteclient.UserMessage) {
			select {
			case paletteMessages <- msg.Content:
			default:
				// Drop if channel full — agent is busy
			}
		})
	}

	// Interactive mode
	console.Info("Chat started. Commands: /tools /servers /cost /history /clear /quit")
	fmt.Println()

	// Terminal input channel — read in goroutine so we can also select on palette messages
	terminalInput := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			terminalInput <- scanner.Text()
		}
		close(terminalInput)
	}()

	for {
		fmt.Print(console.HeaderStyle.Render("you> "))

		var input string
		var fromPalette bool

		select {
		case line, ok := <-terminalInput:
			if !ok {
				goto done
			}
			input = strings.TrimSpace(line)
		case msg := <-paletteMessages:
			input = strings.TrimSpace(msg)
			fromPalette = true
			if input != "" {
				console.Dim(fmt.Sprintf("[palette] %s", input))
			}
		}

		_ = fromPalette // used above for display

		if input == "" {
			continue
		}

		// Session commands
		switch input {
		case "/quit", "/exit", "/q", "quit", "exit":
			fmt.Println()
			finishSession(sess, rec)
			return nil

		case "/tools":
			for _, t := range mgr.AllTools(cfg.Settings.Namespacing) {
				console.Dim(fmt.Sprintf("  %s — %s", t.QualifiedName, t.Description))
			}
			continue

		case "/servers":
			for _, s := range cfg.Servers {
				console.Dim(fmt.Sprintf("  %s (%s)", s.Name, s.Path))
			}
			continue

		case "/cost":
			console.Dim(fmt.Sprintf("  Model:  %s/%s", cfg.Model.Provider, cfg.Model.Model))
			console.Dim(fmt.Sprintf("  Input:  %d tokens", sess.TotalInput))
			console.Dim(fmt.Sprintf("  Output: %d tokens", sess.TotalOutput))
			console.Dim(fmt.Sprintf("  Tools:  %d calls", sess.ToolCalls))
			console.Dim(fmt.Sprintf("  Cost:   ~$%.4f", sess.EstimatedCost()))
			continue

		case "/history":
			for _, m := range sess.Messages {
				role := m.Role
				text := ""
				for _, c := range m.Content {
					if c.Type == "text" && c.Text != "" {
						text = c.Text
					}
				}
				if text != "" {
					summary := text
					if len(summary) > 80 {
						summary = summary[:80] + "..."
					}
					console.Dim(fmt.Sprintf("  [%s] %s", role, summary))
				}
			}
			continue

		case "/clear":
			sess.Messages = nil
			console.Info("History cleared")
			continue
		}

		// Record user turn before sending
		sendActivity("user_message", input)
		if rec.IsEnabled() {
			_ = rec.RecordUserTurn(input)
		}

		// Send message
		initialThinking = true
		sendActivity("processing_start", "Agent is thinking...")
		fmt.Println()
		err = sess.SendMessage(ctx, input)
		if err != nil {
			console.Error(fmt.Sprintf("Error: %v", err))
			sendActivity("error", fmt.Sprintf("Error: %v", err))
		}
		// Resolve any dangling processing indicator
		if initialThinking {
			initialThinking = false
			sendActivity("processing_stop", "Agent responded")
		}
		sendActivity("tool_result", fmt.Sprintf("Done (tools: %d, ~$%.4f)", sess.ToolCalls, sess.EstimatedCost()))
		if sess.BudgetStopped {
			console.Warning(fmt.Sprintf("Per-request budget ($%.2f) exceeded — response truncated", cfg.Settings.BudgetPerRequest))
			sess.BudgetStopped = false // Reset for next message
		}
		fmt.Println()
	}

done:
	fmt.Println()
	finishSession(sess, rec)
	return nil
}

// initRecorder creates a session Recorder. Returns a disabled recorder if --save is not set.
func initRecorder(cfg *agentcfg.AgentConfig) (*session.Recorder, func()) {
	if !chatSave {
		return session.NewDisabled(), nil
	}

	// Create .trove/forge directory in CWD if needed
	troveDir := ".trove/forge"
	if err := os.MkdirAll(troveDir, 0o755); err != nil {
		console.Error(fmt.Sprintf("Cannot create %s: %v", troveDir, err))
		return session.NewDisabled(), nil
	}

	dbPath := filepath.Join(troveDir, "sessions.db")
	store, err := storage.NewSessionStore(dbPath)
	if err != nil {
		console.Error(fmt.Sprintf("Session store: %v", err))
		return session.NewDisabled(), nil
	}

	rec := session.New(store, cfg.Name, cfg.Model.Provider, cfg.Model.Model)
	cleanup := func() { store.Close() }
	return rec, cleanup
}

// finishSession prints the session summary and finalizes recording.
func finishSession(sess *runtime.Session, rec *session.Recorder) {
	summary := sess.Summary()
	console.Dim(summary)

	if rec.IsEnabled() {
		_ = rec.Finish(summary)
		console.Dim(fmt.Sprintf("  Session saved: %s", rec.SessionID()))
	}
}

func runOrchestratorChat(cfg *agentcfg.AgentConfig, prov provider.Provider) error {
	console.Dim(fmt.Sprintf("  Type: orchestrator (%d agents)", len(cfg.Orchestrator.Agents)))
	console.Dim(fmt.Sprintf("  Handoff: %s", cfg.Orchestrator.Handoff))
	fmt.Println()

	factory := func(childCfg *agentcfg.AgentConfig) (provider.Provider, error) {
		return initProvider(childCfg)
	}

	engine, err := orchestrator.New(cfg, prov, factory)
	if err != nil {
		return err
	}
	engine.SetAgentToolWirer(newAgentToolWirer(providerFactoryFromFunc()))
	engine.SetOutput(runtime.DefaultOutput())

	ctx := context.Background()

	// Single message mode for orchestrator
	if chatMessage != "" {
		result, err := engine.Execute(ctx, chatMessage)
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Println(result.Response)
		fmt.Println()
		console.Dim(fmt.Sprintf("  %d agents, %d tokens, ~$%.4f, %s",
			len(result.AgentResults), result.TotalTokens, result.TotalCost, result.Duration.Round(time.Millisecond)))
		return nil
	}

	// Interactive mode
	console.Info("Orchestrator chat. Commands: /dag /quit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(console.HeaderStyle.Render("you> "))
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch input {
		case "/quit", "/exit", "/q":
			return nil
		case "/dag":
			fmt.Print(engine.DAG().RenderASCII())
			continue
		}

		fmt.Println()
		result, err := engine.Execute(ctx, input)
		if err != nil {
			console.Error(fmt.Sprintf("Error: %v", err))
			fmt.Println()
			continue
		}

		fmt.Println()
		fmt.Println(result.Response)
		fmt.Println()
		console.Dim(fmt.Sprintf("  %d agents, %d tokens, ~$%.4f, %s",
			len(result.AgentResults), result.TotalTokens, result.TotalCost, result.Duration.Round(time.Millisecond)))
		fmt.Println()
	}

	return nil
}

// findPaletteServerDir locates the palette/server/ directory relative to the binary
// or common development paths.
func findPaletteServerDir() string {
	// Try relative to CWD (development)
	candidates := []string{
		"palette/server",
		"../palette/server",
	}

	// Try relative to executable
	if ex, err := os.Executable(); err == nil {
		dir := filepath.Dir(ex)
		candidates = append(candidates,
			filepath.Join(dir, "palette", "server"),
			filepath.Join(dir, "..", "palette", "server"),
		)
	}

	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs
		}
	}

	return ""
}

func init() {
	agentChatCmd.Flags().StringVarP(&chatMessage, "message", "m", "", "Single message (non-interactive)")
	agentChatCmd.Flags().BoolVar(&chatSave, "save", false, "Record session to SQLite (.trove/forge/sessions.db)")
	agentChatCmd.Flags().BoolVar(&chatVerbose, "verbose", false, "Show full tool call arguments and results")
	agentChatCmd.Flags().BoolVar(&chatTrace, "trace", false, "Show structured decision log")
	agentCmd.AddCommand(agentChatCmd)
}
