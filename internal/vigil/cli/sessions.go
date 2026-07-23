package cli

import (
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/ceasarb/demigo-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List recorded sessions",
	RunE:  runSessions,
}

var (
	sessionsLimit   int
	sessionsSummary bool
)

func init() {
	sessionsCmd.Flags().IntVar(&sessionsLimit, "limit", 20, "Number of sessions to show")
	sessionsCmd.Flags().BoolVar(&sessionsSummary, "summary", false, "Show aggregated stats")
	rootCmd.AddCommand(sessionsCmd)
}

func runSessions(cmd *cobra.Command, args []string) error {
	cfg, projectRoot, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	mgr := session.NewSessionManager(cfg, projectRoot)
	sessions, err := mgr.ListSessions(sessionsLimit)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		console.Dim("No sessions found.")
		return nil
	}

	if sessionsSummary {
		printSummary(sessions)
		return nil
	}

	printSessionTable(sessions)
	return nil
}

func printSessionTable(sessions []*session.Session) {
	headers := []string{"ID", "Status", "Started", "Duration", "Tools", "Files", "Violations"}
	var rows [][]string

	for _, s := range sessions {
		dur := ""
		if s.DurationSeconds > 0 {
			dur = formatDuration(s.DurationSeconds)
		}

		violations := "clean"
		errCount := 0
		warnCount := 0
		for _, v := range s.Violations {
			if v.Severity == "error" {
				errCount++
			} else {
				warnCount++
			}
		}
		if errCount > 0 || warnCount > 0 {
			violations = fmt.Sprintf("%dE %dW", errCount, warnCount)
		}

		rows = append(rows, []string{
			truncate(s.ID, 10),
			string(s.Status),
			s.StartTime.Format("2006-01-02 15:04"),
			dur,
			fmt.Sprintf("%d", len(s.ToolRuns)),
			fmt.Sprintf("%d", len(s.FileChanges)),
			violations,
		})
	}

	console.Table(headers, rows)
}

func printSummary(sessions []*session.Session) {
	totalDuration := 0.0
	totalFiles := 0
	totalViolations := 0
	totalCost := 0.0

	for _, s := range sessions {
		totalDuration += s.DurationSeconds
		totalFiles += len(s.FileChanges)
		totalViolations += len(s.Violations)
		totalCost += s.TotalCost()
	}

	console.Header("Session Summary")
	console.Dim(fmt.Sprintf("  Total sessions: %d", len(sessions)))
	console.Dim(fmt.Sprintf("  Total duration: %s", formatDuration(totalDuration)))
	console.Dim(fmt.Sprintf("  Files changed: %d", totalFiles))
	console.Dim(fmt.Sprintf("  Violations: %d", totalViolations))
	if totalCost > 0 {
		console.Dim(fmt.Sprintf("  Estimated cost: $%.4f", totalCost))
	}
}
