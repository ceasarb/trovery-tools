package cli

import (
	"fmt"
	"os"

	"github.com/ceasarb/demigo-tools/internal/vigil/policy"
	"github.com/ceasarb/demigo-tools/internal/vigil/reporting"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/ceasarb/demigo-tools/internal/vigil/shared/console"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Re-run policy checks on a session or between git refs",
	RunE:  runAudit,
}

var (
	auditSession string
	auditCI      bool
	auditBaseRef string
	auditHeadRef string
	auditFormat  string
)

func init() {
	auditCmd.Flags().StringVarP(&auditSession, "session", "s", "", "Session ID (default: last session)")
	auditCmd.Flags().BoolVar(&auditCI, "ci", false, "Exit code 1 on error-level violations")
	auditCmd.Flags().StringVar(&auditBaseRef, "base-ref", "", "Base git ref for diff-based audit")
	auditCmd.Flags().StringVar(&auditHeadRef, "head-ref", "", "Head git ref for diff-based audit")
	auditCmd.Flags().StringVar(&auditFormat, "format", "terminal", "Output format: terminal, json, sarif")
	rootCmd.AddCommand(auditCmd)
}

func runAudit(cmd *cobra.Command, args []string) error {
	cfg, projectRoot, err := loadConfigFromCwd()
	if err != nil {
		return err
	}

	engine, err := policy.NewPolicyEngine(cfg.Policies, projectRoot)
	if err != nil {
		return fmt.Errorf("initializing policy engine: %w", err)
	}

	var changes []session.FileChange

	if auditBaseRef != "" && auditHeadRef != "" {
		// Ref-based audit (no session needed).
		changes, err = session.DiffBetweenRefs(projectRoot, auditBaseRef, auditHeadRef)
		if err != nil {
			return fmt.Errorf("computing diff: %w", err)
		}
	} else {
		// Session-based audit.
		mgr := session.NewSessionManager(cfg, projectRoot)
		var s *session.Session

		if auditSession != "" {
			s, err = mgr.GetSession(auditSession)
			if err != nil {
				return err
			}
		} else {
			sessions, err := mgr.ListSessions(1)
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found")
			}
			s = sessions[0]
		}

		changes = s.FileChanges

		// If session has a git snapshot, re-diff to catch current state.
		if s.GitSnapshot != nil && len(changes) == 0 {
			changes, _ = session.DiffSince(projectRoot, s.GitSnapshot)
		}
	}

	violations := engine.CheckPostSession(changes, projectRoot)

	// Output results in the requested format.
	switch auditFormat {
	case "json":
		b, err := reporting.FormatViolationsJSON(violations)
		if err != nil {
			return fmt.Errorf("formatting JSON: %w", err)
		}
		fmt.Println(string(b))
	case "sarif":
		b, err := reporting.FormatSARIF(violations, Version)
		if err != nil {
			return fmt.Errorf("formatting SARIF: %w", err)
		}
		fmt.Println(string(b))
	default:
		printAuditResults(violations, len(changes))
	}

	// CI exit code.
	if auditCI {
		for _, v := range violations {
			if v.Severity == "error" {
				os.Exit(1)
			}
		}
	}

	return nil
}

func printAuditResults(violations []session.PolicyViolation, fileCount int) {
	console.Header(fmt.Sprintf("Audit Results (%d files checked)", fileCount))

	if len(violations) == 0 {
		console.Success("All checks passed")
		return
	}

	errorCount := 0
	warnCount := 0
	for _, v := range violations {
		if v.Severity == "error" {
			errorCount++
		} else {
			warnCount++
		}
	}

	console.Dim(fmt.Sprintf("  Found: %d errors, %d warnings", errorCount, warnCount))
	fmt.Println()

	for _, v := range violations {
		loc := ""
		if v.File != "" {
			loc = v.File
			if v.Line > 0 {
				loc += fmt.Sprintf(":%d", v.Line)
			}
		}
		if v.Severity == "error" {
			console.Error(fmt.Sprintf("[%s] %s %s", v.Rule, v.Message, loc))
		} else {
			console.Warning(fmt.Sprintf("[%s] %s %s", v.Rule, v.Message, loc))
		}
	}
}
