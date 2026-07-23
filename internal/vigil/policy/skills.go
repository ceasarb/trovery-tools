package policy

import (
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
	"github.com/ceasarb/demigo-tools/internal/vigil/skills"
)

// SkillPolicyEngine validates skills against configured policies.
type SkillPolicyEngine struct {
	global   config.SkillsGlobalPolicy
	contexts map[string]config.SkillsPolicy
}

// NewSkillPolicyEngine creates a policy engine from skills config.
func NewSkillPolicyEngine(cfg config.SkillsConfig) *SkillPolicyEngine {
	return &SkillPolicyEngine{
		global:   cfg.Global,
		contexts: cfg.Contexts,
	}
}

// CheckSkills validates discovered skills against policies.
func (e *SkillPolicyEngine) CheckSkills(discovered []*skills.SkillMetadata) []session.PolicyViolation {
	return e.checkAgainstPolicy(discovered, e.global.Policy, e.global.Allowed, e.global.Blocked)
}

// CheckSkillsInContext validates skills in a specific context.
func (e *SkillPolicyEngine) CheckSkillsInContext(discovered []*skills.SkillMetadata, context string) []session.PolicyViolation {
	ctx, ok := e.contexts[context]
	if !ok {
		// Fall back to global.
		return e.CheckSkills(discovered)
	}
	return e.checkAgainstPolicy(discovered, ctx.Policy, ctx.Allowed, ctx.Blocked)
}

func (e *SkillPolicyEngine) checkAgainstPolicy(discovered []*skills.SkillMetadata, policyMode string, allowed, blocked []string) []session.PolicyViolation {
	var violations []session.PolicyViolation

	allowSet := toSet(allowed)
	blockSet := toSet(blocked)

	for _, skill := range discovered {
		name := skill.FullName()

		// Explicit block always wins.
		if blockSet[name] {
			violations = append(violations, session.PolicyViolation{
				Rule:     "skills.blocked",
				Severity: "error",
				Message:  fmt.Sprintf("Skill %q is blocked by policy", name),
				File:     skill.Path,
			})
			continue
		}

		switch policyMode {
		case "allowlist":
			if !allowSet[name] {
				violations = append(violations, session.PolicyViolation{
					Rule:     "skills.not_allowed",
					Severity: "error",
					Message:  fmt.Sprintf("Skill %q is not in the allowlist", name),
					File:     skill.Path,
				})
			}
		case "blocklist":
			// Not blocked (checked above) → allowed.
		case "open", "":
			// Everything allowed.
		default:
			violations = append(violations, session.PolicyViolation{
				Rule:     "skills.unknown_policy",
				Severity: "warning",
				Message:  fmt.Sprintf("Unknown skills policy %q", policyMode),
			})
		}
	}

	return violations
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}
