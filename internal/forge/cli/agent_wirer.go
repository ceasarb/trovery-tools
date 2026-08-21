package cli

import (
	"context"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/agenttool"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/provider"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/runtime"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

// agentToolWirer implements servermgr.AgentToolWirer using agenttool.Server.
type agentToolWirer struct {
	providerFactory agenttool.ProviderFactory
	currentDepth    int
	maxDepth        int
	activitySink    runtime.ActivitySink
}

// newAgentToolWirer creates a wirer that resolves agent-as-tool references.
func newAgentToolWirer(pf agenttool.ProviderFactory) *agentToolWirer {
	return &agentToolWirer{
		providerFactory: pf,
		currentDepth:    0,
		maxDepth:        agenttool.DefaultMaxDepth,
	}
}

func (w *agentToolWirer) WireAgentTool(_ context.Context, ref agentcfg.ServerRef, agentPath string) (protocol.Tool, servermgr.ToolCaller, error) {
	srv, err := agenttool.New(agentPath, w.providerFactory, w.currentDepth+1, w.maxDepth)
	if err != nil {
		return protocol.Tool{}, nil, err
	}
	if w.activitySink != nil {
		srv.SetActivitySink(w.activitySink)
	}
	return srv.ExposedTool(), srv, nil
}

// providerFactoryFromFunc wraps initProvider into an agenttool.ProviderFactory.
func providerFactoryFromFunc() agenttool.ProviderFactory {
	return func(cfg *agentcfg.AgentConfig) (provider.Provider, error) {
		return initProvider(cfg)
	}
}
