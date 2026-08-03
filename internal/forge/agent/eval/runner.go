package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/provider"
)

// ScenarioResult holds the outcome of running a single scenario.
type ScenarioResult struct {
	ScenarioName string            `json:"scenario_name"`
	Passed       bool              `json:"passed"`
	Assertions   []AssertionResult `json:"assertions"`
	Captured     *CapturedRun      `json:"captured"`
	Error        string            `json:"error,omitempty"`
	Duration     time.Duration     `json:"duration_ms"`
}

// EventKind identifies the type of live progress event.
type EventKind int

const (
	EventScenarioStart EventKind = iota
	EventModelCall
	EventToolCall
	EventToolResult
	EventScenarioEnd
)

// Event carries live progress info from the runner.
type Event struct {
	Kind         EventKind
	Scenario     string
	Run          int // 1-based, for multi-run
	TotalRuns    int
	ToolName     string
	ToolDuration time.Duration
	ToolError    string
	Duration     time.Duration
	Passed       bool
	TokensUsed   int
}

// Runner executes eval scenarios against a configured agent.
type Runner struct {
	Config   *agentcfg.AgentConfig
	Provider provider.Provider
	Caller   ToolCaller
	Tools    []provider.ToolDef
	OnEvent  func(Event) // optional live progress callback
}

// NewRunner creates a scenario runner.
func NewRunner(cfg *agentcfg.AgentConfig, prov provider.Provider, caller ToolCaller, tools []provider.ToolDef) *Runner {
	return &Runner{
		Config:   cfg,
		Provider: prov,
		Caller:   caller,
		Tools:    tools,
	}
}

func (r *Runner) emit(e Event) {
	if r.OnEvent != nil {
		r.OnEvent(e)
	}
}

// RunScenario executes a single scenario and returns the result.
func (r *Runner) RunScenario(ctx context.Context, scenario Scenario) *ScenarioResult {
	start := time.Now()

	captured, err := r.executeConversation(ctx, scenario)
	duration := time.Since(start)

	if err != nil {
		return &ScenarioResult{
			ScenarioName: scenario.Name,
			Passed:       false,
			Error:        err.Error(),
			Duration:     duration,
			Captured:     captured,
		}
	}

	captured.Duration = duration

	assertions := RunAssertions(scenario.Assertions, captured)

	allPassed := true
	for _, a := range assertions {
		if !a.Passed {
			allPassed = false
			break
		}
	}

	return &ScenarioResult{
		ScenarioName: scenario.Name,
		Passed:       allPassed,
		Assertions:   assertions,
		Captured:     captured,
		Duration:     duration,
	}
}

func (r *Runner) executeConversation(ctx context.Context, scenario Scenario) (*CapturedRun, error) {
	captured := &CapturedRun{}
	var messages []provider.Message

	tools := r.Tools
	maxToolCalls := r.Config.Settings.MaxToolCalls
	if maxToolCalls <= 0 {
		maxToolCalls = 10
	}

	for _, msg := range scenario.Messages {
		messages = append(messages, provider.Message{
			Role: msg.Role,
			Content: []provider.Content{
				{Type: "text", Text: msg.Content},
			},
		})
		captured.TurnCount++

		// Run the tool-call loop for this turn
		for i := 0; i < maxToolCalls+1; i++ {
			r.emit(Event{Kind: EventModelCall, Scenario: scenario.Name, TokensUsed: captured.TotalTokens})

			resp, err := r.Provider.CreateMessage(messages, tools, r.Config.Model, r.Config.System)
			if err != nil {
				return captured, fmt.Errorf("model call: %w", err)
			}

			captured.TotalTokens += resp.Usage.InputTokens + resp.Usage.OutputTokens

			var assistantContent []provider.Content
			var toolUses []provider.Content
			finalText := ""

			for _, c := range resp.Content {
				switch c.Type {
				case "text":
					finalText += c.Text
					assistantContent = append(assistantContent, c)
				case "tool_use":
					assistantContent = append(assistantContent, c)
					toolUses = append(toolUses, c)
				}
			}

			messages = append(messages, provider.Message{
				Role:    "assistant",
				Content: assistantContent,
			})

			if len(toolUses) == 0 {
				captured.FinalOutput = finalText
				break
			}

			// Execute tool calls
			var toolResults []provider.Content
			for _, tu := range toolUses {
				r.emit(Event{Kind: EventToolCall, Scenario: scenario.Name, ToolName: tu.Name})

				callStart := time.Now()
				result, _, err := r.Caller.CallTool(ctx, tu.Name, tu.Input)
				callDuration := time.Since(callStart)

				ct := CapturedToolCall{
					Name:     tu.Name,
					Duration: callDuration,
				}

				// Extract arguments
				if tu.Input != nil {
					if argBytes, marshalErr := json.Marshal(tu.Input); marshalErr == nil {
						var args map[string]any
						json.Unmarshal(argBytes, &args)
						ct.Arguments = args
					}
				}

				var resultText string
				if err != nil {
					ct.Error = err.Error()
					resultText = fmt.Sprintf("Error: %v", err)
					r.emit(Event{Kind: EventToolResult, Scenario: scenario.Name, ToolName: tu.Name, ToolDuration: callDuration, ToolError: err.Error()})
				} else if result != nil {
					for _, c := range result.Content {
						if c.Type == "text" {
							resultText += c.Text
						}
					}
					ct.Result = resultText
					r.emit(Event{Kind: EventToolResult, Scenario: scenario.Name, ToolName: tu.Name, ToolDuration: callDuration})
				}

				captured.ToolCalls = append(captured.ToolCalls, ct)

				toolResults = append(toolResults, provider.Content{
					Type:      "tool_result",
					ToolUseID: tu.ID,
					Content:   resultText,
				})
			}

			messages = append(messages, provider.Message{
				Role:    "user",
				Content: toolResults,
			})
		}
	}

	// Estimate cost using the same logic as runtime
	captured.TotalCost = estimateCost(r.Config.Model.Model, captured.TotalTokens)

	return captured, nil
}

// estimateCost provides a rough cost estimate. Matches runtime/runtime.go logic.
func estimateCost(model string, totalTokens int) float64 {
	// Simplified: use average of input+output pricing per million tokens
	costs := map[string]float64{
		"claude-opus-4-6":           15.0,
		"claude-sonnet-4-6":         9.0,
		"claude-haiku-4-5-20251001": 3.0,
		"gpt-5.4":                   8.75,
		"gpt-5.4-pro":               105.0,
		"gpt-4.1":                   5.0,
		"gpt-4.1-mini":              1.0,
		"gpt-4o":                    6.25,
		"gpt-4o-mini":               0.375,
	}
	avgCost, ok := costs[model]
	if !ok {
		return 0
	}
	return (float64(totalTokens) / 1_000_000) * avgCost
}
