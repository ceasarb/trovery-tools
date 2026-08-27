package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ceasarb/trovery-tools/internal/vigil/adapters"
	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/policy"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
	"github.com/ceasarb/trovery-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [tool] [-- command...]",
	Short: "Launch a tool within the active session",
	Long: `Launch a named tool adapter or any CLI command inside the active session.

Examples:
  trove vigil run claude-code
  trove vigil run codex
  trove vigil run -- pytest -x
  trove vigil run -- npm install`,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: false,
	RunE:               runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	cfg, projectRoot, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	mgr := session.NewSessionManager(cfg, projectRoot)

	// Verify active session.
	active, _ := mgr.GetActive()
	if active == nil {
		return session.ErrNoActiveSession
	}

	// Determine adapter.
	var adapter adapters.Adapter
	var toolCfg config.ToolConfig
	var genericCmd []string

	if len(args) == 0 {
		return fmt.Errorf("no tool specified — use `trove vigil run <tool>` or `trove vigil run -- <command>`")
	}

	// Check if this is a named adapter.
	toolName := args[0]
	if a := adapters.ResolveAdapter(toolName, nil); a != nil {
		adapter = a
		tc, ok := cfg.Tools[toolName]
		if ok {
			if !tc.Enabled {
				reason := "no reason given"
				if tc.Reason != "" {
					reason = tc.Reason
				}
				return fmt.Errorf("%s is disabled: %s", toolName, reason)
			}
			toolCfg = tc
		}
	} else {
		// Treat all args as a generic command.
		genericCmd = args
		adapter = adapters.NewGenericAdapter(genericCmd)
		toolCfg = config.ToolConfig{Enabled: true}
	}

	// Validate tool is installed.
	if err := adapter.Validate(); err != nil {
		return fmt.Errorf("%s: %v\n  Install: %s", adapter.Name(), err, adapter.InstallHint())
	}

	console.Dim(fmt.Sprintf("Running %s...", adapter.Name()))

	// Start real-time file watcher.
	var fw *policy.FileWatcher
	secretsScanner, scanErr := policy.NewSecretsScanner(cfg.Policies.Secrets, projectRoot)
	if scanErr != nil {
		slog.Warn("could not initialize secrets scanner for watcher", "error", scanErr)
	} else {
		fw, err = policy.NewFileWatcher(projectRoot, secretsScanner)
		if err != nil {
			slog.Warn("could not start file watcher", "error", err)
		} else {
			if err := fw.Start(); err != nil {
				slog.Warn("file watcher start failed", "error", err)
				fw = nil
			}
		}
	}

	// Set up signal forwarding so Ctrl+C kills the child but keeps the session.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		// Signal received — the child process gets it too via process group.
		// We just suppress our own exit.
	}()
	defer signal.Stop(sigCh)

	// Run the tool, passing anything after the tool name through to it
	// (e.g. `trove vigil run lumi polish this note`).
	var extraArgs []string
	if len(genericCmd) == 0 && len(args) > 1 {
		extraArgs = args[1:]
	}
	toolRun, err := adapter.Run(toolCfg, extraArgs)
	if err != nil {
		console.Error(fmt.Sprintf("Tool error: %v", err))
		// Still record the run.
	}

	// Stop file watcher and collect real-time findings.
	if fw != nil {
		watcherChanges, watcherViolations := fw.Stop()
		if len(watcherViolations) > 0 {
			console.Warning(fmt.Sprintf("File watcher detected %d violation(s) during run", len(watcherViolations)))
		}
		if len(watcherChanges) > 0 {
			slog.Debug("file watcher detected changes", "count", len(watcherChanges))
		}
		// Real-time violations are informational — post-session audit is authoritative.
		_ = watcherChanges
	}

	if toolRun != nil {
		if err := mgr.AddToolRun(*toolRun); err != nil {
			console.Warning(fmt.Sprintf("Could not record tool run: %v", err))
		}

		if toolRun.ExitCode != 0 {
			console.Warning(fmt.Sprintf("Tool exited with code %d", toolRun.ExitCode))
		}
	}

	return nil
}
