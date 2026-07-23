package skills

import (
	"fmt"
	"strings"
)

// LoadedSkill holds metadata loaded at agent startup (progressive disclosure).
type LoadedSkill struct {
	Skill    *Skill
	Activated bool
}

// Manager handles skill discovery, loading, and activation for an agent.
type Manager struct {
	skills map[string]*LoadedSkill // keyed by identity (namespace/name)
	vigil  *VigilClient
}

// NewManager creates a skill manager. If vigilClient is nil, governance is disabled.
func NewManager(vigilClient *VigilClient) *Manager {
	return &Manager{
		skills: make(map[string]*LoadedSkill),
		vigil:  vigilClient,
	}
}

// LoadSkills parses and registers skills from the given directories.
func (m *Manager) LoadSkills(dirs []string) []error {
	var errs []error
	for _, dir := range dirs {
		skill, err := ParseSkill(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("skill at %s: %w", dir, err))
			continue
		}
		m.skills[skill.Identity()] = &LoadedSkill{Skill: skill}
	}
	return errs
}

// MetadataPrompt generates the system prompt injection with available skill metadata.
func (m *Manager) MetadataPrompt() string {
	if len(m.skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("You have the following skills available. Use them when relevant to the user's task\n")
	b.WriteString("by calling the activate_skill tool with the skill name.\n\n")
	b.WriteString("Available skills:\n")

	for id, ls := range m.skills {
		// If Vigil is active, check if skill is allowed before showing metadata
		if m.vigil != nil {
			result := m.vigil.CheckPolicy(id)
			if result.Status == "blocked" {
				continue // don't show blocked skills
			}
		}
		b.WriteString(fmt.Sprintf("- %s: %s\n", id, ls.Skill.Description))
	}

	return b.String()
}

// ActivateSkill loads the full SKILL.md body for the named skill.
// Returns the skill body text, or an error if blocked by governance or not found.
func (m *Manager) ActivateSkill(identity string) (string, error) {
	ls, ok := m.skills[identity]
	if !ok {
		return "", fmt.Errorf("skill %q not found. Available: %s", identity, m.availableNames())
	}

	// Check Vigil policy if available
	if m.vigil != nil {
		result := m.vigil.CheckPolicy(identity)
		if result.Status == "blocked" {
			return "", fmt.Errorf(
				"Skill '%s' is not permitted by governance policy.\nPolicy: %s.\nContact your platform team to request access.",
				identity, result.Reason,
			)
		}
	}

	ls.Activated = true
	return ls.Skill.Body, nil
}

// Skills returns all loaded skills.
func (m *Manager) Skills() map[string]*LoadedSkill {
	return m.skills
}

func (m *Manager) availableNames() string {
	var names []string
	for id := range m.skills {
		names = append(names, id)
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}
