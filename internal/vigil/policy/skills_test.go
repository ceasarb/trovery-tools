package policy

import (
	"testing"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/skills"
)

func TestSkillPolicyEngine_OpenPolicy(t *testing.T) {
	engine := NewSkillPolicyEngine(config.SkillsConfig{
		Global: config.SkillsGlobalPolicy{Policy: "open"},
	})

	discovered := []*skills.SkillMetadata{
		{Name: "anything", Namespace: "any"},
	}

	violations := engine.CheckSkills(discovered)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations on open policy, got %d", len(violations))
	}
}

func TestSkillPolicyEngine_Allowlist(t *testing.T) {
	engine := NewSkillPolicyEngine(config.SkillsConfig{
		Global: config.SkillsGlobalPolicy{
			Policy:  "allowlist",
			Allowed: []string{"acme/code-review"},
		},
	})

	discovered := []*skills.SkillMetadata{
		{Name: "code-review", Namespace: "acme"},
		{Name: "unknown-skill", Namespace: "other"},
	}

	violations := engine.CheckSkills(discovered)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Rule != "skills.not_allowed" {
		t.Errorf("expected 'skills.not_allowed', got %q", violations[0].Rule)
	}
}

func TestSkillPolicyEngine_Blocklist(t *testing.T) {
	engine := NewSkillPolicyEngine(config.SkillsConfig{
		Global: config.SkillsGlobalPolicy{
			Policy:  "blocklist",
			Blocked: []string{"evil/malware"},
		},
	})

	discovered := []*skills.SkillMetadata{
		{Name: "good-skill", Namespace: "acme"},
		{Name: "malware", Namespace: "evil"},
	}

	violations := engine.CheckSkills(discovered)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Rule != "skills.blocked" {
		t.Errorf("expected 'skills.blocked', got %q", violations[0].Rule)
	}
}

func TestSkillPolicyEngine_BlockOverridesAllow(t *testing.T) {
	engine := NewSkillPolicyEngine(config.SkillsConfig{
		Global: config.SkillsGlobalPolicy{
			Policy:  "allowlist",
			Allowed: []string{"acme/dangerous"},
			Blocked: []string{"acme/dangerous"},
		},
	})

	discovered := []*skills.SkillMetadata{
		{Name: "dangerous", Namespace: "acme"},
	}

	violations := engine.CheckSkills(discovered)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation (block overrides allow), got %d", len(violations))
	}
	if violations[0].Rule != "skills.blocked" {
		t.Errorf("expected 'skills.blocked', got %q", violations[0].Rule)
	}
}

func TestSkillPolicyEngine_ContextOverride(t *testing.T) {
	engine := NewSkillPolicyEngine(config.SkillsConfig{
		Global: config.SkillsGlobalPolicy{Policy: "open"},
		Contexts: map[string]config.SkillsPolicy{
			"ci": {
				Policy:  "allowlist",
				Allowed: []string{"acme/lint"},
			},
		},
	})

	discovered := []*skills.SkillMetadata{
		{Name: "lint", Namespace: "acme"},
		{Name: "deploy", Namespace: "acme"},
	}

	// Global — everything allowed.
	v1 := engine.CheckSkills(discovered)
	if len(v1) != 0 {
		t.Errorf("expected 0 global violations, got %d", len(v1))
	}

	// CI context — only lint allowed.
	v2 := engine.CheckSkillsInContext(discovered, "ci")
	if len(v2) != 1 {
		t.Fatalf("expected 1 CI violation, got %d", len(v2))
	}
}

func TestSkillPolicyEngine_EmptyPolicy(t *testing.T) {
	engine := NewSkillPolicyEngine(config.SkillsConfig{})

	discovered := []*skills.SkillMetadata{
		{Name: "anything"},
	}

	violations := engine.CheckSkills(discovered)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations with empty policy, got %d", len(violations))
	}
}
