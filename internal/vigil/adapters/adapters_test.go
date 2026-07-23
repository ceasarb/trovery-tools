package adapters

import (
	"testing"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
)

func TestClaudeCodeAdapter_ResolveCommand(t *testing.T) {
	a := &ClaudeCodeAdapter{}
	cfg := config.ToolConfig{
		Config: map[string]string{
			"model":      "claude-sonnet-4-5-20250929",
			"max_tokens": "8096",
		},
	}
	cmd := a.ResolveCommand(cfg)

	if cmd[0] != "claude" {
		t.Errorf("expected 'claude', got %q", cmd[0])
	}
	if len(cmd) != 5 {
		t.Errorf("expected 5 args, got %d: %v", len(cmd), cmd)
	}
}

func TestClaudeCodeAdapter_ResolveCommand_NoConfig(t *testing.T) {
	a := &ClaudeCodeAdapter{}
	cmd := a.ResolveCommand(config.ToolConfig{})
	if len(cmd) != 1 || cmd[0] != "claude" {
		t.Errorf("expected ['claude'], got %v", cmd)
	}
}

func TestClaudeCodeAdapter_Name(t *testing.T) {
	a := &ClaudeCodeAdapter{}
	if a.Name() != "claude-code" {
		t.Errorf("expected 'claude-code', got %q", a.Name())
	}
}

func TestClaudeCodeAdapter_InstallHint(t *testing.T) {
	a := &ClaudeCodeAdapter{}
	if a.InstallHint() == "" {
		t.Error("expected non-empty install hint")
	}
}

func TestGenericAdapter_ResolveCommand(t *testing.T) {
	a := NewGenericAdapter([]string{"pytest", "-x"})
	cmd := a.ResolveCommand(config.ToolConfig{})
	if len(cmd) != 2 || cmd[0] != "pytest" {
		t.Errorf("expected ['pytest', '-x'], got %v", cmd)
	}
}

func TestGenericAdapter_Name(t *testing.T) {
	a := NewGenericAdapter([]string{"pytest"})
	if a.Name() != "generic" {
		t.Errorf("expected 'generic', got %q", a.Name())
	}
}

func TestGenericAdapter_ValidateEmpty(t *testing.T) {
	a := NewGenericAdapter(nil)
	if err := a.Validate(); err == nil {
		t.Error("expected error for empty command")
	}
}

func TestGenericAdapter_Run(t *testing.T) {
	a := NewGenericAdapter([]string{"echo", "hello"})
	run, err := a.Run(config.ToolConfig{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", run.ExitCode)
	}
	if run.Tool != "generic" {
		t.Errorf("expected tool 'generic', got %q", run.Tool)
	}
}

func TestRegistry_ResolveNamed(t *testing.T) {
	a := ResolveAdapter("claude-code", nil)
	if a == nil {
		t.Fatal("expected non-nil adapter for claude-code")
	}
	if a.Name() != "claude-code" {
		t.Errorf("expected name claude-code, got %q", a.Name())
	}
}

func TestRegistry_ResolveGeneric(t *testing.T) {
	a := ResolveAdapter("unknown", []string{"some-command"})
	if a == nil {
		t.Fatal("expected generic adapter fallback")
	}
	if a.Name() != "generic" {
		t.Errorf("expected generic, got %q", a.Name())
	}
}

func TestRegistry_ResolveNil(t *testing.T) {
	a := ResolveAdapter("unknown", nil)
	if a != nil {
		t.Error("expected nil for unknown tool without command")
	}
}

// --- Codex Adapter ---

func TestCodexAdapter_Name(t *testing.T) {
	a := &CodexAdapter{}
	if a.Name() != "codex" {
		t.Errorf("expected 'codex', got %q", a.Name())
	}
}

func TestCodexAdapter_ResolveCommand(t *testing.T) {
	a := &CodexAdapter{}
	cfg := config.ToolConfig{
		Config: map[string]string{"model": "gpt-4o"},
	}
	cmd := a.ResolveCommand(cfg)
	if cmd[0] != "codex" {
		t.Errorf("expected 'codex', got %q", cmd[0])
	}
	if len(cmd) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(cmd), cmd)
	}
}

func TestCodexAdapter_ResolveCommand_NoConfig(t *testing.T) {
	a := &CodexAdapter{}
	cmd := a.ResolveCommand(config.ToolConfig{})
	if len(cmd) != 1 || cmd[0] != "codex" {
		t.Errorf("expected ['codex'], got %v", cmd)
	}
}

func TestCodexAdapter_InstallHint(t *testing.T) {
	a := &CodexAdapter{}
	if a.InstallHint() == "" {
		t.Error("expected non-empty install hint")
	}
}

// --- Cursor Adapter ---

func TestCursorAdapter_Name(t *testing.T) {
	a := &CursorAdapter{}
	if a.Name() != "cursor" {
		t.Errorf("expected 'cursor', got %q", a.Name())
	}
}

func TestCursorAdapter_ResolveCommand(t *testing.T) {
	a := &CursorAdapter{}
	cfg := config.ToolConfig{
		Config: map[string]string{"workspace": "/path/to/project"},
	}
	cmd := a.ResolveCommand(cfg)
	if cmd[0] != "cursor" {
		t.Errorf("expected 'cursor', got %q", cmd[0])
	}
	if cmd[1] != "/path/to/project" {
		t.Errorf("expected workspace path, got %q", cmd[1])
	}
}

func TestCursorAdapter_ResolveCommand_Default(t *testing.T) {
	a := &CursorAdapter{}
	cmd := a.ResolveCommand(config.ToolConfig{})
	if len(cmd) != 2 || cmd[1] != "." {
		t.Errorf("expected ['cursor', '.'], got %v", cmd)
	}
}

func TestCursorAdapter_InstallHint(t *testing.T) {
	a := &CursorAdapter{}
	if a.InstallHint() == "" {
		t.Error("expected non-empty install hint")
	}
}

// --- Forge Adapter ---

func TestForgeAdapter_Name(t *testing.T) {
	a := &ForgeAdapter{}
	if a.Name() != "forge-agent" {
		t.Errorf("expected 'forge-agent', got %q", a.Name())
	}
}

func TestForgeAdapter_ResolveCommand(t *testing.T) {
	a := &ForgeAdapter{}
	cfg := config.ToolConfig{AgentPath: "./agents/my-agent"}
	cmd := a.ResolveCommand(cfg)
	expected := []string{"demi-forge", "agent", "chat", "./agents/my-agent"}
	if len(cmd) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(cmd), cmd)
	}
	for i, v := range expected {
		if cmd[i] != v {
			t.Errorf("arg[%d]: expected %q, got %q", i, v, cmd[i])
		}
	}
}

func TestForgeAdapter_ResolveCommand_NoPath(t *testing.T) {
	a := &ForgeAdapter{}
	cmd := a.ResolveCommand(config.ToolConfig{})
	if len(cmd) != 3 {
		t.Errorf("expected 3 args without agent_path, got %d: %v", len(cmd), cmd)
	}
}

func TestForgeAdapter_InstallHint(t *testing.T) {
	a := &ForgeAdapter{}
	if a.InstallHint() == "" {
		t.Error("expected non-empty install hint")
	}
}

// --- Registry with new adapters ---

func TestRegistry_ResolveCodex(t *testing.T) {
	a := ResolveAdapter("codex", nil)
	if a == nil || a.Name() != "codex" {
		t.Error("expected codex adapter from registry")
	}
}

func TestRegistry_ResolveCursor(t *testing.T) {
	a := ResolveAdapter("cursor", nil)
	if a == nil || a.Name() != "cursor" {
		t.Error("expected cursor adapter from registry")
	}
}

func TestRegistry_ResolveForge(t *testing.T) {
	a := ResolveAdapter("forge-agent", nil)
	if a == nil || a.Name() != "forge-agent" {
		t.Error("expected forge-agent adapter from registry")
	}
}
