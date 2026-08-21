package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	cfg := &AgentConfig{
		Name: "test-agent",
		Model: ModelConfig{
			Provider:    "anthropic",
			APIKeyEnv:   "ANTHROPIC_API_KEY",
			Model:       "claude-haiku-4-5-20251001",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		System: "You are helpful.",
		Servers: []ServerRef{
			{
				Name:    "weather",
				Path:    "/tmp/weather",
				Command: "uv run weather",
			},
		},
		Settings: AgentSettings{
			MaxToolCalls: 25,
			TimeoutSecs:  120,
			Namespacing:  "auto",
		},
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "agent.yaml")); err != nil {
		t.Fatalf("agent.yaml not created: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Name != "test-agent" {
		t.Errorf("Name = %q, want %q", loaded.Name, "test-agent")
	}
	if loaded.Model.Provider != "anthropic" {
		t.Errorf("Provider = %q", loaded.Model.Provider)
	}
	if loaded.Model.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model = %q", loaded.Model.Model)
	}
	if loaded.Model.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("APIKeyEnv = %q", loaded.Model.APIKeyEnv)
	}
	if loaded.System != "You are helpful." {
		t.Errorf("System = %q", loaded.System)
	}
	if len(loaded.Servers) != 1 {
		t.Fatalf("Servers len = %d", len(loaded.Servers))
	}
	if loaded.Servers[0].Name != "weather" {
		t.Errorf("Server name = %q", loaded.Servers[0].Name)
	}
	if loaded.Settings.Namespacing != "auto" {
		t.Errorf("Namespacing = %q", loaded.Settings.Namespacing)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil {
		t.Error("expected error for missing agent.yaml")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("{{invalid"), 0o644)

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestTemplates(t *testing.T) {
	templates := Templates()

	if len(templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(templates))
	}

	names := map[string]bool{}
	for _, tmpl := range templates {
		names[tmpl.Name] = true

		if tmpl.Config.Model.Provider == "" {
			t.Errorf("template %q has empty provider", tmpl.Name)
		}
		if tmpl.Config.Model.Model == "" {
			t.Errorf("template %q has empty model", tmpl.Name)
		}
		if tmpl.Config.Model.APIKeyEnv == "" {
			t.Errorf("template %q has empty api_key_env", tmpl.Name)
		}
		if tmpl.Config.System == "" {
			t.Errorf("template %q has empty system prompt", tmpl.Name)
		}
		// All templates should default to Haiku
		if tmpl.Config.Model.Model != "claude-haiku-4-5-20251001" {
			t.Errorf("template %q defaults to %q, want claude-haiku-4-5-20251001", tmpl.Name, tmpl.Config.Model.Model)
		}
	}

	for _, expected := range []string{"single-agent", "researcher", "custom"} {
		if !names[expected] {
			t.Errorf("missing template: %s", expected)
		}
	}
}

func TestSaveCreatesFile(t *testing.T) {
	dir := t.TempDir()

	cfg := &AgentConfig{
		Name: "minimal",
		Model: ModelConfig{
			Provider: "openai",
			Model:    "gpt-5-mini",
		},
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "agent.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if content == "" {
		t.Error("agent.yaml is empty")
	}
}
