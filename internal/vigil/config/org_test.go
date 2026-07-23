package config

import (
	"testing"
)

func TestMergeOrgPolicy_NilOrg(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Name = "test"
	violations := MergeOrgPolicy(&cfg, nil)
	if len(violations) != 0 {
		t.Errorf("expected no violations with nil org, got %d", len(violations))
	}
}

func TestMergeOrgPolicy_MergesPatterns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Name = "test"
	originalCount := len(cfg.Policies.Secrets.BlockPatterns)

	org := &OrgPolicy{
		Policies: Policies{
			Secrets: SecretsPolicy{
				BlockPatterns: []string{"ORG_SECRET_[0-9]+"},
			},
		},
	}

	MergeOrgPolicy(&cfg, org)

	// Should have original + org pattern (deduped).
	if len(cfg.Policies.Secrets.BlockPatterns) != originalCount+1 {
		t.Errorf("expected %d patterns after merge, got %d", originalCount+1, len(cfg.Policies.Secrets.BlockPatterns))
	}
}

func TestMergeOrgPolicy_LockedField(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Name = "test"
	cfg.Policies.Secrets.BlockPatterns = []string{"REPO_ONLY_PATTERN"}

	org := &OrgPolicy{
		Policies: Policies{
			Secrets: SecretsPolicy{
				BlockPatterns: []string{"ORG_PATTERN"},
			},
		},
		Locked: []string{"policies.secrets.block_patterns"},
	}

	violations := MergeOrgPolicy(&cfg, org)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}

	// Locked field should be org's value.
	if len(cfg.Policies.Secrets.BlockPatterns) != 1 || cfg.Policies.Secrets.BlockPatterns[0] != "ORG_PATTERN" {
		t.Errorf("expected org patterns to override, got %v", cfg.Policies.Secrets.BlockPatterns)
	}
}

func TestMergeOrgPolicy_LockedFieldNoViolation(t *testing.T) {
	org := &OrgPolicy{
		Policies: Policies{
			Secrets: SecretsPolicy{
				BlockPatterns: []string{"ORG_PATTERN"},
			},
		},
		Locked: []string{"policies.secrets.block_patterns"},
	}

	// Repo uses same patterns as org — no violation.
	cfg := DefaultConfig()
	cfg.Name = "test"
	cfg.Policies.Secrets.BlockPatterns = []string{"ORG_PATTERN"}

	violations := MergeOrgPolicy(&cfg, org)
	if len(violations) != 0 {
		t.Errorf("expected no violations when repo matches org, got %d", len(violations))
	}
}

func TestMergeOrgPolicy_MergesFilesystem(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Name = "test"
	cfg.Policies.Filesystem.NoWrite = []string{"deploy/"}

	org := &OrgPolicy{
		Policies: Policies{
			Filesystem: FilesystemPolicy{
				NoWrite: []string{"infrastructure/"},
			},
		},
	}

	MergeOrgPolicy(&cfg, org)

	if len(cfg.Policies.Filesystem.NoWrite) != 2 {
		t.Errorf("expected 2 no_write paths, got %d: %v", len(cfg.Policies.Filesystem.NoWrite), cfg.Policies.Filesystem.NoWrite)
	}
}

func TestMergeOrgPolicy_LockedFilesystem(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Name = "test"
	cfg.Policies.Filesystem.ReadOnly = []string{"custom/"}

	org := &OrgPolicy{
		Policies: Policies{
			Filesystem: FilesystemPolicy{
				ReadOnly: []string{".env*", "secrets/"},
			},
		},
		Locked: []string{"policies.filesystem.read_only"},
	}

	violations := MergeOrgPolicy(&cfg, org)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if len(cfg.Policies.Filesystem.ReadOnly) != 2 {
		t.Errorf("expected 2 org read_only paths, got %v", cfg.Policies.Filesystem.ReadOnly)
	}
}

func TestMergeOrgPolicy_LockedScanAgentOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Name = "test"
	cfg.Policies.Secrets.ScanAgentOutput = false // Repo tries to disable.

	org := &OrgPolicy{
		Policies: Policies{
			Secrets: SecretsPolicy{
				ScanAgentOutput: true,
			},
		},
		Locked: []string{"policies.secrets.scan_agent_output"},
	}

	violations := MergeOrgPolicy(&cfg, org)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if !cfg.Policies.Secrets.ScanAgentOutput {
		t.Error("expected scan_agent_output to be forced true by org")
	}
}

func TestMergedConfigView(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Name = "test"

	// Without org.
	view := MergedConfigView(&cfg, nil)
	if view["name"] != "test" {
		t.Errorf("expected name 'test', got %q", view["name"])
	}
	if view["policies.secrets.block_patterns"] != "[repo]" {
		t.Errorf("expected [repo], got %q", view["policies.secrets.block_patterns"])
	}

	// With org (locked).
	org := &OrgPolicy{
		Locked: []string{"policies.secrets.block_patterns"},
	}
	view = MergedConfigView(&cfg, org)
	if view["policies.secrets.block_patterns"] != "[org-locked]" {
		t.Errorf("expected [org-locked], got %q", view["policies.secrets.block_patterns"])
	}
}

func TestMergeStringSlices_Dedup(t *testing.T) {
	result := mergeStringSlices([]string{"a", "b"}, []string{"b", "c"})
	if len(result) != 3 {
		t.Errorf("expected 3 deduped items, got %d: %v", len(result), result)
	}
}
