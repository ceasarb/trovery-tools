package skills

import (
	"os"
	"path/filepath"
	"testing"
)

const validSkillMD = `---
name: code-review
namespace: acme
version: "1.0.0"
description: Automated code review skill
author: Team Lead
tags:
  - review
  - quality
---

# Code Review Skill

This skill reviews code.
`

func TestParseSkillFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte(validSkillMD), 0644)

	meta, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Name != "code-review" {
		t.Errorf("expected name 'code-review', got %q", meta.Name)
	}
	if meta.Namespace != "acme" {
		t.Errorf("expected namespace 'acme', got %q", meta.Namespace)
	}
	if meta.FullName() != "acme/code-review" {
		t.Errorf("expected 'acme/code-review', got %q", meta.FullName())
	}
	if len(meta.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(meta.Tags))
	}
	if meta.Path != path {
		t.Errorf("expected path %q, got %q", path, meta.Path)
	}
}

func TestParseSkillFile_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("# Just a regular markdown file\n"), 0644)

	_, err := ParseSkillFile(path)
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseSkillFile_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nversion: \"1.0\"\n---\n"), 0644)

	_, err := ParseSkillFile(path)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestParseSkillFile_NoNamespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\n---\n"), 0644)

	meta, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.FullName() != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", meta.FullName())
	}
}

func TestSkillScanner_Scan(t *testing.T) {
	dir := t.TempDir()

	// Create skill directories.
	skillDir := filepath.Join(dir, "skills", "code-review")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(validSkillMD), 0644)

	scanner := NewSkillScanner([]string{filepath.Join(dir, "skills")})
	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(discovered))
	}
	if discovered[0].Name != "code-review" {
		t.Errorf("expected 'code-review', got %q", discovered[0].Name)
	}
}

func TestSkillScanner_EmptyPath(t *testing.T) {
	scanner := NewSkillScanner([]string{"/nonexistent/path"})
	discovered, err := scanner.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discovered) != 0 {
		t.Errorf("expected 0 skills for nonexistent path, got %d", len(discovered))
	}
}

func TestSkillScanner_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	os.MkdirAll(hidden, 0755)
	os.WriteFile(filepath.Join(hidden, "SKILL.md"), []byte(validSkillMD), 0644)

	scanner := NewSkillScanner([]string{dir})
	discovered, _ := scanner.Scan()
	if len(discovered) != 0 {
		t.Errorf("expected 0 skills (hidden dir skipped), got %d", len(discovered))
	}
}
