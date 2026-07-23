package skills

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// SkillScanner discovers and parses SKILL.md files in given paths.
type SkillScanner struct {
	scanPaths []string
}

// NewSkillScanner creates a scanner that searches the given paths.
func NewSkillScanner(scanPaths []string) *SkillScanner {
	return &SkillScanner{scanPaths: scanPaths}
}

// Scan walks all scan paths and returns discovered skill metadata.
func (s *SkillScanner) Scan() ([]*SkillMetadata, error) {
	var skills []*SkillMetadata

	for _, root := range s.scanPaths {
		info, err := os.Stat(root)
		if err != nil {
			slog.Warn("skill scan path not found", "path", root)
			continue
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				// Skip hidden dirs and common non-skill dirs.
				base := filepath.Base(path)
				if strings.HasPrefix(base, ".") || base == "node_modules" || base == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}

			if strings.ToUpper(fi.Name()) == "SKILL.MD" {
				meta, err := ParseSkillFile(path)
				if err != nil {
					slog.Warn("could not parse skill file", "path", path, "error", err)
					return nil
				}
				skills = append(skills, meta)
			}
			return nil
		})
		if err != nil {
			slog.Warn("error scanning skills", "path", root, "error", err)
		}
	}

	return skills, nil
}
