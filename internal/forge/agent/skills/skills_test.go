package skills

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSkillMD = `---
name: code-review
namespace: acme
version: 0.1.0
description: >
  Performs structured code review following team standards.
tags:
  - code-quality
---

# Code Review

## Instructions

When reviewing code, check for error handling and naming conventions.
`

const minimalSkillMD = `---
name: test-skill
description: A minimal test skill
---

Do the thing.
`

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}

// --- Parser tests ---

func TestParseSkillValid(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, validSkillMD)

	skill, err := ParseSkill(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if skill.Name != "code-review" {
		t.Fatalf("expected name=code-review, got %s", skill.Name)
	}
	if skill.Namespace != "acme" {
		t.Fatalf("expected namespace=acme, got %s", skill.Namespace)
	}
	if skill.Version != "0.1.0" {
		t.Fatalf("expected version=0.1.0, got %s", skill.Version)
	}
	if !strings.Contains(skill.Description, "code review") {
		t.Fatalf("unexpected description: %s", skill.Description)
	}
	if !strings.Contains(skill.Body, "When reviewing code") {
		t.Fatalf("expected body to contain instructions, got: %s", skill.Body)
	}
}

func TestParseSkillMinimal(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, minimalSkillMD)

	skill, err := ParseSkill(dir)
	if err != nil {
		t.Fatal(err)
	}

	if skill.Name != "test-skill" {
		t.Fatalf("expected name=test-skill, got %s", skill.Name)
	}
	if skill.Namespace != "" {
		t.Fatalf("expected empty namespace, got %s", skill.Namespace)
	}
}

func TestParseSkillMissing(t *testing.T) {
	_, err := ParseSkill(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
}

func TestParseSkillNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("just markdown"), 0o644)

	_, err := ParseSkill(dir)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestIdentityDefaultNamespace(t *testing.T) {
	s := &Skill{Name: "my-skill"}
	if s.Identity() != "local/my-skill" {
		t.Fatalf("expected local/my-skill, got %s", s.Identity())
	}
}

func TestIdentityCustomNamespace(t *testing.T) {
	s := &Skill{Name: "my-skill", Namespace: "acme"}
	if s.Identity() != "acme/my-skill" {
		t.Fatalf("expected acme/my-skill, got %s", s.Identity())
	}
}

// --- Validation tests ---

func TestValidateValid(t *testing.T) {
	skill := &Skill{Name: "code-review", Description: "Reviews code", Body: "Instructions here"}
	r := Validate(skill)
	if !r.Valid {
		t.Fatalf("expected valid, got errors: %v", r.Errors)
	}
}

func TestValidateMissingName(t *testing.T) {
	skill := &Skill{Description: "Reviews code", Body: "x"}
	r := Validate(skill)
	if r.Valid {
		t.Fatal("expected invalid for missing name")
	}
}

func TestValidateMissingDescription(t *testing.T) {
	skill := &Skill{Name: "test", Body: "x"}
	r := Validate(skill)
	if r.Valid {
		t.Fatal("expected invalid for missing description")
	}
}

func TestValidateNameTooLong(t *testing.T) {
	skill := &Skill{Name: strings.Repeat("a", 65), Description: "test", Body: "x"}
	r := Validate(skill)
	if r.Valid {
		t.Fatal("expected invalid for name too long")
	}
}

func TestValidateNameInvalidChars(t *testing.T) {
	skill := &Skill{Name: "Code_Review", Description: "test", Body: "x"}
	r := Validate(skill)
	if r.Valid {
		t.Fatal("expected invalid for uppercase/underscore in name")
	}
}

func TestValidateXMLInName(t *testing.T) {
	skill := &Skill{Name: "<script>", Description: "test", Body: "x"}
	r := Validate(skill)
	if r.Valid {
		t.Fatal("expected invalid for XML in name")
	}
}

func TestValidateEmptyBody(t *testing.T) {
	skill := &Skill{Name: "test", Description: "test", Body: ""}
	r := Validate(skill)
	if !r.Valid {
		t.Fatal("empty body should be valid (warning only)")
	}
	if len(r.Warnings) == 0 {
		t.Fatal("expected warning for empty body")
	}
}

func TestValidateInvalidSemver(t *testing.T) {
	skill := &Skill{Name: "test", Description: "test", Version: "abc", Body: "x"}
	r := Validate(skill)
	if !r.Valid {
		t.Fatal("invalid semver should be warning, not error")
	}
	if len(r.Warnings) == 0 {
		t.Fatal("expected warning for invalid semver")
	}
}

func TestValidateDirectoryValid(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, validSkillMD)

	skill, result := ValidateDirectory(dir)
	if skill == nil {
		t.Fatal("expected non-nil skill")
	}
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
}

func TestValidateDirectoryMissing(t *testing.T) {
	_, result := ValidateDirectory(filepath.Join(t.TempDir(), "nonexistent"))
	if result.Valid {
		t.Fatal("expected invalid for missing directory")
	}
}

// --- Activation tests ---

func TestManagerLoadAndActivate(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, validSkillMD)

	mgr := NewManager(nil)
	errs := mgr.LoadSkills([]string{dir})
	if len(errs) > 0 {
		t.Fatalf("load errors: %v", errs)
	}

	prompt := mgr.MetadataPrompt()
	if !strings.Contains(prompt, "acme/code-review") {
		t.Fatalf("expected skill in metadata prompt, got:\n%s", prompt)
	}

	body, err := mgr.ActivateSkill("acme/code-review")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !strings.Contains(body, "When reviewing code") {
		t.Fatalf("expected skill body, got: %s", body)
	}
}

func TestManagerActivateNotFound(t *testing.T) {
	mgr := NewManager(nil)
	_, err := mgr.ActivateSkill("nonexistent/skill")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestManagerWithMockVigil(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, validSkillMD)

	// Mock vigil that blocks everything
	vigil := &VigilClient{available: true}
	mgr := NewManager(vigil)
	mgr.LoadSkills([]string{dir})

	// Vigil will use the real CheckPolicy which shells out to trove-vigil.
	// Since trove-vigil is not installed, it will allow by default.
	// We test that the flow works without errors.
	body, err := mgr.ActivateSkill("acme/code-review")
	if err != nil {
		// If trove-vigil happens to be installed and blocks, that's fine
		t.Logf("activation result: err=%v", err)
		return
	}
	if !strings.Contains(body, "When reviewing code") {
		t.Fatalf("expected skill body")
	}
}

// --- Vigil detection ---

func TestDetectVigilNotInstalled(t *testing.T) {
	// trove-vigil is almost certainly not installed in test env
	// This just tests that detection doesn't crash
	v := DetectVigil()
	if v != nil {
		t.Log("trove-vigil detected — skipping 'not installed' assertion")
	}
}

func TestVigilCheckPolicyNilClient(t *testing.T) {
	var v *VigilClient
	result := v.CheckPolicy("anything")
	if result.Status != "allowed" {
		t.Fatalf("nil client should allow, got %s", result.Status)
	}
}

// --- Pack tests ---

func TestPackValid(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "code-review")
	writeSkill(t, dir, validSkillMD)
	os.MkdirAll(filepath.Join(dir, "scripts"), 0o755)
	os.WriteFile(filepath.Join(dir, "scripts", "check.sh"), []byte("#!/bin/bash\necho ok"), 0o755)

	archivePath, err := Pack(dir)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	if !strings.HasSuffix(archivePath, "code-review-0.1.0.skill.tar.gz") {
		t.Fatalf("unexpected archive name: %s", archivePath)
	}

	// Verify archive contents
	f, _ := os.Open(archivePath)
	defer f.Close()
	gr, _ := gzip.NewReader(f)
	defer gr.Close()
	tr := tar.NewReader(gr)

	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		files = append(files, hdr.Name)
	}

	hasSkillMD := false
	hasScript := false
	for _, f := range files {
		if strings.HasSuffix(f, "SKILL.md") {
			hasSkillMD = true
		}
		if strings.Contains(f, "check.sh") {
			hasScript = true
		}
	}

	if !hasSkillMD {
		t.Fatal("archive missing SKILL.md")
	}
	if !hasScript {
		t.Fatal("archive missing scripts/check.sh")
	}
}

func TestPackNoVersion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-skill")
	writeSkill(t, dir, minimalSkillMD)

	_, err := Pack(dir)
	if err == nil {
		t.Fatal("expected error for skill without version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Fatalf("expected version error, got: %v", err)
	}
}

func TestPackInvalidSkill(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad-skill")
	writeSkill(t, dir, `---
description: no name
---
body
`)

	_, err := Pack(dir)
	if err == nil {
		t.Fatal("expected error for invalid skill")
	}
}
