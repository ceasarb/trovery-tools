package agenttool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/provider"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/runtime"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/protocol"
)

const DefaultMaxDepth = 3

// ProviderFactory creates a provider from an agent config.
type ProviderFactory func(cfg *agentcfg.AgentConfig) (provider.Provider, error)

// Server wraps an agent as an MCP tool, implementing the interface expected by servermgr.
type Server struct {
	agentPath      string
	config         *agentcfg.AgentConfig
	expose         *agentcfg.ExposeConfig
	maxDepth       int
	currentDepth   int
	providerFactory ProviderFactory
	activitySink   runtime.ActivitySink

	// Aggregated stats from child execution
	TokensUsed  int
	InputTokens int
	OutputTokens int
	ToolCalls   int
	CostUSD     float64
}

// SetActivitySink sets a callback for forwarding child agent activity events.
func (s *Server) SetActivitySink(sink runtime.ActivitySink) {
	s.activitySink = sink
}

// New creates an agent-as-tool server.
func New(agentPath string, providerFactory ProviderFactory, currentDepth, maxDepth int) (*Server, error) {
	cfg, err := agentcfg.Load(agentPath)
	if err != nil {
		return nil, fmt.Errorf("load agent config: %w", err)
	}

	if cfg.Expose == nil || !cfg.Expose.AsTool {
		return nil, fmt.Errorf("agent %q does not expose itself as a tool (missing expose.as_tool: true)", cfg.Name)
	}

	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}

	return &Server{
		agentPath:       agentPath,
		config:          cfg,
		expose:          cfg.Expose,
		maxDepth:        maxDepth,
		currentDepth:    currentDepth,
		providerFactory: providerFactory,
	}, nil
}

// ExposedTool returns the MCP tool definition for this agent.
func (s *Server) ExposedTool() protocol.Tool {
	toolName := s.expose.ToolName
	if toolName == "" {
		toolName = s.config.Name
	}

	desc := s.expose.Description
	if desc == "" {
		desc = fmt.Sprintf("Invoke the %s agent", s.config.Name)
	}

	// Simple input schema: { "message": string }
	schema := json.RawMessage(`{"type":"object","properties":{"message":{"type":"string","description":"The message to send to the agent"}},"required":["message"]}`)

	return protocol.Tool{
		Name:        toolName,
		Description: desc,
		InputSchema: schema,
	}
}

// CallTool executes the child agent with the given arguments.
func (s *Server) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	start := time.Now()

	// Depth check
	if s.currentDepth >= s.maxDepth {
		return &protocol.ToolCallResult{
			Content: []protocol.ContentBlock{{Type: "text", Text: "Max agent depth exceeded"}},
			IsError: true,
		}, time.Since(start), nil
	}

	// Extract message from args
	message, err := extractMessage(args)
	if err != nil {
		return nil, time.Since(start), err
	}

	// Create provider
	prov, err := s.providerFactory(s.config)
	if err != nil {
		return nil, time.Since(start), fmt.Errorf("init provider: %w", err)
	}

	// Start child's MCP servers
	mgr := servermgr.NewManager()
	defer mgr.Close()

	for _, srv := range s.config.Servers {
		if srv.IsAgentRef() {
			continue // Don't recursively start nested agent-as-tool servers here
		}
		serverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := mgr.StartServer(serverCtx, srv, s.agentPath)
		cancel()
		if err != nil {
			// Non-fatal: continue without this server
			continue
		}
	}

	// Run the agent with nested output so tool calls are visible
	sess := runtime.NewSession(s.config, prov, mgr)

	var responseText string
	sess.Output = runtime.NestedOutput(s.config.Name, func(text string) {
		responseText += text
	}, s.activitySink)

	if err := sess.SendMessage(ctx, message); err != nil {
		return nil, time.Since(start), err
	}

	// Aggregate stats
	s.TokensUsed += sess.TotalInput + sess.TotalOutput
	s.InputTokens += sess.TotalInput
	s.OutputTokens += sess.TotalOutput
	s.ToolCalls += sess.ToolCalls
	s.CostUSD += sess.EstimatedCost()

	duration := time.Since(start)
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{{Type: "text", Text: responseText}},
	}, duration, nil
}

// ChildStats returns aggregated stats from all child agent executions.
func (s *Server) ChildStats() servermgr.ChildStats {
	return servermgr.ChildStats{
		InputTokens:  s.InputTokens,
		OutputTokens: s.OutputTokens,
		ToolCalls:    s.ToolCalls,
		CostUSD:      s.CostUSD,
	}
}

// Close is a no-op for agent-as-tool servers (child resources are per-call).
func (s *Server) Close() {}

// extractMessage pulls the "message" field from tool call args.
func extractMessage(args interface{}) (string, error) {
	if args == nil {
		return "", fmt.Errorf("no arguments provided")
	}

	switch v := args.(type) {
	case map[string]interface{}:
		if msg, ok := v["message"].(string); ok {
			return msg, nil
		}
		return "", fmt.Errorf("missing 'message' field in arguments")
	case string:
		return v, nil
	default:
		// Try JSON round-trip
		data, err := json.Marshal(args)
		if err != nil {
			return "", fmt.Errorf("cannot parse arguments: %w", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return string(data), nil
		}
		if msg, ok := m["message"].(string); ok {
			return msg, nil
		}
		return string(data), nil
	}
}
