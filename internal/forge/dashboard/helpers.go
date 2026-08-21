package dashboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/agenttool"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/provider"
	anthropicProvider "github.com/ceasarb/trovery-tools/pkg/forge/agent/provider/anthropic"
	ollamaProvider "github.com/ceasarb/trovery-tools/pkg/forge/agent/provider/ollama"
	openaiProvider "github.com/ceasarb/trovery-tools/pkg/forge/agent/provider/openai"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

// resolveAgentDir finds an agent directory by name within the workspace.
func (s *Server) resolveAgentDir(name string) (string, error) {
	agentDir := filepath.Join(s.workDir, "agents", name)
	if _, err := os.Stat(filepath.Join(agentDir, "agent.yaml")); err == nil {
		return agentDir, nil
	}
	return "", fmt.Errorf("agent not found: %s", name)
}

// initDashboardProvider creates a model provider from agent config.
func initDashboardProvider(cfg *agentcfg.AgentConfig) (provider.Provider, error) {
	switch cfg.Model.Provider {
	case "openai":
		return openaiProvider.New()
	case "anthropic":
		return anthropicProvider.New()
	case "ollama":
		return ollamaProvider.New()
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Model.Provider)
	}
}

// dashboardAgentToolWirer implements servermgr.AgentToolWirer for the dashboard.
type dashboardAgentToolWirer struct{}

func (w *dashboardAgentToolWirer) WireAgentTool(_ context.Context, ref agentcfg.ServerRef, agentPath string) (protocol.Tool, servermgr.ToolCaller, error) {
	factory := func(cfg *agentcfg.AgentConfig) (provider.Provider, error) {
		return initDashboardProvider(cfg)
	}
	srv, err := agenttool.New(agentPath, factory, 1, agenttool.DefaultMaxDepth)
	if err != nil {
		return protocol.Tool{}, nil, err
	}
	return srv.ExposedTool(), srv, nil
}
