package policy

import (
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
)

// PolicyEngine orchestrates all policy checks.
type PolicyEngine struct {
	Secrets    *SecretsScanner
	Filesystem *FilesystemChecker
}

// NewPolicyEngine creates an engine from the given policies.
func NewPolicyEngine(cfg config.Policies, projectRoot string) (*PolicyEngine, error) {
	secrets, err := NewSecretsScanner(cfg.Secrets, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("initializing secrets scanner: %w", err)
	}

	return &PolicyEngine{
		Secrets:    secrets,
		Filesystem: NewFilesystemChecker(cfg.Filesystem),
	}, nil
}

// CheckPostSession runs all post-session policy checks.
func (e *PolicyEngine) CheckPostSession(changes []session.FileChange, projectRoot string) []session.PolicyViolation {
	var violations []session.PolicyViolation

	violations = append(violations, e.Secrets.ScanFiles(changes, projectRoot)...)
	violations = append(violations, e.Filesystem.CheckWrites(changes)...)

	return violations
}

// CheckPreRun validates a tool is enabled before running.
func (e *PolicyEngine) CheckPreRun(tool string, cfg *config.VigilConfig) []session.PolicyViolation {
	var violations []session.PolicyViolation

	tc, ok := cfg.Tools[tool]
	if !ok {
		// Unknown tool — allow generic commands.
		return nil
	}

	if !tc.Enabled {
		reason := "no reason given"
		if tc.Reason != "" {
			reason = tc.Reason
		}
		violations = append(violations, session.PolicyViolation{
			Rule:     "tools.enabled",
			Severity: "error",
			Message:  fmt.Sprintf("Tool %q is disabled: %s", tool, reason),
		})
	}

	return violations
}
