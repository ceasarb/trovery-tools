package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/orchestrator"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/servermgr"
	serverconfig "github.com/ceasarb/trovery-tools/pkg/forge/shared/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/console"
	"github.com/ceasarb/trovery-tools/internal/forge/workspace"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent lifecycle — create, chat, eval, and more",
	Long: console.HeaderStyle.Render("Agent Commands") + "\n\n" +
		"Manage the full AI agent lifecycle:\n" +
		"  create      Scaffold a new agent with agent.yaml\n" +
		"  add-server  Wire an MCP server into an agent\n" +
		"  chat        Start an interactive agent session\n" +
		"  list        List agents in workspace\n" +
		"  inspect     Show agent config, tools, and DAG",
}

// Flags
var (
	agentName        string
	agentDescription string
	agentTemplate    string
	agentProvider    string
	agentModel       string
)

var agentCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Scaffold a new agent with agent.yaml",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAgentCreate,
}

func runAgentCreate(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		agentName = args[0]
	}

	if agentName == "" {
		err := huh.NewInput().
			Title("Agent name").
			Placeholder("assistant").
			Value(&agentName).
			Run()
		if err != nil {
			return fmt.Errorf("missing required flag: --name <agent-name>")
		}
	}

	if agentTemplate == "" {
		templates := agentcfg.Templates()
		options := make([]huh.Option[string], len(templates))
		for i, t := range templates {
			options[i] = huh.NewOption(fmt.Sprintf("%s — %s", t.Name, t.Description), t.Name)
		}

		err := huh.NewSelect[string]().
			Title("Template").
			Options(options...).
			Value(&agentTemplate).
			Run()
		if err != nil {
			names := make([]string, len(templates))
			for i, t := range templates {
				names[i] = t.Name
			}
			return fmt.Errorf("missing required flag: --template <%s>", strings.Join(names, "|"))
		}
	}

	// Find template
	var tmpl agentcfg.Template
	for _, t := range agentcfg.Templates() {
		if t.Name == agentTemplate {
			tmpl = t
			break
		}
	}
	if tmpl.Name == "" {
		return fmt.Errorf("unknown template: %s", agentTemplate)
	}

	console.Header("Creating agent: " + agentName)

	// Determine output directory
	outputDir, err := workspace.DetectOutputDir("agent")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	agentDir := filepath.Join(outputDir, agentName)

	if _, err := os.Stat(agentDir); err == nil {
		console.Error(fmt.Sprintf("Directory already exists: %s", agentDir))
		return fmt.Errorf("directory exists: %s", agentDir)
	}

	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return err
	}

	// Build config from template
	cfg := tmpl.Config
	cfg.Name = agentName

	if agentDescription != "" {
		cfg.Description = agentDescription
	}
	if agentProvider != "" {
		cfg.Model.Provider = agentProvider
	}
	if agentModel != "" {
		cfg.Model.Model = agentModel
	}

	// Write agent.yaml
	if err := agentcfg.Save(agentDir, &cfg); err != nil {
		console.Error(fmt.Sprintf("Write agent.yaml: %v", err))
		return err
	}

	fmt.Println()
	console.Success("Agent created at " + agentDir)
	fmt.Println()
	console.Dim("  agent.yaml")
	fmt.Println()
	console.Info("Next steps:")
	console.Dim("  trove forge agent add-server " + agentName + " --server ./servers/<server-name>")
	console.Dim("  trove forge agent chat " + agentName)

	return nil
}

// --- add-server ---

var (
	addServerPath string
)

var agentAddServerCmd = &cobra.Command{
	Use:   "add-server [agent-name]",
	Short: "Wire an MCP server into an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentAddServer,
}

func runAgentAddServer(cmd *cobra.Command, args []string) error {
	agentName := args[0]

	if addServerPath == "" {
		console.Error("--server flag is required")
		return fmt.Errorf("missing --server flag")
	}

	// Resolve agent directory
	agentDir, err := resolveAgentDir(agentName)
	if err != nil {
		return err
	}

	// Load agent config
	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	// Resolve server path
	serverPath, err := filepath.Abs(addServerPath)
	if err != nil {
		return err
	}

	// Load server config to get name and command
	serverCfg, err := serverconfig.LoadServerConfig(serverPath)
	if err != nil {
		console.Error(fmt.Sprintf("Not a valid server directory: %v", err))
		return err
	}

	console.Header("Adding server: " + serverCfg.Server.Name)

	// Discover tools
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tools, err := servermgr.DiscoverTools(ctx, serverCfg.Server.Command, serverPath)
	if err != nil {
		console.Warning(fmt.Sprintf("Could not discover tools: %v", err))
	} else {
		console.Dim(fmt.Sprintf("  Discovered %d tool(s):", len(tools)))
		for _, t := range tools {
			console.Dim(fmt.Sprintf("    %s — %s", t.Name, t.Description))
		}
	}

	// Check for duplicate server
	for _, s := range cfg.Servers {
		if s.Name == serverCfg.Server.Name {
			console.Warning("Server already wired: " + serverCfg.Server.Name)
			return nil
		}
	}

	// Add server reference
	cfg.Servers = append(cfg.Servers, agentcfg.ServerRef{
		Name:    serverCfg.Server.Name,
		Path:    serverPath,
		Command: serverCfg.Server.Command,
	})

	// Save updated config
	if err := agentcfg.Save(agentDir, cfg); err != nil {
		return err
	}

	fmt.Println()
	console.Success("Server wired into " + agentName)

	return nil
}

// --- list ---

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents in workspace",
	RunE:  runAgentList,
}

func runAgentList(cmd *cobra.Command, args []string) error {
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

	agentsDir := ws.AgentsDir()
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			console.Info("No agents/ directory")
			return nil
		}
		return err
	}

	if len(entries) == 0 {
		console.Info("No agents found. Run: trove forge agent create")
		return nil
	}

	console.Header("Agents in " + ws.Name)
	fmt.Println()
	for _, e := range entries {
		if e.IsDir() {
			// Try to load config for details
			cfg, err := agentcfg.Load(filepath.Join(agentsDir, e.Name()))
			if err == nil {
				console.Dim(fmt.Sprintf("  %s (%s/%s, %d servers)", e.Name(), cfg.Model.Provider, cfg.Model.Model, len(cfg.Servers)))
			} else {
				console.Dim("  " + e.Name())
			}
		}
	}

	return nil
}

// --- inspect ---

var agentInspectCmd = &cobra.Command{
	Use:   "inspect [agent-name]",
	Short: "Show agent config, tools, and DAG",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentInspect,
}

func runAgentInspect(cmd *cobra.Command, args []string) error {
	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	console.Header("Agent: " + cfg.Name)
	fmt.Println()

	console.Info("Model")
	console.Dim(fmt.Sprintf("  Provider:    %s", cfg.Model.Provider))
	console.Dim(fmt.Sprintf("  Model:       %s", cfg.Model.Model))
	console.Dim(fmt.Sprintf("  Max tokens:  %d", cfg.Model.MaxTokens))
	fmt.Println()

	console.Info("System Prompt")
	console.Dim("  " + cfg.System)
	fmt.Println()

	if len(cfg.Servers) > 0 {
		console.Info("Servers")
		for _, s := range cfg.Servers {
			console.Dim(fmt.Sprintf("  %s (%s)", s.Name, s.Path))
		}
	} else if !cfg.IsOrchestrator() {
		console.Dim("  No servers wired. Run: trove forge agent add-server " + cfg.Name + " --server <path>")
	}

	// Show orchestrator DAG
	if cfg.IsOrchestrator() {
		fmt.Println()
		console.Info("Orchestrator DAG")
		console.Dim(fmt.Sprintf("  Handoff: %s", cfg.Orchestrator.Handoff))
		console.Dim(fmt.Sprintf("  Agents:  %d", len(cfg.Orchestrator.Agents)))
		fmt.Println()

		nodes := make([]orchestrator.Node, len(cfg.Orchestrator.Agents))
		for i, a := range cfg.Orchestrator.Agents {
			nodes[i] = orchestrator.Node{Name: a.Name, Path: a.Path, DependsOn: a.DependsOn}
		}
		dag, err := orchestrator.BuildDAG(nodes)
		if err != nil {
			console.Error(fmt.Sprintf("  DAG error: %v", err))
		} else {
			fmt.Print(dag.RenderASCII())
		}
	}

	return nil
}

// --- helpers ---

func resolveAgentDir(name string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check if we're already in the agent dir
	if _, err := os.Stat(filepath.Join(cwd, "agent.yaml")); err == nil {
		return cwd, nil
	}

	// Check workspace agents/
	ws, err := workspace.Find(cwd)
	if err != nil {
		return "", err
	}

	if ws != nil {
		agentDir := filepath.Join(ws.AgentsDir(), name)
		if _, err := os.Stat(filepath.Join(agentDir, "agent.yaml")); err == nil {
			return agentDir, nil
		}
	}

	// Check relative path
	dir := filepath.Join(cwd, name)
	if _, err := os.Stat(filepath.Join(dir, "agent.yaml")); err == nil {
		return dir, nil
	}

	// List available agents to help the user
	hint := ""
	if ws != nil {
		entries, err := os.ReadDir(ws.AgentsDir())
		if err == nil && len(entries) > 0 {
			var names []string
			for _, e := range entries {
				if e.IsDir() {
					names = append(names, e.Name())
				}
			}
			if len(names) > 0 {
				hint = fmt.Sprintf("\n  Available agents: %s", strings.Join(names, ", "))
			}
		}
	}

	console.Error(fmt.Sprintf("Agent not found: %s%s", name, hint))
	return "", fmt.Errorf("agent not found: %s", name)
}

func init() {
	agentCreateCmd.Flags().StringVar(&agentName, "name", "", "Agent name")
	agentCreateCmd.Flags().StringVar(&agentDescription, "description", "", "Agent description")
	agentCreateCmd.Flags().StringVar(&agentTemplate, "template", "", "Agent template (single-agent, researcher, custom)")
	agentCreateCmd.Flags().StringVar(&agentProvider, "provider", "", "Model provider (anthropic, openai, ollama)")
	agentCreateCmd.Flags().StringVar(&agentModel, "model", "", "Model name")

	agentAddServerCmd.Flags().StringVar(&addServerPath, "server", "", "Path to MCP server directory")

	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentAddServerCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentInspectCmd)
}
