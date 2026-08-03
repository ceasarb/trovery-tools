package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/provider"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/runtime"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/env"
)

// AgentResult holds the output of a single child agent execution.
type AgentResult struct {
	Name     string
	Response string
	Duration time.Duration
	Tokens   int
	Cost     float64
	Error    error
}

// Result holds the final orchestration output.
type Result struct {
	Response     string
	AgentResults []AgentResult
	TotalTokens  int
	TotalCost    float64
	Duration     time.Duration
}

// ProviderFactory creates a provider from an agent config.
type ProviderFactory func(cfg *agentcfg.AgentConfig) (provider.Provider, error)

// Engine executes an orchestrator DAG.
type Engine struct {
	dag             *DAG
	handoff         string
	providerFactory ProviderFactory
	parentProvider  provider.Provider
	parentConfig    *agentcfg.AgentConfig
	output          runtime.Output
	agentToolWirer  servermgr.AgentToolWirer
}

// New creates an orchestrator engine from config.
func New(cfg *agentcfg.AgentConfig, parentProvider provider.Provider, providerFactory ProviderFactory) (*Engine, error) {
	if cfg.Orchestrator == nil {
		return nil, fmt.Errorf("agent %q is not an orchestrator", cfg.Name)
	}

	// Build DAG from orchestrator config
	nodes := make([]Node, len(cfg.Orchestrator.Agents))
	for i, a := range cfg.Orchestrator.Agents {
		nodes[i] = Node{
			Name:      a.Name,
			Path:      a.Path,
			DependsOn: a.DependsOn,
		}
	}

	dag, err := BuildDAG(nodes)
	if err != nil {
		return nil, fmt.Errorf("build DAG: %w", err)
	}

	handoff := cfg.Orchestrator.Handoff
	if handoff == "" {
		handoff = "full_output"
	}

	return &Engine{
		dag:             dag,
		handoff:         handoff,
		providerFactory: providerFactory,
		parentProvider:  parentProvider,
		parentConfig:    cfg,
		output:          runtime.DefaultOutput(),
	}, nil
}

// SetOutput configures the output handler for the engine.
func (e *Engine) SetOutput(output runtime.Output) {
	e.output = output
}

// SetAgentToolWirer sets the handler for agent-as-tool references in child agents.
func (e *Engine) SetAgentToolWirer(w servermgr.AgentToolWirer) {
	e.agentToolWirer = w
}

// DAG returns the underlying DAG for inspection.
func (e *Engine) DAG() *DAG {
	return e.dag
}

// Execute runs the orchestrator DAG with the given user input.
func (e *Engine) Execute(ctx context.Context, input string) (*Result, error) {
	start := time.Now()
	var allResults []AgentResult

	// Track outputs by agent name for dependency passing
	outputs := make(map[string]string)

	// Execute layer by layer
	for layerIdx, layer := range e.dag.Layers {
		e.output.OnToolStart(fmt.Sprintf("layer %d: %s", layerIdx+1, strings.Join(layer, ", ")))

		layerResults, err := e.executeLayer(ctx, layer, layerIdx, input, outputs)
		if err != nil {
			return nil, fmt.Errorf("layer %d: %w", layerIdx+1, err)
		}

		for _, r := range layerResults {
			allResults = append(allResults, r)
			if r.Error == nil {
				outputs[r.Name] = r.Response
			}
		}
	}

	// Aggregate all outputs for synthesis
	aggregated := e.aggregate(allResults)

	// Show processing indicator while the orchestrator synthesizes
	synthLabel := fmt.Sprintf("synthesizing %d agent result(s)", len(allResults))
	synthStart := time.Now()
	e.output.OnProcessingStart(synthLabel)

	// Feed to orchestrator's model for final synthesis
	synthesized, tokens, cost, err := e.synthesize(ctx, input, aggregated)

	e.output.OnProcessingStop(synthLabel, time.Since(synthStart), cost)

	if err != nil {
		return nil, fmt.Errorf("synthesis: %w", err)
	}

	totalTokens := tokens
	totalCost := cost
	for _, r := range allResults {
		totalTokens += r.Tokens
		totalCost += r.Cost
	}

	return &Result{
		Response:     synthesized,
		AgentResults: allResults,
		TotalTokens:  totalTokens,
		TotalCost:    totalCost,
		Duration:     time.Since(start),
	}, nil
}

// executeLayer runs all agents in a layer concurrently.
func (e *Engine) executeLayer(ctx context.Context, layer []string, layerIdx int, userInput string, priorOutputs map[string]string) ([]AgentResult, error) {
	results := make([]AgentResult, len(layer))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, agentName := range layer {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()

			// Build input for this agent
			agentInput := e.buildAgentInput(name, userInput, priorOutputs)

			result := e.executeChildAgent(ctx, name, agentInput)

			mu.Lock()
			results[idx] = result
			if result.Error != nil && firstErr == nil {
				firstErr = result.Error
			}
			mu.Unlock()

			if result.Error != nil {
				e.output.OnToolResult(name, fmt.Sprintf("error: %v", result.Error), result.Duration)
			} else {
				summary := result.Response
				if len(summary) > 80 {
					summary = summary[:80] + "..."
				}
				e.output.OnToolResult(name, summary, result.Duration)
			}
		}(i, agentName)
	}

	wg.Wait()
	return results, nil // Individual errors are in results, don't fail the whole layer
}

// executeChildAgent loads and runs a single child agent.
func (e *Engine) executeChildAgent(ctx context.Context, name string, input string) AgentResult {
	start := time.Now()

	node, ok := e.dag.Nodes[name]
	if !ok {
		return AgentResult{Name: name, Error: fmt.Errorf("agent %q not found in DAG", name), Duration: time.Since(start)}
	}

	// Load child agent config
	childCfg, err := agentcfg.Load(node.Path)
	if err != nil {
		return AgentResult{Name: name, Error: fmt.Errorf("load config: %w", err), Duration: time.Since(start)}
	}

	// Load env for child agent
	env.LoadDotenv()

	// Create provider for child agent
	childProvider, err := e.providerFactory(childCfg)
	if err != nil {
		return AgentResult{Name: name, Error: fmt.Errorf("init provider: %w", err), Duration: time.Since(start)}
	}

	// Start child's MCP servers
	mgr := servermgr.NewManager()
	if e.agentToolWirer != nil {
		mgr.SetAgentToolWirer(e.agentToolWirer)
	}
	defer mgr.Close()

	for _, s := range childCfg.Servers {
		serverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := mgr.StartServer(serverCtx, s, node.Path)
		cancel()
		if err != nil {
			return AgentResult{Name: name, Error: fmt.Errorf("start server %s: %w", s.Name, err), Duration: time.Since(start)}
		}
	}

	// Run the agent
	sess := runtime.NewSession(childCfg, childProvider, mgr)
	sess.Output = runtime.SilentOutput()

	var responseText string
	sess.Output.OnText = func(text string) { responseText += text }

	if err := sess.SendMessage(ctx, input); err != nil {
		return AgentResult{Name: name, Error: err, Duration: time.Since(start)}
	}

	return AgentResult{
		Name:     name,
		Response: responseText,
		Duration: time.Since(start),
		Tokens:   sess.TotalInput + sess.TotalOutput,
		Cost:     sess.EstimatedCost(),
	}
}

// buildAgentInput constructs the input for a child agent based on its dependencies.
func (e *Engine) buildAgentInput(agentName, userInput string, priorOutputs map[string]string) string {
	node := e.dag.Nodes[agentName]

	// If no dependencies, use the original user input
	if len(node.DependsOn) == 0 {
		return userInput
	}

	// Build input from dependency outputs
	var parts []string
	parts = append(parts, fmt.Sprintf("Original request: %s", userInput))
	parts = append(parts, "")

	// Sort deps for deterministic ordering
	deps := make([]string, len(node.DependsOn))
	copy(deps, node.DependsOn)
	sort.Strings(deps)

	for _, dep := range deps {
		if output, ok := priorOutputs[dep]; ok {
			parts = append(parts, fmt.Sprintf("Output from %s:\n%s", dep, output))
			parts = append(parts, "")
		}
	}

	return strings.Join(parts, "\n")
}

// aggregate combines all agent results into a single text for synthesis.
func (e *Engine) aggregate(results []AgentResult) string {
	var parts []string
	for _, r := range results {
		if r.Error != nil {
			parts = append(parts, fmt.Sprintf("## %s\n[Error: %v]", r.Name, r.Error))
		} else {
			parts = append(parts, fmt.Sprintf("## %s\n%s", r.Name, r.Response))
		}
	}
	return strings.Join(parts, "\n\n")
}

// synthesize sends the aggregated results to the orchestrator's model for final synthesis.
func (e *Engine) synthesize(ctx context.Context, originalInput, aggregatedResults string) (string, int, float64, error) {
	prompt := fmt.Sprintf(
		"You are synthesizing results from multiple agents.\n\nOriginal request: %s\n\nAgent outputs:\n%s\n\nProvide a unified, coherent response.",
		originalInput, aggregatedResults,
	)

	messages := []provider.Message{
		{Role: "user", Content: []provider.Content{{Type: "text", Text: prompt}}},
	}

	resp, err := e.parentProvider.CreateMessage(messages, nil, e.parentConfig.Model, e.parentConfig.System)
	if err != nil {
		return "", 0, 0, err
	}

	var text string
	for _, c := range resp.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	tokens := resp.Usage.InputTokens + resp.Usage.OutputTokens
	cost := runtime.EstimateCost(e.parentConfig.Model.Model, resp.Usage.InputTokens, resp.Usage.OutputTokens)

	return text, tokens, cost, nil
}
