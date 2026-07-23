package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/provider"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/console"
)

// ToolCallEvent is emitted after a tool call completes.
type ToolCallEvent struct {
	ToolName   string
	ServerName string
	Arguments  map[string]any
	Result     string
	Error      string
	Duration   time.Duration
}

// TurnEvent is emitted after an assistant turn completes (before tool calls).
type TurnEvent struct {
	Text       string
	TokensIn   int
	TokensOut  int
	StopReason string
}

// Hooks allows callers to observe runtime events without modifying the loop.
type Hooks struct {
	// OnAssistantTurn is called after each model response.
	OnAssistantTurn func(event TurnEvent)
	// OnToolCall is called after each tool invocation.
	OnToolCall func(event ToolCallEvent)
}

// Output controls how the runtime presents output during message processing.
// For CLI usage, this prints to stdout. For HTTP servers, this can stream SSE or collect silently.
type Output struct {
	OnText             func(text string)
	OnToolStart        func(name string)
	OnToolResult       func(name, summary string, elapsed time.Duration)
	OnDone             func()
	OnMaxTools         func()
	OnProcessingStart  func(label string)
	OnProcessingStop   func(label string, elapsed time.Duration, cost float64)
}

// DefaultOutput returns an Output that prints to stdout (CLI behavior).
func DefaultOutput() Output {
	var spinner *console.Spinner

	return Output{
		OnText: func(text string) {
			fmt.Print(text)
		},
		OnToolStart: func(name string) {
			fmt.Println()
			console.Dim(fmt.Sprintf("  ▸ calling %s ...", name))
		},
		OnToolResult: func(name, summary string, elapsed time.Duration) {
			console.Dim(fmt.Sprintf("  ✓ %s (%s)", name, elapsed.Round(time.Millisecond)))
			console.Dim(fmt.Sprintf("  → %s", summary))
		},
		OnDone: func() {
			fmt.Println()
		},
		OnMaxTools: func() {
			console.Warning("Max tool calls reached")
		},
		OnProcessingStart: func(label string) {
			spinner = console.StartSpinner(label)
		},
		OnProcessingStop: func(label string, elapsed time.Duration, cost float64) {
			if spinner != nil {
				costStr := ""
				if cost > 0 {
					costStr = fmt.Sprintf(", ~$%.4f", cost)
				}
				spinner.StopWithMessage(fmt.Sprintf("✓ %s (%s%s)", label, elapsed.Round(time.Millisecond), costStr))
				spinner = nil
			}
		},
	}
}

// SilentOutput returns an Output that discards all presentation output.
func SilentOutput() Output {
	return Output{
		OnText:            func(string) {},
		OnToolStart:       func(string) {},
		OnToolResult:      func(string, string, time.Duration) {},
		OnDone:            func() {},
		OnMaxTools:        func() {},
		OnProcessingStart: func(string) {},
		OnProcessingStop:  func(string, time.Duration, float64) {},
	}
}

// ActivitySink receives real-time activity events from the runtime.
// Used to forward child agent activity to external systems (e.g., Palette viewer).
type ActivitySink func(event, content string)

// NestedOutput returns an Output for child agents that shows tool calls
// indented under the parent context. Text is captured via onText but not
// printed to stdout (the parent gets the full response as a tool result).
// An optional ActivitySink forwards events to external observers.
func NestedOutput(agentName string, onText func(string), sinks ...ActivitySink) Output {
	indent := "    "
	emit := func(event, content string) {
		for _, sink := range sinks {
			if sink != nil {
				sink(event, content)
			}
		}
	}

	return Output{
		OnText: func(text string) {
			if onText != nil {
				onText(text)
			}
		},
		OnToolStart: func(name string) {
			console.Dim(fmt.Sprintf("%s↳ %s: calling %s ...", indent, agentName, name))
			emit("tool_start", fmt.Sprintf("%s: %s", agentName, name))
		},
		OnToolResult: func(name, summary string, elapsed time.Duration) {
			console.Dim(fmt.Sprintf("%s↳ %s: ✓ %s (%s)", indent, agentName, name, elapsed.Round(time.Millisecond)))
			emit("tool_result", fmt.Sprintf("%s: %s (%s)", agentName, name, elapsed.Round(time.Millisecond)))
		},
		OnDone:    func() {},
		OnMaxTools: func() {},
		OnProcessingStart: func(label string) {
			console.Dim(fmt.Sprintf("%s↳ %s: %s ...", indent, agentName, label))
			emit("thinking", fmt.Sprintf("%s: %s", agentName, label))
		},
		OnProcessingStop: func(label string, elapsed time.Duration, cost float64) {
			console.Dim(fmt.Sprintf("%s↳ %s: ✓ %s (%s)", indent, agentName, label, elapsed.Round(time.Millisecond)))
		},
	}
}

// BudgetChecker is called after each model turn to enforce per-request cost limits.
// Return a non-nil error to stop the tool-call loop.
type BudgetChecker func(currentCost float64) error

// Session tracks an agent chat session.
type Session struct {
	Config        *agentcfg.AgentConfig
	Provider      provider.Provider
	ServerMgr     *servermgr.Manager
	Messages      []provider.Message
	TotalInput    int
	TotalOutput   int
	ToolCalls     int
	StartTime     time.Time
	Hooks         Hooks
	Output        Output
	BudgetCheck   BudgetChecker // optional per-request budget enforcement
	BudgetStopped bool          // true if the loop was stopped by budget check
}

// CostPerMillion is a rough cost estimate (USD per million tokens).
type CostPerMillion struct {
	Input  float64
	Output float64
}

// Rough pricing — will be made configurable later
var modelCosts = map[string]CostPerMillion{
	// Claude 4.6
	"claude-opus-4-6":           {Input: 5.0, Output: 25.0},
	"claude-sonnet-4-6":         {Input: 3.0, Output: 15.0},
	"claude-haiku-4-5-20251001": {Input: 1.0, Output: 5.0},
	// Legacy
	"claude-sonnet-4-20250514": {Input: 3.0, Output: 15.0},
	"claude-opus-4-20250514":   {Input: 15.0, Output: 75.0},
	// OpenAI — GPT-5.x
	"gpt-5.4":      {Input: 2.5, Output: 15.0},
	"gpt-5.4-pro":  {Input: 30.0, Output: 180.0},
	"gpt-5.2":      {Input: 1.75, Output: 14.0},
	"gpt-5.1":      {Input: 1.25, Output: 10.0},
	"gpt-5":        {Input: 1.25, Output: 10.0},
	"gpt-5-mini":   {Input: 0.25, Output: 2.0},
	"gpt-5-nano":   {Input: 0.05, Output: 0.4},
	// OpenAI — GPT-4.x
	"gpt-4.1":      {Input: 2.0, Output: 8.0},
	"gpt-4.1-mini": {Input: 0.4, Output: 1.6},
	"gpt-4.1-nano": {Input: 0.1, Output: 0.4},
	"gpt-4o":       {Input: 2.5, Output: 10.0},
	"gpt-4o-mini":  {Input: 0.15, Output: 0.6},
	// Ollama — local models, zero cost
	"llama3.1":     {Input: 0, Output: 0},
	"llama3.2":     {Input: 0, Output: 0},
	"qwen2.5":      {Input: 0, Output: 0},
	"mistral":      {Input: 0, Output: 0},
	"deepseek-r1":  {Input: 0, Output: 0},
}

// NewSession creates a new agent runtime session with default stdout output.
func NewSession(cfg *agentcfg.AgentConfig, prov provider.Provider, mgr *servermgr.Manager) *Session {
	return &Session{
		Config:    cfg,
		Provider:  prov,
		ServerMgr: mgr,
		StartTime: time.Now(),
		Output:    DefaultOutput(),
	}
}

// SendMessage processes a user message through the full tool-call loop.
func (s *Session) SendMessage(ctx context.Context, userText string) error {
	// Add user message
	s.Messages = append(s.Messages, provider.Message{
		Role: "user",
		Content: []provider.Content{
			{Type: "text", Text: userText},
		},
	})

	// Build tool definitions
	tools := s.buildToolDefs()

	// Tool-call loop
	var pendingProcessing bool
	var processingStart time.Time
	var processingLabel string

	maxIterations := s.Config.Settings.MaxToolCalls
	if maxIterations <= 0 {
		maxIterations = 50 // sensible default — prevents infinite loops while allowing complex workflows
	}
	for i := 0; i < maxIterations+1; i++ {
		// Call model with streaming
		var textParts []string
		resp, err := s.Provider.CreateMessageStream(s.Messages, tools, s.Config.Model, s.Config.System, func(event provider.StreamEvent) {
			// Stop the processing spinner on first streaming event
			if pendingProcessing {
				pendingProcessing = false
				s.Output.OnProcessingStop(processingLabel, time.Since(processingStart), s.EstimatedCost())
			}
			switch event.Type {
			case "text":
				s.Output.OnText(event.Text)
				textParts = append(textParts, event.Text)
			case "tool_use_start":
				s.Output.OnToolStart(event.Name)
			case "done":
				s.Output.OnDone()
			}
		})
		if err != nil {
			console.Error(fmt.Sprintf("Model error: %v", err))
			return err
		}

		// Track usage
		s.TotalInput += resp.Usage.InputTokens
		s.TotalOutput += resp.Usage.OutputTokens

		// Check per-request budget after each model turn
		if s.BudgetCheck != nil {
			if err := s.BudgetCheck(s.EstimatedCost()); err != nil {
				s.BudgetStopped = true
				// Still build the assistant message so the partial response is available
			}
		}

		// Build assistant message content
		var assistantContent []provider.Content

		// Add text content
		fullText := ""
		for _, t := range textParts {
			fullText += t
		}
		if fullText != "" {
			assistantContent = append(assistantContent, provider.Content{
				Type: "text",
				Text: fullText,
			})
		}

		// Add tool use content from response
		var toolUses []provider.Content
		for _, c := range resp.Content {
			if c.Type == "tool_use" {
				assistantContent = append(assistantContent, c)
				toolUses = append(toolUses, c)
			}
		}

		// Add assistant message
		s.Messages = append(s.Messages, provider.Message{
			Role:    "assistant",
			Content: assistantContent,
		})

		// Fire assistant turn hook
		if s.Hooks.OnAssistantTurn != nil {
			s.Hooks.OnAssistantTurn(TurnEvent{
				Text:       fullText,
				TokensIn:   resp.Usage.InputTokens,
				TokensOut:  resp.Usage.OutputTokens,
				StopReason: resp.StopReason,
			})
		}

		// If no tool calls, we're done
		if len(toolUses) == 0 {
			return nil
		}

		// If budget was exceeded this turn, stop before executing more tool calls
		if s.BudgetStopped {
			return nil
		}

		// Execute tool calls (parallel or sequential)
		var toolResults []provider.Content
		if s.Config.Settings.ParallelToolCalls && len(toolUses) > 1 {
			toolResults = s.executeToolsParallel(ctx, toolUses)
		} else {
			toolResults = s.executeToolsSequential(ctx, toolUses)
		}

		// Add tool results as user message
		s.Messages = append(s.Messages, provider.Message{
			Role:    "user",
			Content: toolResults,
		})

		// Start processing indicator — model is about to crunch tool results
		processingLabel = fmt.Sprintf("processing %d tool result(s)", len(toolResults))
		processingStart = time.Now()
		pendingProcessing = true
		s.Output.OnProcessingStart(processingLabel)
	}

	s.Output.OnMaxTools()
	return nil
}

// EstimatedCost returns the estimated cost in USD.
func (s *Session) EstimatedCost() float64 {
	return EstimateCost(s.Config.Model.Model, s.TotalInput, s.TotalOutput)
}

// EstimateCost calculates estimated cost for a given model and token counts.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	cost, ok := modelCosts[model]
	if !ok {
		return 0
	}
	return (float64(inputTokens)/1_000_000)*cost.Input + (float64(outputTokens)/1_000_000)*cost.Output
}

// Summary returns a session summary string including child agent stats.
func (s *Session) Summary() string {
	duration := time.Since(s.StartTime)
	turns := len(s.Messages) / 2 // rough
	cost := s.EstimatedCost()

	childTotal, perAgent := s.ServerMgr.AggregateChildStats()

	totalToolCalls := s.ToolCalls + childTotal.ToolCalls
	totalInput := s.TotalInput + childTotal.InputTokens
	totalOutput := s.TotalOutput + childTotal.OutputTokens
	totalCost := cost + childTotal.CostUSD

	summary := fmt.Sprintf(
		"Session: %d turns, %d tool calls, %d input / %d output tokens, ~$%.4f, %s",
		turns, totalToolCalls, totalInput, totalOutput, totalCost, duration.Round(time.Second),
	)

	if len(perAgent) > 0 {
		summary += fmt.Sprintf(
			"\n  ├─ %s: %d tool calls, %d in / %d out, ~$%.4f",
			s.Config.Name, s.ToolCalls, s.TotalInput, s.TotalOutput, cost,
		)
		for i, agent := range perAgent {
			connector := "├─"
			if i == len(perAgent)-1 {
				connector = "└─"
			}
			summary += fmt.Sprintf(
				"\n  %s %s: %d tool calls, %d in / %d out, ~$%.4f",
				connector, agent.Name, agent.Stats.ToolCalls, agent.Stats.InputTokens, agent.Stats.OutputTokens, agent.Stats.CostUSD,
			)
		}
	}

	return summary
}

// executeToolsSequential runs tool calls one at a time.
func (s *Session) executeToolsSequential(ctx context.Context, toolUses []provider.Content) []provider.Content {
	var results []provider.Content
	for _, tu := range toolUses {
		result := s.executeSingleTool(ctx, tu)
		results = append(results, result)
	}
	return results
}

// executeToolsParallel runs tool calls concurrently and collects results in order.
func (s *Session) executeToolsParallel(ctx context.Context, toolUses []provider.Content) []provider.Content {
	results := make([]provider.Content, len(toolUses))
	var wg sync.WaitGroup

	for i, tu := range toolUses {
		wg.Add(1)
		go func(idx int, call provider.Content) {
			defer wg.Done()
			results[idx] = s.executeSingleTool(ctx, call)
		}(i, tu)
	}

	wg.Wait()
	return results
}

// executeSingleTool executes one tool call and returns the result content.
func (s *Session) executeSingleTool(ctx context.Context, tu provider.Content) provider.Content {
	s.ToolCalls++

	start := time.Now()
	result, duration, err := s.ServerMgr.CallTool(ctx, tu.Name, tu.Input)
	if duration == 0 {
		duration = time.Since(start)
	}

	var resultText string
	var errText string
	if err != nil {
		resultText = fmt.Sprintf("Error: %v", err)
		errText = err.Error()
	} else {
		for _, c := range result.Content {
			if c.Type == "text" {
				resultText += c.Text
			}
		}
	}

	// Show inline result
	summary := resultText
	if len(summary) > 100 {
		summary = summary[:100] + "..."
	}
	s.Output.OnToolResult(tu.Name, summary, duration)

	// Fire tool call hook
	if s.Hooks.OnToolCall != nil {
		args := argsToMap(tu.Input)
		serverName := serverNameFromTool(tu.Name)
		s.Hooks.OnToolCall(ToolCallEvent{
			ToolName:   tu.Name,
			ServerName: serverName,
			Arguments:  args,
			Result:     resultText,
			Error:      errText,
			Duration:   duration,
		})
	}

	return provider.Content{
		Type:      "tool_result",
		ToolUseID: tu.ID,
		Content:   resultText,
	}
}

func (s *Session) buildToolDefs() []provider.ToolDef {
	nsTools := s.ServerMgr.AllTools(s.Config.Settings.Namespacing)
	defs := make([]provider.ToolDef, len(nsTools))
	for i, t := range nsTools {
		var schema interface{}
		if len(t.InputSchema) > 0 {
			json.Unmarshal(t.InputSchema, &schema)
		}
		defs[i] = provider.ToolDef{
			Name:        t.QualifiedName,
			Description: t.Description,
			InputSchema: schema,
		}
	}
	return defs
}

// argsToMap converts a provider tool input (interface{}) to map[string]any.
func argsToMap(input interface{}) map[string]any {
	if input == nil {
		return nil
	}
	if m, ok := input.(map[string]any); ok {
		return m
	}
	// Try round-trip via JSON for other types
	data, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// serverNameFromTool extracts the server name from a qualified tool name (server.tool).
// For non-namespaced tools, returns "unknown".
func serverNameFromTool(name string) string {
	for i, c := range name {
		if c == '.' {
			return name[:i]
		}
	}
	return "unknown"
}
