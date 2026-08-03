package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Policy Tests ---

func TestGetPreset(t *testing.T) {
	tests := []struct {
		name     string
		wantMem  int
		wantCPUs float64
		wantErr  bool
	}{
		{"strict", 256, 0.5, false},
		{"standard", 512, 1.0, false},
		{"permissive", 1024, 2.0, false},
		{"nonexistent", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := GetPreset(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.MemoryMB != tt.wantMem {
				t.Errorf("MemoryMB = %d, want %d", p.MemoryMB, tt.wantMem)
			}
			if p.CPUs != tt.wantCPUs {
				t.Errorf("CPUs = %f, want %f", p.CPUs, tt.wantCPUs)
			}
			if p.Name != tt.name {
				t.Errorf("Name = %q, want %q", p.Name, tt.name)
			}
		})
	}
}

func TestStrictPolicyDetails(t *testing.T) {
	p, err := GetPreset("strict")
	if err != nil {
		t.Fatal(err)
	}
	if p.Network {
		t.Error("strict policy should have Network=false")
	}
	if !p.ReadOnly {
		t.Error("strict policy should have ReadOnly=true")
	}
	if p.PidsLimit != 50 {
		t.Errorf("strict PidsLimit = %d, want 50", p.PidsLimit)
	}
}

func TestLoadPolicyFile(t *testing.T) {
	content := `name: custom-test
network: true
domains:
  - api.example.com
  - cdn.example.com
read_only: true
memory_mb: 768
cpus: 1.5
pids_limit: 75
`
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicyFile(path)
	if err != nil {
		t.Fatalf("LoadPolicyFile: %v", err)
	}

	if p.Name != "custom-test" {
		t.Errorf("Name = %q, want %q", p.Name, "custom-test")
	}
	if !p.Network {
		t.Error("expected Network=true")
	}
	if len(p.Domains) != 2 {
		t.Errorf("Domains count = %d, want 2", len(p.Domains))
	}
	if p.MemoryMB != 768 {
		t.Errorf("MemoryMB = %d, want 768", p.MemoryMB)
	}
	if p.CPUs != 1.5 {
		t.Errorf("CPUs = %f, want 1.5", p.CPUs)
	}
}

func TestLoadPolicyFileDefaultName(t *testing.T) {
	content := `network: false
memory_mb: 128
`
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "custom" {
		t.Errorf("Name = %q, want %q", p.Name, "custom")
	}
}

func TestResolvePolicy(t *testing.T) {
	// Preset name
	p, err := ResolvePolicy("strict")
	if err != nil {
		t.Fatalf("ResolvePolicy(strict): %v", err)
	}
	if p.Name != "strict" {
		t.Errorf("got name %q, want strict", p.Name)
	}

	// File path
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte("name: from-file\nmemory_mb: 999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = ResolvePolicy(path)
	if err != nil {
		t.Fatalf("ResolvePolicy(file): %v", err)
	}
	if p.Name != "from-file" {
		t.Errorf("got name %q, want from-file", p.Name)
	}

	// Invalid
	_, err = ResolvePolicy("nonexistent-garbage-path")
	if err == nil {
		t.Error("expected error for invalid policy")
	}
}

// --- Dockerfile Tests ---

func TestDetectLanguagePython(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	lang, err := DetectLanguage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if lang != LangPython {
		t.Errorf("got %q, want %q", lang, LangPython)
	}
}

func TestDetectLanguagePyproject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	lang, err := DetectLanguage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if lang != LangPython {
		t.Errorf("got %q, want %q", lang, LangPython)
	}
}

func TestDetectLanguageTypeScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	lang, err := DetectLanguage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if lang != LangTypeScript {
		t.Errorf("got %q, want %q", lang, LangTypeScript)
	}
}

func TestDetectLanguageUnknown(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectLanguage(dir)
	if err == nil {
		t.Error("expected error for unknown language")
	}
}

func TestGenerateDockerfilePython(t *testing.T) {
	// GenerateDockerfile with no dir falls back to pyproject template
	df := GenerateDockerfile(LangPython, "uv run python server.py")

	if !strings.Contains(df, "python:3.13-slim") {
		t.Error("Python Dockerfile should use python:3.13-slim")
	}
	if !strings.Contains(df, "USER nobody") {
		t.Error("Dockerfile should set USER nobody (pyproject template)")
	}
	if !strings.Contains(df, "CMD") {
		t.Error("Dockerfile should contain CMD when command provided")
	}
	if !strings.Contains(df, "server.py") {
		t.Error("CMD should contain the server command")
	}
}

func TestGenerateDockerfilePythonWithRequirements(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	df := GenerateDockerfileForDir(LangPython, "uv run python server.py", dir)

	if !strings.Contains(df, "requirements.txt") {
		t.Error("Python Dockerfile should reference requirements.txt")
	}
	if !strings.Contains(df, "USER trove") {
		t.Error("Dockerfile should set USER trove")
	}
}

func TestGenerateDockerfileTypeScript(t *testing.T) {
	df := GenerateDockerfile(LangTypeScript, "node dist/index.js")

	if !strings.Contains(df, "node:22-slim") {
		t.Error("TypeScript Dockerfile should use node:22-slim")
	}
	if !strings.Contains(df, "npm ci") {
		t.Error("TypeScript Dockerfile should use npm ci")
	}
	if !strings.Contains(df, "USER trove") {
		t.Error("Dockerfile should set USER trove")
	}
	if !strings.Contains(df, "index.js") {
		t.Error("CMD should contain the server command")
	}
}

func TestGenerateDockerfileNoCommand(t *testing.T) {
	df := GenerateDockerfile(LangPython, "")
	if strings.Contains(df, "CMD") {
		t.Error("Dockerfile should not contain CMD when no command given")
	}
}

func TestWriteDockerfile(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteDockerfile(dir, LangPython, "python server.py")
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(path) != "Dockerfile.trove" {
		t.Errorf("expected Dockerfile.trove, got %s", filepath.Base(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "python:3.13-slim") {
		t.Error("written Dockerfile should contain python base image")
	}
}

// --- Firewall Tests ---

func TestGenerateFirewallScript(t *testing.T) {
	script := GenerateFirewallScript([]string{"api.example.com", "cdn.example.com"})

	if !strings.Contains(script, "#!/bin/sh") {
		t.Error("script should start with shebang")
	}
	if !strings.Contains(script, "iptables") {
		t.Error("script should use iptables")
	}
	if !strings.Contains(script, "api.example.com") {
		t.Error("script should reference first domain")
	}
	if !strings.Contains(script, "cdn.example.com") {
		t.Error("script should reference second domain")
	}
	if !strings.Contains(script, "OUTPUT -j DROP") {
		t.Error("script should have a DROP rule at the end")
	}
	if !strings.Contains(script, "dport 53") {
		t.Error("script should allow DNS")
	}
	if !strings.Contains(script, "ESTABLISHED,RELATED") {
		t.Error("script should allow established connections")
	}
}

func TestGenerateFirewallScriptEmpty(t *testing.T) {
	script := GenerateFirewallScript([]string{})
	if script != "" {
		t.Error("empty domains should produce empty script")
	}

	script = GenerateFirewallScript(nil)
	if script != "" {
		t.Error("nil domains should produce empty script")
	}
}
