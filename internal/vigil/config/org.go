package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const orgPolicyFileName = "org-policy.yaml"

// OrgPolicy represents organization-wide policy defaults.
// Repos inherit these unless explicitly overridden.
// Locked fields cannot be overridden by repo-level config.
type OrgPolicy struct {
	Policies Policies `yaml:"policies"`
	Locked   []string `yaml:"locked,omitempty"` // Field paths that repos can't override.
}

// orgPolicyDir returns the path to ~/.demi/vigil/.
func orgPolicyDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".demi/vigil")
}

// OrgPolicyPath returns the full path to the org policy file.
func OrgPolicyPath() string {
	dir := orgPolicyDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, orgPolicyFileName)
}

// LoadOrgPolicy loads org-wide policy from ~/.demi/vigil/org-policy.yaml.
// Returns nil if the file doesn't exist (org policy is optional).
func LoadOrgPolicy() (*OrgPolicy, error) {
	path := OrgPolicyPath()
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var org OrgPolicy
	if err := yaml.Unmarshal(data, &org); err != nil {
		return nil, err
	}

	return &org, nil
}

// MergeOrgPolicy merges org-level policy into a repo-level config.
// Org values serve as defaults — repo values win unless the field is locked.
// Returns a list of locked-field violations (fields the repo tried to override).
func MergeOrgPolicy(cfg *VigilConfig, org *OrgPolicy) []string {
	if org == nil {
		return nil
	}

	locked := toStringSet(org.Locked)
	var violations []string

	// Secrets policy merge.
	if locked["policies.secrets.block_patterns"] {
		if len(cfg.Policies.Secrets.BlockPatterns) > 0 {
			// Check if repo changed the patterns from org defaults.
			if !stringSlicesEqual(cfg.Policies.Secrets.BlockPatterns, org.Policies.Secrets.BlockPatterns) {
				violations = append(violations, "policies.secrets.block_patterns is locked by org policy")
			}
		}
		cfg.Policies.Secrets.BlockPatterns = org.Policies.Secrets.BlockPatterns
	} else {
		// Merge: org patterns are base, repo patterns are additive.
		cfg.Policies.Secrets.BlockPatterns = mergeStringSlices(
			org.Policies.Secrets.BlockPatterns,
			cfg.Policies.Secrets.BlockPatterns,
		)
	}

	if locked["policies.secrets.scan_agent_output"] {
		if cfg.Policies.Secrets.ScanAgentOutput != org.Policies.Secrets.ScanAgentOutput {
			violations = append(violations, "policies.secrets.scan_agent_output is locked by org policy")
		}
		cfg.Policies.Secrets.ScanAgentOutput = org.Policies.Secrets.ScanAgentOutput
	}

	if locked["policies.secrets.scan_commits"] {
		if cfg.Policies.Secrets.ScanCommits != org.Policies.Secrets.ScanCommits {
			violations = append(violations, "policies.secrets.scan_commits is locked by org policy")
		}
		cfg.Policies.Secrets.ScanCommits = org.Policies.Secrets.ScanCommits
	}

	// Filesystem policy merge.
	if locked["policies.filesystem.read_only"] {
		if !stringSlicesEqual(cfg.Policies.Filesystem.ReadOnly, org.Policies.Filesystem.ReadOnly) && len(cfg.Policies.Filesystem.ReadOnly) > 0 {
			violations = append(violations, "policies.filesystem.read_only is locked by org policy")
		}
		cfg.Policies.Filesystem.ReadOnly = org.Policies.Filesystem.ReadOnly
	} else {
		cfg.Policies.Filesystem.ReadOnly = mergeStringSlices(
			org.Policies.Filesystem.ReadOnly,
			cfg.Policies.Filesystem.ReadOnly,
		)
	}

	if !locked["policies.filesystem.no_write"] {
		cfg.Policies.Filesystem.NoWrite = mergeStringSlices(
			org.Policies.Filesystem.NoWrite,
			cfg.Policies.Filesystem.NoWrite,
		)
	} else {
		if len(cfg.Policies.Filesystem.NoWrite) > 0 && !stringSlicesEqual(cfg.Policies.Filesystem.NoWrite, org.Policies.Filesystem.NoWrite) {
			violations = append(violations, "policies.filesystem.no_write is locked by org policy")
		}
		cfg.Policies.Filesystem.NoWrite = org.Policies.Filesystem.NoWrite
	}

	// Log if org policy was applied.
	if len(org.Locked) > 0 {
		slog.Debug("org policy applied", "locked_fields", org.Locked, "violations", len(violations))
	}

	return violations
}

// MergedConfigView returns a human-readable representation of the merged config,
// annotating which values came from org policy vs repo.
func MergedConfigView(cfg *VigilConfig, org *OrgPolicy) map[string]string {
	view := make(map[string]string)
	locked := make(map[string]bool)
	if org != nil {
		locked = toStringSet(org.Locked)
	}

	view["name"] = cfg.Name

	if locked["policies.secrets.block_patterns"] {
		view["policies.secrets.block_patterns"] = "[org-locked]"
	} else if org != nil && len(org.Policies.Secrets.BlockPatterns) > 0 {
		view["policies.secrets.block_patterns"] = "[org + repo merged]"
	} else {
		view["policies.secrets.block_patterns"] = "[repo]"
	}

	if locked["policies.filesystem.read_only"] {
		view["policies.filesystem.read_only"] = "[org-locked]"
	} else if org != nil && len(org.Policies.Filesystem.ReadOnly) > 0 {
		view["policies.filesystem.read_only"] = "[org + repo merged]"
	} else {
		view["policies.filesystem.read_only"] = "[repo]"
	}

	return view
}

func toStringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}

func mergeStringSlices(base, override []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range base {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range override {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
