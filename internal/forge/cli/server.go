package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/server/devserver"
	"github.com/ceasarb/trovery-tools/pkg/forge/server/harness"
	"github.com/ceasarb/trovery-tools/internal/forge/server/scaffold"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/console"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/env"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/templates"
	"github.com/ceasarb/trovery-tools/internal/forge/workspace"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "MCP server lifecycle — create, test, dev, and more",
	Long: console.HeaderStyle.Render("Server Commands") + "\n\n" +
		"Manage the full MCP server development lifecycle:\n" +
		"  create    Scaffold a new MCP server project\n" +
		"  test      Run test fixtures against a server\n" +
		"  dev       Start dev server with hot reload + REPL",
}

// Flags for server create
var (
	serverName      string
	serverLang      string
	serverTransport string
	serverDesc      string
)

var serverCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Scaffold a new MCP server project",
	RunE:  runServerCreate,
}

func runServerCreate(cmd *cobra.Command, args []string) error {
	// Interactive prompts for missing values
	if serverName == "" {
		err := huh.NewInput().
			Title("Server name").
			Placeholder("weather-api").
			Value(&serverName).
			Run()
		if err != nil {
			return err
		}
	}

	if serverLang == "" {
		err := huh.NewSelect[string]().
			Title("Language").
			Options(
				huh.NewOption("Python", "python"),
				huh.NewOption("TypeScript", "typescript"),
			).
			Value(&serverLang).
			Run()
		if err != nil {
			return err
		}
	}

	if serverTransport == "" {
		err := huh.NewSelect[string]().
			Title("Transport").
			Options(
				huh.NewOption("stdio", "stdio"),
				huh.NewOption("HTTP (Streamable)", "http"),
			).
			Value(&serverTransport).
			Run()
		if err != nil {
			return err
		}
	}

	// HTTP transport only supported for Python
	if serverLang == "typescript" && serverTransport == "http" {
		console.Warning("HTTP transport not yet available for TypeScript — using stdio")
		serverTransport = "stdio"
	}

	if serverDesc == "" {
		serverDesc = "An MCP server built with Trovery Forge"
	}

	console.Header("Creating server: " + serverName)

	// Determine output directory
	outputDir, err := workspace.DetectOutputDir("server")
	if err != nil {
		return err
	}

	// Auto-create servers/ if in workspace but dir missing
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	serverDir := filepath.Join(outputDir, serverName)

	// Check if directory already exists
	if _, err := os.Stat(serverDir); err == nil {
		console.Error(fmt.Sprintf("Directory already exists: %s", serverDir))
		return fmt.Errorf("directory exists: %s", serverDir)
	}

	// Generate files
	files, err := scaffold.Run(scaffold.Options{
		Name:        serverName,
		Language:    scaffold.Language(serverLang),
		Transport:   scaffold.Transport(serverTransport),
		Description: serverDesc,
		OutputDir:   serverDir,
	})
	if err != nil {
		console.Error(fmt.Sprintf("Scaffold failed: %v", err))
		return err
	}

	// Write files to disk
	if err := templates.WriteFiles(serverDir, files); err != nil {
		console.Error(fmt.Sprintf("Write failed: %v", err))
		return err
	}

	// Print summary
	fmt.Println()
	console.Success("Server created at " + serverDir)
	fmt.Println()

	for _, f := range files {
		console.Dim("  " + f.Path)
	}

	fmt.Println()
	console.Info("Next steps:")
	console.Dim("  cd " + serverDir)
	if serverLang == "python" {
		console.Dim("  uv sync")
	} else {
		console.Dim("  npm install")
		console.Dim("  npm run build")
	}
	console.Dim("  trove forge server test")
	console.Dim("  trove forge server dev")

	return nil
}

var serverTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Run test fixtures against a server",
	RunE:  runServerTest,
}

func runServerTest(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	env.LoadDotenv()

	// Load server config
	cfg, err := config.LoadServerConfig(cwd)
	if err != nil {
		console.Error(fmt.Sprintf("No trove.toml found: %v", err))
		console.Dim("  Run this command from inside a server directory")
		return err
	}

	console.Header("Testing: " + cfg.Server.Name)
	fmt.Println()

	// Load fixtures
	fixturesDir := filepath.Join(cwd, cfg.Testing.Fixtures)
	suites, err := harness.LoadFixtures(fixturesDir)
	if err != nil {
		console.Error(fmt.Sprintf("Load fixtures: %v", err))
		return err
	}

	// Parse command
	parts := strings.Fields(cfg.Server.Command)
	if len(parts) == 0 {
		return fmt.Errorf("empty server command in trove.toml")
	}

	// Resolve env vars for the server
	extraEnv, err := env.ResolveServerEnv(cwd, cfg.Server.Env)
	if err != nil {
		return fmt.Errorf("server %s: %w", cfg.Server.Name, err)
	}

	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := harness.StartWithEnv(ctx, parts[0], parts[1:], cwd, extraEnv)
	if err != nil {
		console.Error(fmt.Sprintf("Start server: %v", err))
		return err
	}
	defer client.Close()

	// Discover tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		console.Error(fmt.Sprintf("List tools: %v", err))
		return err
	}
	console.Dim(fmt.Sprintf("  Discovered %d tool(s)", len(tools)))
	fmt.Println()

	// Run fixtures
	var passed, failed int
	var results []harness.TestResult

	for _, suite := range suites {
		for _, tc := range suite.Tests {
			result, duration, err := client.CallTool(ctx, tc.Tool, tc.Input)

			tr := harness.TestResult{
				Name:     tc.Name,
				Duration: duration,
			}

			if err != nil {
				tr.Passed = false
				tr.Error = err.Error()
			} else {
				// Build output for assertion
				text := ""
				for _, c := range result.Content {
					if c.Type == "text" {
						text += c.Text
					}
				}

				output := &harness.ToolCallOutput{
					Text:     text,
					IsError:  result.IsError,
					Duration: duration,
				}

				if checkErr := harness.CheckExpectation(tc.Expect, output); checkErr != nil {
					tr.Passed = false
					tr.Error = checkErr.Error()
				} else {
					tr.Passed = true
				}
			}

			results = append(results, tr)
			if tr.Passed {
				passed++
				console.Success(fmt.Sprintf("%s (%dms)", tr.Name, tr.Duration.Milliseconds()))
			} else {
				failed++
				console.Error(fmt.Sprintf("%s: %s", tr.Name, tr.Error))
			}
		}
	}

	// Summary
	fmt.Println()
	if failed == 0 {
		console.Success(fmt.Sprintf("All %d tests passed", passed))
	} else {
		console.Error(fmt.Sprintf("%d passed, %d failed", passed, failed))
		return fmt.Errorf("%d test(s) failed", failed)
	}

	return nil
}

var serverDevCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start dev server with hot reload and REPL",
	RunE:  runServerDev,
}

func runServerDev(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	env.LoadDotenv()

	cfg, err := config.LoadServerConfig(cwd)
	if err != nil {
		console.Error(fmt.Sprintf("No trove.toml found: %v", err))
		console.Dim("  Run this command from inside a server directory")
		return err
	}

	// Resolve env vars for the server
	extraEnv, envErr := env.ResolveServerEnv(cwd, cfg.Server.Env)
	if envErr != nil {
		return fmt.Errorf("server %s: %w", cfg.Server.Name, envErr)
	}

	console.Header("Dev server: " + cfg.Server.Name)
	if len(extraEnv) > 0 {
		console.Dim(fmt.Sprintf("  Loaded %d env var(s)", len(extraEnv)))
	}
	console.Dim("  Watching for file changes...")
	fmt.Println()

	return devserver.RunWithEnv(cfg, cwd, extraEnv)
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List servers in workspace",
	RunE:  runServerList,
}

func runServerList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	ws, err := workspace.Find(cwd)
	if err != nil {
		return err
	}

	if ws == nil {
		console.Warning("Not inside an Trovery Forge workspace")
		return nil
	}

	serversDir := ws.ServersDir()
	entries, err := os.ReadDir(serversDir)
	if err != nil {
		if os.IsNotExist(err) {
			console.Info("No servers/ directory in this workspace")
			return nil
		}
		return err
	}

	if len(entries) == 0 {
		console.Info("No servers found. Run: trove forge server create")
		return nil
	}

	console.Header("Servers in " + ws.Name)
	fmt.Println()
	for _, e := range entries {
		if e.IsDir() {
			console.Dim("  " + e.Name())
		}
	}

	return nil
}

func init() {
	serverCreateCmd.Flags().StringVar(&serverName, "name", "", "Server name")
	serverCreateCmd.Flags().StringVar(&serverLang, "language", "", "Language (python, typescript)")
	serverCreateCmd.Flags().StringVar(&serverTransport, "transport", "", "Transport (stdio, http)")
	serverCreateCmd.Flags().StringVar(&serverDesc, "description", "", "Server description")

	serverCmd.AddCommand(serverCreateCmd)
	serverCmd.AddCommand(serverTestCmd)
	serverCmd.AddCommand(serverDevCmd)
	serverCmd.AddCommand(serverListCmd)
}
