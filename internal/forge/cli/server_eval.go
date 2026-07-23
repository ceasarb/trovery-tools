package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ceasarb/demigo-tools/internal/forge/server/eval"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/config"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/storage"
	"github.com/ceasarb/demigo-tools/internal/forge/workspace"
	"github.com/spf13/cobra"
)

var (
	srvEvalSuitePath      string
	srvEvalUpdateBaseline bool
	srvEvalUpdateSnapshot bool
	srvEvalReport         bool
	srvEvalFormat         string
)

var serverEvalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Run eval suites against a server",
	RunE:  runServerEval,
}

func runServerEval(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Load server config to verify we're in a server directory
	cfg, err := config.LoadServerConfig(cwd)
	if err != nil {
		console.Error(fmt.Sprintf("No demi.toml found: %v", err))
		console.Dim("  Run this command from inside a server directory")
		return err
	}

	console.Header("Eval: " + cfg.Server.Name)
	fmt.Println()

	// Open eval store at workspace root
	ws, _ := workspace.Find(cwd)
	var demiDir string
	if ws != nil {
		demiDir = filepath.Join(ws.Root, ".demi/forge")
	} else {
		demiDir = filepath.Join(cwd, ".demi/forge")
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

	// Discover suites
	var suitePaths []string
	if srvEvalSuitePath != "" {
		suitePaths = []string{srvEvalSuitePath}
	} else {
		evalsDir := filepath.Join(cwd, "evals")
		discovered, err := eval.DiscoverSuites(evalsDir)
		if err != nil {
			console.Error(fmt.Sprintf("No eval suites found in %s", evalsDir))
			console.Dim("  Create *.eval.yaml files in evals/ or use --suite")
			return err
		}
		if len(discovered) == 0 {
			console.Warning("No eval suites found")
			console.Dim("  Create *.eval.yaml files in evals/ or use --suite")
			return nil
		}
		suitePaths = discovered
	}

	engine := eval.New(store)
	var anyFailed bool

	for _, path := range suitePaths {
		suite, err := eval.LoadSuite(path)
		if err != nil {
			console.Error(fmt.Sprintf("Load suite %s: %v", path, err))
			anyFailed = true
			continue
		}

		// If suite has no server command, use one from config
		if suite.Server == "" {
			suite.Server = cfg.Server.Command
		}

		console.Info(fmt.Sprintf("Suite: %s (%d scenarios)", suite.Name, len(suite.Scenarios)))

		result, err := engine.RunSuite(cmd.Context(), suite, cwd)
		if err != nil {
			console.Error(fmt.Sprintf("Run suite: %v", err))
			anyFailed = true
			continue
		}

		// Detect regressions
		regressions, _ := eval.DetectRegressions(store, suite.Name, result.Scenarios)

		// Output results
		switch srvEvalFormat {
		case "json":
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal results: %w", err)
			}
			fmt.Println(string(data))
		default:
			printEvalResults(result, regressions)
		}

		// Update baselines if requested
		if srvEvalUpdateBaseline {
			if err := eval.UpdateBaselines(store, suite.Name, result.Scenarios); err != nil {
				console.Error(fmt.Sprintf("Update baselines: %v", err))
			} else {
				console.Success("Baselines updated")
			}
		}

		// Update snapshots if requested
		if srvEvalUpdateSnapshot {
			if err := eval.UpdateSnapshots(suite, result.ResultData, cwd); err != nil {
				console.Error(fmt.Sprintf("Update snapshots: %v", err))
			} else {
				console.Success("Snapshots updated")
			}
		}

		// Generate HTML report if requested
		if srvEvalReport {
			html, err := eval.GenerateReport(result, regressions)
			if err != nil {
				console.Error(fmt.Sprintf("Generate report: %v", err))
			} else {
				reportPath := filepath.Join(demiDir, fmt.Sprintf("eval-report-%s.html", result.RunID))
				if err := os.WriteFile(reportPath, []byte(html), 0o644); err != nil {
					console.Error(fmt.Sprintf("Write report: %v", err))
				} else {
					console.Success("Report saved: " + reportPath)
					openBrowser(reportPath)
				}
			}
		}

		if result.Status != "passed" {
			anyFailed = true
		}
	}

	if anyFailed {
		return fmt.Errorf("one or more eval suites failed")
	}

	return nil
}

func printEvalResults(result *eval.RunResult, regressions []eval.Regression) {
	for _, sr := range result.Scenarios {
		switch sr.Status {
		case "passed":
			console.Success(fmt.Sprintf("%s (%dms)", sr.Name, sr.Duration.Milliseconds()))
		case "failed":
			console.Error(fmt.Sprintf("%s (%dms)", sr.Name, sr.Duration.Milliseconds()))
			for _, a := range sr.Assertions {
				if !a.Passed {
					console.Dim(fmt.Sprintf("    %s(%s): %s", a.Type, a.Field, a.Message))
				}
			}
		default:
			console.Warning(fmt.Sprintf("%s: %s", sr.Name, sr.Error))
		}
	}

	fmt.Println()

	if len(regressions) > 0 {
		console.Warning(fmt.Sprintf("%d regression(s) detected:", len(regressions)))
		for _, r := range regressions {
			console.Dim(fmt.Sprintf("  %s: was %s, now %s", r.ScenarioName, r.PreviousStatus, r.CurrentStatus))
		}
		fmt.Println()
	}

	if result.Failed == 0 && result.Skipped == 0 {
		console.Success(fmt.Sprintf("All %d scenarios passed", result.Passed))
	} else {
		console.Error(fmt.Sprintf("%d passed, %d failed, %d skipped", result.Passed, result.Failed, result.Skipped))
	}
}


func init() {
	serverEvalCmd.Flags().StringVar(&srvEvalSuitePath, "suite", "", "Path to specific eval suite YAML")
	serverEvalCmd.Flags().BoolVar(&srvEvalUpdateBaseline, "update-baselines", false, "Set current results as new baselines")
	serverEvalCmd.Flags().BoolVar(&srvEvalUpdateSnapshot, "update-snapshots", false, "Overwrite golden files with current results")
	serverEvalCmd.Flags().BoolVar(&srvEvalReport, "report", false, "Generate HTML report and open in browser")
	serverEvalCmd.Flags().StringVar(&srvEvalFormat, "format", "text", "Output format (text, json)")

	serverCmd.AddCommand(serverEvalCmd)
}
