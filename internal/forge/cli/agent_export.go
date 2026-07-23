package cli

import (
	"fmt"
	"strings"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/export"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOutput string
)

var agentExportCmd = &cobra.Command{
	Use:   "export [agent-name]",
	Short: "Generate deployment artifacts from an agent config",
	Long: console.HeaderStyle.Render("Agent Export") + "\n\n" +
		"Generate standalone deployment artifacts from an agent's config.\n\n" +
		"Formats:\n" +
		"  python      Standalone Python script + requirements.txt\n" +
		"  fastapi     FastAPI REST API wrapper\n" +
		"  docker      Dockerfile + docker-compose.yml\n" +
		"  mcp-client  Claude Desktop config (claude_desktop_config.json)",
	Args: cobra.ExactArgs(1),
	RunE: runAgentExport,
}

func runAgentExport(cmd *cobra.Command, args []string) error {
	if exportFormat == "" {
		console.Error("--format flag is required")
		validFmts := make([]string, len(export.ValidFormats()))
		for i, f := range export.ValidFormats() {
			validFmts[i] = string(f)
		}
		console.Dim("  Valid formats: " + strings.Join(validFmts, ", "))
		return fmt.Errorf("missing --format flag")
	}

	if !export.IsValidFormat(exportFormat) {
		console.Error(fmt.Sprintf("Unknown format: %s", exportFormat))
		validFmts := make([]string, len(export.ValidFormats()))
		for i, f := range export.ValidFormats() {
			validFmts[i] = string(f)
		}
		console.Dim("  Valid formats: " + strings.Join(validFmts, ", "))
		return fmt.Errorf("unknown format: %s", exportFormat)
	}

	agentDir, err := resolveAgentDir(args[0])
	if err != nil {
		return err
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		return err
	}

	console.Header("Exporting agent: " + cfg.Name)
	console.Dim(fmt.Sprintf("  Format: %s", exportFormat))
	console.Dim(fmt.Sprintf("  Output: %s", exportOutput))
	fmt.Println()

	exporter := export.New(cfg, export.Format(exportFormat), exportOutput)
	result, err := exporter.Export()
	if err != nil {
		console.Error(fmt.Sprintf("Export failed: %v", err))
		return err
	}

	console.Success(fmt.Sprintf("Exported %d file(s):", len(result.Files)))
	fmt.Println()
	for _, f := range result.Files {
		console.Dim(fmt.Sprintf("  %s — %s", f.Path, f.Description))
	}

	return nil
}

func init() {
	agentExportCmd.Flags().StringVar(&exportFormat, "format", "", "Export format: python, fastapi, docker, mcp-client (required)")
	agentExportCmd.Flags().StringVar(&exportOutput, "output", "./export", "Output directory")
	agentCmd.AddCommand(agentExportCmd)
}
