package skills

import (
	"os/exec"
)

// PolicyResult holds the outcome of a Vigil policy check.
type PolicyResult struct {
	Status string // "allowed", "blocked", "warn"
	Reason string
}

// VigilClient interfaces with the trove-vigil CLI for skill governance.
type VigilClient struct {
	available bool
}

// DetectVigil checks if trove-vigil is installed and returns a client.
// Returns nil if Vigil is not available.
func DetectVigil() *VigilClient {
	_, err := exec.LookPath("trove-vigil")
	if err != nil {
		return nil
	}
	return &VigilClient{available: true}
}

// IsAvailable returns true if the Vigil CLI was detected.
func (v *VigilClient) IsAvailable() bool {
	return v != nil && v.available
}

// CheckPolicy checks if a skill is allowed by Vigil governance.
// If Vigil is not available, always returns allowed.
func (v *VigilClient) CheckPolicy(skillIdentity string) PolicyResult {
	if v == nil || !v.available {
		return PolicyResult{Status: "allowed", Reason: "vigil not installed"}
	}

	// Execute: trove-vigil skills check <identity>
	cmd := exec.Command("trove-vigil", "skills", "check", skillIdentity)
	output, err := cmd.Output()
	if err != nil {
		// If trove-vigil returns non-zero, the skill is blocked
		if exitErr, ok := err.(*exec.ExitError); ok {
			return PolicyResult{
				Status: "blocked",
				Reason: string(exitErr.Stderr),
			}
		}
		// If trove-vigil fails to run, allow by default (soft dependency)
		return PolicyResult{Status: "allowed", Reason: "vigil check failed, allowing by default"}
	}

	_ = output // trove-vigil outputs details, but for CRAWL we just care about exit code
	return PolicyResult{Status: "allowed", Reason: "vigil policy allows"}
}
