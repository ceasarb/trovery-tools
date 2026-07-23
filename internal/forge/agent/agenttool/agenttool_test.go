package agenttool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/provider"
)

func writeAgentYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewAgentAsToolRequiresExpose(t *testing.T) {
	dir := t.TempDir()
	writeAgentYAML(t, dir, `
name: test-agent
model:
  provider: openai
  model: gpt-4o-mini
system_prompt: test
`)

	_, err := New(dir, nil, 0, DefaultMaxDepth)
	if err == nil {
		t.Fatal("expected error for agent without expose config")
	}
}

func TestNewAgentAsToolSuccess(t *testing.T) {
	dir := t.TempDir()
	writeAgentYAML(t, dir, `
name: researcher
model:
  provider: openai
  model: gpt-4o-mini
system_prompt: Research topics
expose:
  as_tool: true
  tool_name: deep_research
  description: Research a topic deeply
`)

	srv, err := New(dir, nil, 0, DefaultMaxDepth)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	tool := srv.ExposedTool()
	if tool.Name != "deep_research" {
		t.Fatalf("expected tool name deep_research, got %s", tool.Name)
	}
	if tool.Description != "Research a topic deeply" {
		t.Fatalf("expected description, got %s", tool.Description)
	}
}

func TestExposedToolDefaultName(t *testing.T) {
	dir := t.TempDir()
	writeAgentYAML(t, dir, `
name: my-agent
model:
  provider: openai
  model: gpt-4o-mini
system_prompt: test
expose:
  as_tool: true
`)

	srv, err := New(dir, nil, 0, DefaultMaxDepth)
	if err != nil {
		t.Fatal(err)
	}

	tool := srv.ExposedTool()
	if tool.Name != "my-agent" {
		t.Fatalf("expected tool name my-agent (default), got %s", tool.Name)
	}
}

func TestDepthLimitExceeded(t *testing.T) {
	dir := t.TempDir()
	writeAgentYAML(t, dir, `
name: deep-agent
model:
  provider: openai
  model: gpt-4o-mini
system_prompt: test
expose:
  as_tool: true
`)

	srv, err := New(dir, nil, 3, 3) // currentDepth == maxDepth
	if err != nil {
		t.Fatal(err)
	}

	result, _, err := srv.CallTool(context.Background(), "deep-agent", map[string]interface{}{"message": "hi"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for depth exceeded")
	}
	if result.Content[0].Text != "Max agent depth exceeded" {
		t.Fatalf("expected depth exceeded message, got: %s", result.Content[0].Text)
	}
}

func TestDepthLimitNotExceeded(t *testing.T) {
	dir := t.TempDir()
	writeAgentYAML(t, dir, `
name: shallow
model:
  provider: openai
  model: gpt-4o-mini
system_prompt: test
expose:
  as_tool: true
`)

	// Mock provider that returns a simple response
	mockFactory := func(cfg *agentcfg.AgentConfig) (provider.Provider, error) {
		return &mockProvider{response: "mock response"}, nil
	}

	srv, err := New(dir, mockFactory, 0, 3)
	if err != nil {
		t.Fatal(err)
	}

	result, duration, err := srv.CallTool(context.Background(), "shallow", map[string]interface{}{"message": "hi"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result.IsError {
		t.Fatal("expected no error")
	}
	if result.Content[0].Text != "mock response" {
		t.Fatalf("expected mock response, got: %s", result.Content[0].Text)
	}
	if duration <= 0 {
		t.Fatal("expected positive duration")
	}
	if srv.TokensUsed != 30 { // 10 in + 20 out
		t.Fatalf("expected 30 tokens aggregated, got %d", srv.TokensUsed)
	}
}

func TestExtractMessage(t *testing.T) {
	tests := []struct {
		name    string
		args    interface{}
		want    string
		wantErr bool
	}{
		{"map with message", map[string]interface{}{"message": "hello"}, "hello", false},
		{"string", "direct string", "direct string", false},
		{"nil", nil, "", true},
		{"map without message", map[string]interface{}{"other": "value"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractMessage(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// mockProvider for testing
type mockProvider struct {
	response string
}

func (m *mockProvider) CreateMessage(msgs []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string) (*provider.Response, error) {
	return &provider.Response{
		Content:    []provider.Content{{Type: "text", Text: m.response}},
		Usage:      provider.Usage{InputTokens: 10, OutputTokens: 20},
		StopReason: "end_turn",
	}, nil
}

func (m *mockProvider) CreateMessageStream(msgs []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string, handler provider.StreamHandler) (*provider.Response, error) {
	handler(provider.StreamEvent{Type: "text", Text: m.response})
	handler(provider.StreamEvent{Type: "done"})
	return &provider.Response{
		Content:    []provider.Content{{Type: "text", Text: m.response}},
		Usage:      provider.Usage{InputTokens: 10, OutputTokens: 20},
		StopReason: "end_turn",
	}, nil
}
