package servermgr

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/server/harness"
	serverconfig "github.com/ceasarb/trovery-tools/pkg/forge/shared/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/env"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

// ToolCaller can execute a tool call. Both harness.Client and agenttool.Server implement this.
type ToolCaller interface {
	CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error)
	Close()
}

// ChildStats provides aggregated stats from a child agent execution.
// Implemented by agenttool.Server.
type ChildStats struct {
	InputTokens  int
	OutputTokens int
	ToolCalls    int
	CostUSD      float64
}

// ChildStatsProvider is optionally implemented by ToolCallers that track child stats.
type ChildStatsProvider interface {
	ChildStats() ChildStats
}

// harnessClientWrapper adapts harness.Client to the ToolCaller interface.
type harnessClientWrapper struct {
	client *harness.Client
}

func (h *harnessClientWrapper) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	return h.client.CallTool(ctx, name, args)
}

func (h *harnessClientWrapper) Close() {
	h.client.Close()
}

// ManagedServer wraps an MCP server connection with its config and discovered tools.
type ManagedServer struct {
	Ref    agentcfg.ServerRef
	Client *harness.Client   // nil for agent-as-tool servers
	Caller ToolCaller        // used for tool calls
	Tools  []protocol.Tool
}

// AgentToolWirer can set up an agent-as-tool from a ServerRef.
// Returns the tool definition and a ToolCaller for invocations.
// This is implemented by the CLI layer to avoid a circular dependency with agenttool.
type AgentToolWirer interface {
	WireAgentTool(ctx context.Context, ref agentcfg.ServerRef, workDir string) (protocol.Tool, ToolCaller, error)
}

// Manager handles the lifecycle of MCP servers for an agent.
type Manager struct {
	servers        []*ManagedServer
	agentToolWirer AgentToolWirer
}

// NewManager creates a new server manager.
func NewManager() *Manager {
	return &Manager{}
}

// SetAgentToolWirer sets the handler for agent-as-tool references.
func (m *Manager) SetAgentToolWirer(w AgentToolWirer) {
	m.agentToolWirer = w
}

// StartServer launches and initializes an MCP server.
// For agent-as-tool references (ref.IsAgentRef()), it delegates to the
// AgentToolWirer if one has been set via SetAgentToolWirer.
func (m *Manager) StartServer(ctx context.Context, ref agentcfg.ServerRef, workDir string) (*ManagedServer, error) {
	// Handle agent-as-tool references
	if ref.IsAgentRef() {
		if m.agentToolWirer == nil {
			return nil, fmt.Errorf("server %s is an agent reference but no agent-tool wirer is configured", ref.Name)
		}
		agentPath := ref.Agent
		if !filepath.IsAbs(agentPath) {
			agentPath = filepath.Join(workDir, agentPath)
		}
		tool, caller, err := m.agentToolWirer.WireAgentTool(ctx, ref, agentPath)
		if err != nil {
			return nil, fmt.Errorf("wire agent-as-tool %s: %w", ref.Name, err)
		}
		return m.RegisterAgentTool(ref, tool, caller), nil
	}

	// Determine command and server directory
	command := ref.Command
	dir := ref.Path
	if dir == "" {
		dir = workDir
	}

	// Try to load server config for command and env var declarations
	var requiredEnv []string
	if srvCfg, err := serverconfig.LoadServerConfig(dir); err == nil {
		if command == "" {
			command = srvCfg.Server.Command
		}
		requiredEnv = srvCfg.Server.Env
	}

	if command == "" && ref.Path != "" {
		command = fmt.Sprintf("uv run %s", ref.Name) // fallback
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("no command for server %s", ref.Name)
	}

	// Resolve env vars: process env → per-server .env → error if missing
	extraEnv, err := env.ResolveServerEnv(dir, requiredEnv)
	if err != nil {
		return nil, fmt.Errorf("server %s: %w", ref.Name, err)
	}

	client, err := harness.StartWithEnv(ctx, parts[0], parts[1:], dir, extraEnv)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", ref.Name, err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("list tools from %s: %w", ref.Name, err)
	}

	// Apply tool filtering if configured
	filtered := FilterTools(tools, ref.Tools)

	ms := &ManagedServer{
		Ref:    ref,
		Client: client,
		Caller: &harnessClientWrapper{client: client},
		Tools:  filtered,
	}

	m.servers = append(m.servers, ms)
	return ms, nil
}

// RegisterAgentTool adds an agent-as-tool to the manager with a pre-built tool definition and caller.
func (m *Manager) RegisterAgentTool(ref agentcfg.ServerRef, tool protocol.Tool, caller ToolCaller) *ManagedServer {
	ms := &ManagedServer{
		Ref:    ref,
		Caller: caller,
		Tools:  []protocol.Tool{tool},
	}
	m.servers = append(m.servers, ms)
	return ms
}

// RegisterToolProvider adds a custom tool provider (like Palette) to the manager.
func (m *Manager) RegisterToolProvider(name string, tools []protocol.Tool, caller ToolCaller) {
	ms := &ManagedServer{
		Ref:    agentcfg.ServerRef{Name: name},
		Caller: caller,
		Tools:  tools,
	}
	m.servers = append(m.servers, ms)
}

// AllTools returns all tools across all servers, optionally namespaced.
func (m *Manager) AllTools(namespacing string) []NamespacedTool {
	// Check for collisions
	toolCounts := map[string]int{}
	for _, s := range m.servers {
		for _, t := range s.Tools {
			toolCounts[t.Name]++
		}
	}

	hasCollisions := false
	for _, count := range toolCounts {
		if count > 1 {
			hasCollisions = true
			break
		}
	}

	shouldNamespace := namespacing == "always" || (namespacing == "auto" && hasCollisions)

	var tools []NamespacedTool
	for _, s := range m.servers {
		for _, t := range s.Tools {
			nt := NamespacedTool{
				Tool:       t,
				ServerName: s.Ref.Name,
			}
			if shouldNamespace {
				nt.QualifiedName = s.Ref.Name + "." + t.Name
			} else {
				nt.QualifiedName = t.Name
			}
			tools = append(tools, nt)
		}
	}

	return tools
}

// CallTool finds the right server and calls the tool.
func (m *Manager) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	// Try exact match first (namespaced: server.tool)
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		serverName, toolName := parts[0], parts[1]
		for _, s := range m.servers {
			if s.Ref.Name == serverName {
				return s.Caller.CallTool(ctx, toolName, args)
			}
		}
	}

	// Try non-namespaced match
	for _, s := range m.servers {
		for _, t := range s.Tools {
			if t.Name == name {
				return s.Caller.CallTool(ctx, name, args)
			}
		}
	}

	return nil, 0, fmt.Errorf("tool not found: %s", name)
}

// NamedChildStats pairs a server name with its stats.
type NamedChildStats struct {
	Name  string
	Stats ChildStats
}

// AggregateChildStats collects stats from all agent-as-tool servers.
func (m *Manager) AggregateChildStats() (ChildStats, []NamedChildStats) {
	if m == nil {
		return ChildStats{}, nil
	}
	var total ChildStats
	var perAgent []NamedChildStats
	for _, s := range m.servers {
		if provider, ok := s.Caller.(ChildStatsProvider); ok {
			cs := provider.ChildStats()
			total.InputTokens += cs.InputTokens
			total.OutputTokens += cs.OutputTokens
			total.ToolCalls += cs.ToolCalls
			total.CostUSD += cs.CostUSD
			if cs.ToolCalls > 0 {
				perAgent = append(perAgent, NamedChildStats{Name: s.Ref.Name, Stats: cs})
			}
		}
	}
	return total, perAgent
}

// Close shuts down all managed servers.
func (m *Manager) Close() {
	for _, s := range m.servers {
		if s.Caller != nil {
			s.Caller.Close()
		}
	}
	m.servers = nil
}

// NamespacedTool wraps a tool with its qualified name.
type NamespacedTool struct {
	protocol.Tool
	ServerName    string
	QualifiedName string
}

// DiscoverTools connects to a server and returns its tools without keeping the connection.
func DiscoverTools(ctx context.Context, command string, dir string) ([]protocol.Tool, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	client, err := harness.Start(ctx, parts[0], parts[1:], dir)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.ListTools(ctx)
}
