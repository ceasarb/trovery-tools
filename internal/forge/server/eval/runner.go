package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ceasarb/trovery-tools/pkg/forge/server/harness"
	"github.com/ceasarb/trovery-tools/pkg/forge/shared/protocol"
)

// ScenarioResult holds the outcome of running a single scenario.
type ScenarioResult struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"` // passed, failed, error
	Duration   time.Duration     `json:"duration"`
	Assertions []AssertionResult `json:"assertions"`
	Error      string            `json:"error,omitempty"`
	ResultData map[string]any    `json:"-"` // raw tool output for snapshot updates
}

// RunScenario executes a single scenario against a running MCP server client.
// It runs setup steps, calls the main tool, and evaluates all assertions.
func RunScenario(ctx context.Context, client *harness.Client, scenario Scenario) ScenarioResult {
	start := time.Now()

	result := ScenarioResult{
		Name: scenario.Name,
	}

	// Execute setup steps
	for i, step := range scenario.Setup {
		_, _, err := client.CallTool(ctx, step.Tool, step.Input)
		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("setup step %d (%s) failed: %v", i+1, step.Tool, err)
			result.Duration = time.Since(start)
			return result
		}
	}

	// Execute main tool call
	toolResult, _, err := client.CallTool(ctx, scenario.Tool, scenario.Input)
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("tool call %s failed: %v", scenario.Tool, err)
		result.Duration = time.Since(start)
		return result
	}

	// Parse the tool result into a map for assertion checking
	data := toolResultToMap(toolResult)
	result.ResultData = data

	// Run assertions
	allPassed := true
	for _, assertion := range scenario.Assertions {
		ar := RunAssertion(assertion, data)
		result.Assertions = append(result.Assertions, ar)
		if !ar.Passed {
			allPassed = false
		}
	}

	if allPassed {
		result.Status = "passed"
	} else {
		result.Status = "failed"
	}

	result.Duration = time.Since(start)
	return result
}

// toolResultToMap converts a ToolCallResult into a map suitable for assertions.
// The map contains:
//   - "text": concatenated text content
//   - "isError": whether the result is an error
//   - "content": array of content blocks
//
// If the text is valid JSON, its fields are merged into the top level.
func toolResultToMap(r *protocol.ToolCallResult) map[string]any {
	if r == nil {
		return map[string]any{}
	}

	data := map[string]any{
		"isError": r.IsError,
	}

	// Build text from content blocks
	var textParts []string
	var contentArr []map[string]any
	for _, c := range r.Content {
		block := map[string]any{
			"type": c.Type,
		}
		if c.Type == "text" {
			block["text"] = c.Text
			textParts = append(textParts, c.Text)
		}
		contentArr = append(contentArr, block)
	}

	text := strings.Join(textParts, "")
	data["text"] = text
	data["content"] = contentArr

	// Try to parse text as JSON and merge fields
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		for k, v := range parsed {
			data[k] = v
		}
	}

	return data
}
