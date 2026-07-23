package cli

import (
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/ceasarb/demigo-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display session details",
	RunE:  runLog,
}

var (
	logSession string
	logCost    bool
)

func init() {
	logCmd.Flags().StringVarP(&logSession, "session", "s", "", "Session ID (default: last session)")
	logCmd.Flags().BoolVar(&logCost, "cost", false, "Show only token/cost breakdown")
	rootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) error {
	cfg, projectRoot, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	mgr := session.NewSessionManager(cfg, projectRoot)

	var s *session.Session

	if logSession != "" {
		s, err = mgr.GetSession(logSession)
		if err != nil {
			return err
		}
	} else {
		// Try active session first, then most recent.
		s, _ = mgr.GetActive()
		if s == nil {
			sessions, err := mgr.ListSessions(1)
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found")
			}
			s = sessions[0]
		}
	}

	if logCost {
		printCost(s)
		return nil
	}

	printFullLog(s)
	return nil
}

func printFullLog(s *session.Session) {
	console.Header(fmt.Sprintf("Session: %s", s.ID))
	console.Dim(fmt.Sprintf("  Status: %s", s.Status))
	console.Dim(fmt.Sprintf("  Started: %s", s.StartTime.Format("2006-01-02 15:04:05")))
	if !s.EndTime.IsZero() {
		console.Dim(fmt.Sprintf("  Ended: %s", s.EndTime.Format("2006-01-02 15:04:05")))
		console.Dim(fmt.Sprintf("  Duration: %s", formatDuration(s.DurationSeconds)))
	}

	if s.GitSnapshot != nil {
		fmt.Println()
		console.Header("Git Snapshot")
		console.Dim(fmt.Sprintf("  Branch: %s", s.GitSnapshot.Branch))
		console.Dim(fmt.Sprintf("  HEAD: %s", s.GitSnapshot.HeadSHA))
		if len(s.GitSnapshot.DirtyFiles) > 0 {
			console.Dim(fmt.Sprintf("  Dirty files: %d", len(s.GitSnapshot.DirtyFiles)))
		}
	}

	if len(s.ToolRuns) > 0 {
		fmt.Println()
		console.Header("Tool Runs")
		for _, tr := range s.ToolRuns {
			dur := ""
			if !tr.EndTime.IsZero() {
				dur = formatDuration(tr.EndTime.Sub(tr.StartTime).Seconds())
			}
			console.Dim(fmt.Sprintf("  %s — %s (exit %d) %s", tr.Tool, tr.Command, tr.ExitCode, dur))
		}
	}

	if len(s.FileChanges) > 0 {
		fmt.Println()
		console.Header(fmt.Sprintf("File Changes (%d)", len(s.FileChanges)))
		for _, fc := range s.FileChanges {
			console.Dim(fmt.Sprintf("  %-10s %s (+%d -%d)", fc.ChangeType, fc.Path, fc.Additions, fc.Deletions))
		}
	}

	if len(s.Violations) > 0 {
		fmt.Println()
		console.Header("Violations")
		for _, v := range s.Violations {
			loc := ""
			if v.File != "" {
				loc = v.File
				if v.Line > 0 {
					loc += fmt.Sprintf(":%d", v.Line)
				}
			}
			if v.Severity == "error" {
				console.Error(fmt.Sprintf("  [%s] %s %s", v.Rule, v.Message, loc))
			} else {
				console.Warning(fmt.Sprintf("  [%s] %s %s", v.Rule, v.Message, loc))
			}
		}
	} else {
		fmt.Println()
		console.Success("No violations")
	}

	if len(s.TokenUsage) > 0 {
		fmt.Println()
		printCost(s)
	}
}

func printCost(s *session.Session) {
	if len(s.TokenUsage) == 0 {
		console.Dim("No token usage data for this session.")
		return
	}

	console.Header("Token Usage")
	headers := []string{"Tool", "Model", "Input", "Output", "Total", "Cost"}
	var rows [][]string
	for _, u := range s.TokenUsage {
		rows = append(rows, []string{
			u.Tool,
			u.Model,
			fmt.Sprintf("%d", u.InputTokens),
			fmt.Sprintf("%d", u.OutputTokens),
			fmt.Sprintf("%d", u.TotalTokens),
			fmt.Sprintf("$%.4f", u.EstimatedCostUSD),
		})
	}
	console.Table(headers, rows)

	if s.TotalCost() > 0 {
		fmt.Println()
		console.Dim(fmt.Sprintf("  Total: $%.4f", s.TotalCost()))
	}
}
