package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ceasarb/trovery-tools/pkg/forge/server/harness"
	"github.com/ceasarb/trovery-tools/internal/forge/server/validate"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/console"
	"github.com/spf13/cobra"
)

var (
	validateFormat   string
	validateCategory string
)

var serverValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate MCP protocol compliance for a server",
	RunE:  runServerValidate,
}

func runServerValidate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Load server config
	cfg, err := config.LoadServerConfig(cwd)
	if err != nil {
		console.Error(fmt.Sprintf("No trove.toml found: %v", err))
		console.Dim("  Run this command from inside a server directory")
		return err
	}

	console.Header("Validating: " + cfg.Server.Name)
	fmt.Println()

	// Parse command
	parts := strings.Fields(cfg.Server.Command)
	if len(parts) == 0 {
		return fmt.Errorf("empty server command in trove.toml")
	}

	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := harness.Start(ctx, parts[0], parts[1:], cwd)
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

	// Run validation
	v := validate.NewValidator()

	var report *validate.Report
	if validateCategory != "" {
		cat := validate.Category(validateCategory)
		report = v.ValidateCategory(tools, cat)
	} else {
		report = v.Validate(tools)
	}

	// Output
	switch validateFormat {
	case "json":
		data, err := validate.FormatJSON(report)
		if err != nil {
			return fmt.Errorf("format JSON: %w", err)
		}
		fmt.Println(string(data))
	default:
		fmt.Print(validate.FormatText(report))
	}

	// Exit code: 2 if errors found
	if report.Summary.Errors > 0 {
		return fmt.Errorf("%d validation error(s) found", report.Summary.Errors)
	}

	if report.Summary.Warnings == 0 && report.Summary.Infos == 0 {
		console.Success("All checks passed")
	}

	return nil
}

func init() {
	serverValidateCmd.Flags().StringVar(&validateFormat, "format", "text", "Output format (text, json)")
	serverValidateCmd.Flags().StringVar(&validateCategory, "category", "", "Filter to specific category")

	serverCmd.AddCommand(serverValidateCmd)
}
