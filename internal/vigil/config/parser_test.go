package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	content := `
name: test-project
version: "0.1.0"
tools:
  claude-code:
    enabled: true
policies:
  secrets:
    scan_agent_output: true
    scan_commits: true
    block_patterns:
      - "AWS_SECRET_ACCESS_KEY"
  filesystem:
    read_only:
      - ".env*"
tracking:
  log_file_changes: true
  log_commands: true
  track_tokens: true
  track_cost: true
  session_dir: ".demi/vigil/sessions/"
  export_format: "json"
`
	path := filepath.Join(dir, ".demi/vigil.yaml")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "test-project" {
		t.Errorf("expected name 'test-project', got %q", cfg.Name)
	}
	if !cfg.Tools["claude-code"].Enabled {
		t.Error("expected claude-code to be enabled")
	}
}

func TestLoadConfig_MissingName(t *testing.T) {
	dir := t.TempDir()
	content := `version: "0.1.0"`
	path := filepath.Join(dir, ".demi/vigil.yaml")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected validation error for missing name")
	}
	valErr, ok := err.(*ConfigValidationError)
	if !ok {
		t.Fatalf("expected ConfigValidationError, got %T: %v", err, err)
	}
	if len(valErr.Issues) == 0 {
		t.Error("expected at least one issue")
	}
}

func TestLoadConfig_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	content := `
name: test
policies:
  secrets:
    block_patterns:
      - "[invalid"
`
	path := filepath.Join(dir, ".demi/vigil.yaml")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected validation error for invalid regex")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	content := `name: minimal`
	path := filepath.Join(dir, ".demi/vigil.yaml")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check defaults were applied.
	if cfg.Version != "0.1.0" {
		t.Errorf("expected default version '0.1.0', got %q", cfg.Version)
	}
	if !cfg.Policies.Secrets.ScanAgentOutput {
		t.Error("expected scan_agent_output default true")
	}
	if !cfg.Tracking.LogFileChanges {
		t.Error("expected log_file_changes default true")
	}
	if cfg.Tracking.SessionDir != ".demi/vigil/sessions/" {
		t.Errorf("expected default session_dir, got %q", cfg.Tracking.SessionDir)
	}
	if len(cfg.Policies.Secrets.BlockPatterns) != 6 {
		t.Errorf("expected 6 default block patterns, got %d", len(cfg.Policies.Secrets.BlockPatterns))
	}
	if len(cfg.Policies.Filesystem.ReadOnly) != 5 {
		t.Errorf("expected 5 default read_only patterns, got %d", len(cfg.Policies.Filesystem.ReadOnly))
	}
}

func TestLoadConfig_ForgeAgentMissingPath(t *testing.T) {
	dir := t.TempDir()
	content := `
name: test
tools:
  forge-agent:
    enabled: true
`
	path := filepath.Join(dir, ".demi/vigil.yaml")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for forge-agent without agent_path")
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/.demi/vigil.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFindConfig_SearchesUpward(t *testing.T) {
	dir := t.TempDir()
	// Create .demi/vigil.yaml at root.
	os.MkdirAll(filepath.Join(dir, ".demi"), 0o755)
	os.WriteFile(filepath.Join(dir, ".demi/vigil.yaml"), []byte("name: root"), 0644)

	// Create a nested child directory.
	child := filepath.Join(dir, "a", "b", "c")
	os.MkdirAll(child, 0755)

	// Chdir to child.
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(child)

	found, err := FindConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks for comparison (macOS /var → /private/var).
	expected, _ := filepath.EvalSymlinks(filepath.Join(dir, ".demi/vigil.yaml"))
	actual, _ := filepath.EvalSymlinks(found)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

func TestFindConfig_NotFound(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	_, err := FindConfig()
	if err != ErrConfigNotFound {
		t.Errorf("expected ErrConfigNotFound, got %v", err)
	}
}
