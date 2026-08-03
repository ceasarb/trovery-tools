package skills

import "github.com/ceasarb/trovery-tools/internal/skill"

// The SKILL.md parser lives in the shared internal/skill package so trove-forge and
// trove-vigil parse identically (ADR-006). Skill and ParseSkill re-export it, keeping
// this package's existing API — validation, packing, and activation below still
// operate on *Skill.
type Skill = skill.Skill

// ParseSkill parses the SKILL.md in dir.
func ParseSkill(dir string) (*Skill, error) {
	return skill.ParseDir(dir)
}
