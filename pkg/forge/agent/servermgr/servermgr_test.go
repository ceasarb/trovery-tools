package servermgr

import (
	"encoding/json"
	"testing"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

func TestAllToolsNoCollision(t *testing.T) {
	mgr := &Manager{
		servers: []*ManagedServer{
			{
				Ref: agentcfg.ServerRef{Name: "weather"},
				Tools: []protocol.Tool{
					{Name: "get_forecast", Description: "Get weather forecast"},
				},
			},
			{
				Ref: agentcfg.ServerRef{Name: "calendar"},
				Tools: []protocol.Tool{
					{Name: "get_events", Description: "Get calendar events"},
				},
			},
		},
	}

	// auto mode, no collisions — no namespacing
	tools := mgr.AllTools("auto")
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].QualifiedName != "get_forecast" {
		t.Errorf("expected non-namespaced name, got %q", tools[0].QualifiedName)
	}
}

func TestAllToolsWithCollision(t *testing.T) {
	mgr := &Manager{
		servers: []*ManagedServer{
			{
				Ref: agentcfg.ServerRef{Name: "weather"},
				Tools: []protocol.Tool{
					{Name: "search", Description: "Search weather"},
				},
			},
			{
				Ref: agentcfg.ServerRef{Name: "calendar"},
				Tools: []protocol.Tool{
					{Name: "search", Description: "Search calendar"},
				},
			},
		},
	}

	// auto mode, collision — should namespace
	tools := mgr.AllTools("auto")
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.QualifiedName] = true
	}

	if !names["weather.search"] {
		t.Error("expected weather.search")
	}
	if !names["calendar.search"] {
		t.Error("expected calendar.search")
	}
}

func TestAllToolsAlwaysNamespace(t *testing.T) {
	mgr := &Manager{
		servers: []*ManagedServer{
			{
				Ref: agentcfg.ServerRef{Name: "weather"},
				Tools: []protocol.Tool{
					{Name: "get_forecast", Description: "Forecast"},
				},
			},
		},
	}

	// always mode — namespace even without collisions
	tools := mgr.AllTools("always")
	if tools[0].QualifiedName != "weather.get_forecast" {
		t.Errorf("expected namespaced, got %q", tools[0].QualifiedName)
	}
}

func TestAllToolsNeverNamespace(t *testing.T) {
	mgr := &Manager{
		servers: []*ManagedServer{
			{
				Ref: agentcfg.ServerRef{Name: "weather"},
				Tools: []protocol.Tool{
					{Name: "search", Description: "Search"},
				},
			},
			{
				Ref: agentcfg.ServerRef{Name: "calendar"},
				Tools: []protocol.Tool{
					{Name: "search", Description: "Search"},
				},
			},
		},
	}

	// never mode — no namespacing even with collisions
	tools := mgr.AllTools("never")
	for _, tool := range tools {
		if tool.QualifiedName != "search" {
			t.Errorf("expected non-namespaced, got %q", tool.QualifiedName)
		}
	}
}

func TestAllToolsEmpty(t *testing.T) {
	mgr := NewManager()
	tools := mgr.AllTools("auto")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestCallToolNotFound(t *testing.T) {
	mgr := &Manager{
		servers: []*ManagedServer{
			{
				Ref: agentcfg.ServerRef{Name: "weather"},
				Tools: []protocol.Tool{
					{Name: "get_forecast"},
				},
			},
		},
	}

	_, _, err := mgr.CallTool(nil, "nonexistent", nil)
	if err == nil {
		t.Error("expected error for missing tool")
	}
}

func TestNamespacedToolFields(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)

	mgr := &Manager{
		servers: []*ManagedServer{
			{
				Ref: agentcfg.ServerRef{Name: "weather"},
				Tools: []protocol.Tool{
					{Name: "hello", Description: "Say hello", InputSchema: schema},
				},
			},
		},
	}

	tools := mgr.AllTools("always")
	if len(tools) != 1 {
		t.Fatal("expected 1 tool")
	}

	tool := tools[0]
	if tool.ServerName != "weather" {
		t.Errorf("ServerName = %q", tool.ServerName)
	}
	if tool.QualifiedName != "weather.hello" {
		t.Errorf("QualifiedName = %q", tool.QualifiedName)
	}
	if tool.Description != "Say hello" {
		t.Errorf("Description = %q", tool.Description)
	}
}
