package cli

import (
	"fmt"
	"time"

	"github.com/ceasarb/demigo-tools/internal/vigil/policy"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/ceasarb/demigo-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Finalize the active session",
	RunE:  runStop,
}

var stopForce bool

func init() {
	stopCmd.Flags().BoolVar(&stopForce, "force", false, "Force-stop a stale session (24h+)")
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	cfg, projectRoot, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	mgr := session.NewSessionManager(cfg, projectRoot)

	// Handle force-stop.
	if stopForce {
		s, err := mgr.ForceStop()
		if err != nil {
			return err
		}
		if s != nil {
			console.Warning(fmt.Sprintf("Force-stopped session %s", s.ID))
		} else {
			console.Success("No active session to stop.")
		}
		return nil
	}

	// Get active session for git diff.
	active, _ := mgr.GetActive()
	if active == nil {
		return session.ErrNoActiveSession
	}

	// Compute file changes via git diff.
	var changes []session.FileChange
	if active.GitSnapshot != nil {
		diff, err := session.DiffSince(projectRoot, active.GitSnapshot)
		if err != nil {
			console.Warning(fmt.Sprintf("Could not compute git diff: %v", err))
		} else {
			changes = diff
		}
	}

	// Run policy checks.
	var violations []session.PolicyViolation
	engine, err := policy.NewPolicyEngine(cfg.Policies, projectRoot)
	if err != nil {
		console.Warning(fmt.Sprintf("Could not initialize policy engine: %v", err))
	} else {
		violations = engine.CheckPostSession(changes, projectRoot)
	}

	s, err := mgr.Stop(changes, violations)
	if err != nil {
		return err
	}

	printSessionSummary(s)
	return nil
}

func printSessionSummary(s *session.Session) {
	console.Header(fmt.Sprintf("Session stopped: %s", s.ID))
	console.Dim(fmt.Sprintf("  Duration: %s", formatDuration(s.DurationSeconds)))
	console.Dim(fmt.Sprintf("  Files changed: %d", len(s.FileChanges)))

	// Group changes by source for clearer output.
	committed, working := groupChangesBySource(s.FileChanges)

	if len(committed) > 0 {
		for _, fc := range committed {
			console.Dim(formatChangePrefix(fc) + fc.Path)
		}
	}

	if len(working) > 0 {
		console.Dim("")
		console.Warning(fmt.Sprintf("  Uncommitted changes: %d", len(working)))
		for _, fc := range working {
			tag := fc.Source
			if tag == "" {
				tag = "modified"
			}
			console.Dim(fmt.Sprintf("     %-10s %s", tag, fc.Path))
		}
	}

	console.Dim(fmt.Sprintf("  Tool runs: %d", len(s.ToolRuns)))

	if len(s.Violations) == 0 {
		console.Success("No violations")
	} else {
		errorCount := 0
		warnCount := 0
		for _, v := range s.Violations {
			if v.Severity == "error" {
				errorCount++
			} else {
				warnCount++
			}
		}
		if errorCount > 0 {
			console.Error(fmt.Sprintf("Violations: %dE %dW", errorCount, warnCount))
		} else {
			console.Warning(fmt.Sprintf("Violations: %dW", warnCount))
		}

		for _, v := range s.Violations {
			loc := ""
			if v.File != "" {
				loc = v.File
				if v.Line > 0 {
					loc += fmt.Sprintf(":%d", v.Line)
				}
				loc = " (" + loc + ")"
			}
			if v.Severity == "error" {
				console.Error(fmt.Sprintf("  %s%s", v.Message, loc))
			} else {
				console.Warning(fmt.Sprintf("  %s%s", v.Message, loc))
			}
		}
	}
}

func formatChangePrefix(fc session.FileChange) string {
	switch fc.ChangeType {
	case "added":
		return "     added    "
	case "modified":
		return "     modified "
	case "deleted":
		return "     deleted  "
	default:
		return "     " + fc.ChangeType + " "
	}
}

func groupChangesBySource(changes []session.FileChange) (committed, working []session.FileChange) {
	for _, fc := range changes {
		switch fc.Source {
		case "committed", "":
			committed = append(committed, fc)
		default:
			working = append(working, fc)
		}
	}
	return
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
