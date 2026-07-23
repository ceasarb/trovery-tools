package skills

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Pack creates a distributable .tar.gz archive of a skill directory.
// The archive name is based on the skill identity and version.
func Pack(dir string) (string, error) {
	skill, err := ParseSkill(dir)
	if err != nil {
		return "", fmt.Errorf("parse skill: %w", err)
	}

	result := Validate(skill)
	if !result.Valid {
		return "", fmt.Errorf("skill validation failed:\n%s", FormatResult(result))
	}

	if skill.Version == "" {
		return "", fmt.Errorf("skill must have a version to pack (add version: to SKILL.md frontmatter)")
	}

	// Build archive name: name-version.skill.tar.gz
	archiveName := fmt.Sprintf("%s-%s.skill.tar.gz", skill.Name, skill.Version)
	archivePath := filepath.Join(filepath.Dir(dir), archiveName)

	f, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Walk the skill directory and add all files
	baseDir := filepath.Base(dir)
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		name := info.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Create tar header
		relPath, _ := filepath.Rel(filepath.Dir(dir), path)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})
	if err != nil {
		os.Remove(archivePath)
		return "", fmt.Errorf("archive: %w", err)
	}

	_ = baseDir
	return archivePath, nil
}
