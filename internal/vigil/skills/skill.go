package skills

import "github.com/ceasarb/trovery-tools/internal/skill"

// The SKILL.md parser lives in the shared internal/skill package so trove-vigil and
// trove-forge parse identically (ADR-006). SkillMetadata and ParseSkillFile re-export
// it, keeping this package's existing API — the scanner and policy engine still
// operate on *SkillMetadata.
type SkillMetadata = skill.Skill

// ParseSkillFile parses the SKILL.md at path.
func ParseSkillFile(path string) (*SkillMetadata, error) {
	return skill.ParseFile(path)
}
