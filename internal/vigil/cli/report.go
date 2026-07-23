package cli

import (
	"fmt"
	"path/filepath"

	"github.com/ceasarb/demigo-tools/internal/vigil/reporting"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate usage reports from session data",
	RunE:  runReport,
}

var (
	reportDays   int
	reportByTool bool
	reportFormat string
)

func init() {
	reportCmd.Flags().IntVar(&reportDays, "days", 30, "Number of days to include")
	reportCmd.Flags().BoolVar(&reportByTool, "by-tool", false, "Group results by tool")
	reportCmd.Flags().StringVar(&reportFormat, "format", "terminal", "Output format: terminal, json, csv")
	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	cfg, projectRoot, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(projectRoot, cfg.Tracking.SessionDir, "index.db")
	idx, err := session.OpenSQLiteIndex(dbPath)
	if err != nil {
		return fmt.Errorf("opening session index: %w", err)
	}
	defer idx.Close()

	// Auto-rebuild index from JSON files.
	sessionsDir := filepath.Join(projectRoot, cfg.Tracking.SessionDir)
	idx.RebuildIndex(sessionsDir)

	groupBy := "status"
	if reportByTool {
		groupBy = "tool"
	}

	gen := reporting.NewReportGenerator(idx)
	output, err := gen.Generate(reporting.ReportOptions{
		Days:    reportDays,
		GroupBy: groupBy,
		Format:  reportFormat,
	})
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}
