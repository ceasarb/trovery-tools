package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSkillMD = `---
name: code-review
namespace: acme
version: 0.1.0
description: Performs structured code review.
tags:
  - code-quality
---

# Code Review

When reviewing code, check error handling.
`

func writeSkill(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseDir(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, validSkillMD)

	s, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Name != "code-review" || s.Namespace != "acme" || s.Version != "0.1.0" {
		t.Fatalf("unexpected metadata: %+v", s)
	}
	if s.Path != dir {
		t.Fatalf("ParseDir Path = %q, want dir %q", s.Path, dir)
	}
	if want := "When reviewing code"; !strings.Contains(s.Body, want) {
		t.Fatalf("body missing %q: %q", want, s.Body)
	}
}

func TestParseResolvesTroveMetadata(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, `---
name: code-review
description: desc
metadata:
  trove.namespace: acme
  trove.version: 2.3.4
---
body
`)
	s, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Namespace != "acme" {
		t.Errorf("Namespace = %q, want acme (from metadata)", s.Namespace)
	}
	if s.Version != "2.3.4" {
		t.Errorf("Version = %q, want 2.3.4 (from metadata)", s.Version)
	}
}

// Skills authored before the Trovery rebrand use the tandem.* prefix; they must
// still resolve via the legacy-key fallback.
func TestParseFallsBackToLegacyTandemPrefix(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, `---
name: code-review
description: desc
metadata:
  tandem.namespace: acme
  tandem.version: 2.3.4
---
body
`)
	s, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Namespace != "acme" || s.Version != "2.3.4" {
		t.Errorf("legacy tandem.* prefix should resolve: got namespace=%q version=%q", s.Namespace, s.Version)
	}
}

func TestParseFallsBackToLegacyTopLevel(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, `---
name: code-review
namespace: legacy
version: 0.9.0
description: desc
---
body
`)
	s, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Namespace != "legacy" {
		t.Errorf("Namespace = %q, want legacy (fallback)", s.Namespace)
	}
	if s.Version != "0.9.0" {
		t.Errorf("Version = %q, want 0.9.0 (fallback)", s.Version)
	}
}

func TestParseMetadataWinsOverLegacy(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, `---
name: code-review
namespace: legacy
version: 0.9.0
description: desc
metadata:
  trove.namespace: acme
  trove.version: 2.3.4
---
body
`)
	s, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Namespace != "acme" || s.Version != "2.3.4" {
		t.Errorf("metadata should win: got namespace=%q version=%q", s.Namespace, s.Version)
	}
}

func TestParseFileSetsFilePath(t *testing.T) {
	dir := t.TempDir()
	path := writeSkill(t, dir, validSkillMD)

	s, err := ParseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Path != path {
		t.Fatalf("ParseFile Path = %q, want file %q", s.Path, path)
	}
}

func TestParseRejectsMissingName(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "---\nversion: \"1.0\"\n---\nbody\n")
	if _, err := ParseDir(dir); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseRejectsNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "just markdown, no frontmatter\n")
	if _, err := ParseDir(dir); err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestIdentityDefaultsNamespaceFullNameDoesNot(t *testing.T) {
	s := &Skill{Name: "my-skill"}
	if got := s.Identity(); got != "local/my-skill" {
		t.Errorf("Identity() = %q, want local/my-skill", got)
	}
	if got := s.FullName(); got != "my-skill" {
		t.Errorf("FullName() = %q, want my-skill", got)
	}
	s.Namespace = "acme"
	if got := s.Identity(); got != "acme/my-skill" {
		t.Errorf("Identity() = %q, want acme/my-skill", got)
	}
	if got := s.FullName(); got != "acme/my-skill" {
		t.Errorf("FullName() = %q, want acme/my-skill", got)
	}
}
