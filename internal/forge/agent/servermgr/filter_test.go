package servermgr

import (
	"testing"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/protocol"
)

func makeTools(names ...string) []protocol.Tool {
	tools := make([]protocol.Tool, len(names))
	for i, name := range names {
		tools[i] = protocol.Tool{Name: name, Description: "desc for " + name}
	}
	return tools
}

func toolNames(tools []protocol.Tool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

func TestFilterToolsNilFilter(t *testing.T) {
	tools := makeTools("search", "delete", "create")
	result := FilterTools(tools, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 tools, got %d", len(result))
	}
}

func TestFilterToolsIncludeExact(t *testing.T) {
	tools := makeTools("search", "delete", "create")
	filter := &agentcfg.ToolFilter{
		Include: []string{"search", "create"},
	}

	result := FilterTools(tools, filter)
	names := toolNames(result)
	if len(names) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(names), names)
	}
	if names[0] != "search" || names[1] != "create" {
		t.Errorf("unexpected tools: %v", names)
	}
}

func TestFilterToolsIncludeGlob(t *testing.T) {
	tools := makeTools("search_users", "search_posts", "delete_user", "get_profile")
	filter := &agentcfg.ToolFilter{
		Include: []string{"search_*"},
	}

	result := FilterTools(tools, filter)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	names := toolNames(result)
	if names[0] != "search_users" || names[1] != "search_posts" {
		t.Errorf("unexpected tools: %v", names)
	}
}

func TestFilterToolsExcludeExact(t *testing.T) {
	tools := makeTools("search", "delete", "create")
	filter := &agentcfg.ToolFilter{
		Exclude: []string{"delete"},
	}

	result := FilterTools(tools, filter)
	names := toolNames(result)
	if len(names) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(names))
	}
	if names[0] != "search" || names[1] != "create" {
		t.Errorf("unexpected tools: %v", names)
	}
}

func TestFilterToolsExcludeGlob(t *testing.T) {
	tools := makeTools("search_users", "delete_user", "delete_post", "get_profile")
	filter := &agentcfg.ToolFilter{
		Exclude: []string{"delete_*"},
	}

	result := FilterTools(tools, filter)
	names := toolNames(result)
	if len(names) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(names))
	}
	if names[0] != "search_users" || names[1] != "get_profile" {
		t.Errorf("unexpected tools: %v", names)
	}
}

func TestFilterToolsIncludeThenExclude(t *testing.T) {
	tools := makeTools("search_users", "search_posts", "search_admin", "delete_user")
	filter := &agentcfg.ToolFilter{
		Include: []string{"search_*"},
		Exclude: []string{"search_admin"},
	}

	result := FilterTools(tools, filter)
	names := toolNames(result)
	if len(names) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(names), names)
	}
	if names[0] != "search_users" || names[1] != "search_posts" {
		t.Errorf("unexpected tools: %v", names)
	}
}

func TestFilterToolsEmptyInclude(t *testing.T) {
	tools := makeTools("search", "delete")
	filter := &agentcfg.ToolFilter{
		Include: []string{},
	}

	result := FilterTools(tools, filter)
	if len(result) != 2 {
		t.Errorf("empty include should pass all, got %d", len(result))
	}
}

func TestFilterToolsEmptyExclude(t *testing.T) {
	tools := makeTools("search", "delete")
	filter := &agentcfg.ToolFilter{
		Exclude: []string{},
	}

	result := FilterTools(tools, filter)
	if len(result) != 2 {
		t.Errorf("empty exclude should pass all, got %d", len(result))
	}
}

func TestFilterToolsNoMatch(t *testing.T) {
	tools := makeTools("search", "delete")
	filter := &agentcfg.ToolFilter{
		Include: []string{"nonexistent"},
	}

	result := FilterTools(tools, filter)
	if len(result) != 0 {
		t.Errorf("expected 0 tools, got %d", len(result))
	}
}

func TestFilterToolsMultipleGlobPatterns(t *testing.T) {
	tools := makeTools("search_users", "get_profile", "list_items", "delete_user")
	filter := &agentcfg.ToolFilter{
		Include: []string{"search_*", "get_*", "list_*"},
	}

	result := FilterTools(tools, filter)
	if len(result) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result))
	}
}

func TestFilterToolsQuestionMarkGlob(t *testing.T) {
	tools := makeTools("get_a", "get_b", "get_ab")
	filter := &agentcfg.ToolFilter{
		Include: []string{"get_?"},
	}

	result := FilterTools(tools, filter)
	names := toolNames(result)
	if len(names) != 2 {
		t.Fatalf("expected 2 tools (get_a, get_b), got %d: %v", len(names), names)
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		expect   bool
	}{
		{"search", []string{"search"}, true},
		{"search", []string{"delete"}, false},
		{"search_users", []string{"search_*"}, true},
		{"delete_user", []string{"search_*"}, false},
		{"get_a", []string{"get_?"}, true},
		{"get_ab", []string{"get_?"}, false},
		{"search", []string{"s*", "d*"}, true},
		{"alpha", []string{"s*", "d*"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAny(tt.name, tt.patterns)
			if got != tt.expect {
				t.Errorf("matchesAny(%q, %v) = %v, want %v", tt.name, tt.patterns, got, tt.expect)
			}
		})
	}
}

func TestAllToolsWithFiltering(t *testing.T) {
	mgr := &Manager{
		servers: []*ManagedServer{
			{
				Ref: agentcfg.ServerRef{
					Name: "weather",
					Tools: &agentcfg.ToolFilter{
						Include: []string{"get_*"},
					},
				},
				Tools: makeTools("get_forecast", "get_alerts", "delete_cache"),
			},
		},
	}

	// AllTools should return the already-filtered tools
	// (filtering happens at StartServer time, which already applied the filter)
	tools := mgr.AllTools("auto")
	// In this test, we manually set Tools, so the filter was NOT applied by StartServer.
	// The tools are what they are — 3 tools.
	if len(tools) != 3 {
		t.Errorf("expected 3 tools (filter applied at start time, not AllTools), got %d", len(tools))
	}
}
