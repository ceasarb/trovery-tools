package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseDir reads and parses the SKILL.md inside dir. Path is set to the
// directory. This is Forge's entrypoint (formerly ParseSkill).
func ParseDir(dir string) (*Skill, error) {
	s, err := ParseFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, err
	}
	s.Path = dir
	return s, nil
}

// ParseFile reads and parses a SKILL.md at the given file path. Path is set to
// the file. This is Vigil's entrypoint (formerly ParseSkillFile).
func ParseFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	s, err := parseFrontmatterAndBody(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s.Path = path

	if s.Name == "" {
		return nil, fmt.Errorf("skill name is required in %s", path)
	}
	return s, nil
}

// parseFrontmatterAndBody splits SKILL.md into YAML frontmatter and markdown body.
func parseFrontmatterAndBody(content string) (*Skill, error) {
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("SKILL.md must start with --- (YAML frontmatter)")
	}

	// Find the closing ---.
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, fmt.Errorf("SKILL.md missing closing --- for frontmatter")
	}

	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+4:]) // skip past \n---

	// raw carries the spec-core fields plus the legacy top-level namespace/
	// version, which are read only as a one-release fallback (ADR-003).
	var raw struct {
		Name        string            `yaml:"name"`
		Description string            `yaml:"description"`
		Author      string            `yaml:"author"`
		Tags        []string          `yaml:"tags"`
		Metadata    map[string]string `yaml:"metadata"`
		Namespace   string            `yaml:"namespace"` // legacy, superseded by metadata
		Version     string            `yaml:"version"`   // legacy, superseded by metadata
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &raw); err != nil {
		return nil, fmt.Errorf("parse frontmatter YAML: %w", err)
	}

	s := &Skill{
		Name:        raw.Name,
		Description: raw.Description,
		Author:      raw.Author,
		Tags:        raw.Tags,
		Metadata:    raw.Metadata,
		Body:        body,
	}
	s.resolveMetadata(raw.Namespace, raw.Version)
	return s, nil
}

// resolveMetadata fills Namespace and Version from the demi.* metadata keys,
// then the legacy tandem.* keys, then the legacy top-level values — in that
// precedence order, each used only when the higher-priority source is absent (ADR-003).
func (s *Skill) resolveMetadata(legacyNamespace, legacyVersion string) {
	s.Namespace = legacyNamespace
	s.Version = legacyVersion
	if v := s.Metadata[MetaNamespace]; v != "" {
		s.Namespace = v
	} else if v := s.Metadata[legacyMetaNamespace]; v != "" {
		s.Namespace = v
	}
	if v := s.Metadata[MetaVersion]; v != "" {
		s.Version = v
	} else if v := s.Metadata[legacyMetaVersion]; v != "" {
		s.Version = v
	}
}
