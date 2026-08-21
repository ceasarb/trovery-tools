package eval

import (
	"fmt"
	"regexp"
	"strings"
)

// AssertionResult records whether a single assertion passed or failed.
type AssertionResult struct {
	Type     string `json:"type"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// RunAssertions evaluates all assertions against a captured run.
// Assertions that specify lists (tools, values) expand into multiple checks.
func RunAssertions(assertions []Assertion, captured *CapturedRun) []AssertionResult {
	var results []AssertionResult
	for _, a := range assertions {
		results = append(results, runAssertion(a, captured)...)
	}
	return results
}

func runAssertion(a Assertion, c *CapturedRun) []AssertionResult {
	switch a.Type {

	// --- Tool assertions ---
	case "tool_called":
		return eachTool(a, c, true)
	case "tool_not_called":
		return eachTool(a, c, false)
	case "tool_sequence_includes":
		return []AssertionResult{assertToolSequenceIncludes(a, c)}
	case "min_tool_calls":
		return []AssertionResult{assertMinToolCalls(a, c)}
	case "max_tool_calls":
		return []AssertionResult{assertMaxToolCalls(a, c)}

	// --- Output assertions ---
	case "output_contains":
		return eachValue(a, c, false)
	case "output_not_contains":
		return eachValue(a, c, true)
	case "output_matches":
		return []AssertionResult{assertOutputMatches(a, c)}

	// --- Budget assertions ---
	case "max_turns":
		return []AssertionResult{assertMaxTurns(a, c)}
	case "max_tokens_used":
		return []AssertionResult{assertMaxTokensUsed(a, c)}
	case "max_cost_usd":
		return []AssertionResult{assertMaxCostUSD(a, c)}
	case "max_latency_seconds":
		return []AssertionResult{assertMaxLatencySeconds(a, c)}

	// --- Quality assertions ---
	case "no_tool_errors":
		return []AssertionResult{assertNoToolErrors(a, c)}
	case "no_hallucinated_stats":
		return []AssertionResult{assertNoHallucinatedStats(a, c)}
	case "cites_data_source":
		return []AssertionResult{assertCitesDataSource(a, c)}

	default:
		return []AssertionResult{{
			Type:    a.Type,
			Passed:  false,
			Message: fmt.Sprintf("unknown assertion type: %s", a.Type),
		}}
	}
}

// --- Tool helpers ---

// eachTool expands tools list OR single expected into individual tool_called/tool_not_called checks.
func eachTool(a Assertion, c *CapturedRun, wantCalled bool) []AssertionResult {
	names := resolveToolNames(a)
	if len(names) == 0 {
		return []AssertionResult{{
			Type:    a.Type,
			Passed:  false,
			Message: "no tool name(s) specified — use 'tools: [...]' or 'expected: \"name\"'",
		}}
	}

	var results []AssertionResult
	for _, name := range names {
		if wantCalled {
			results = append(results, checkToolCalled(a.Type, name, c))
		} else {
			results = append(results, checkToolNotCalled(a.Type, name, c))
		}
	}
	return results
}

func resolveToolNames(a Assertion) []string {
	if len(a.Tools) > 0 {
		return a.Tools
	}
	if a.Expected != nil {
		switch v := a.Expected.(type) {
		case string:
			return []string{v}
		case []any:
			return toStringSlice(v)
		}
	}
	return nil
}

func checkToolCalled(aType, toolName string, c *CapturedRun) AssertionResult {
	for _, tc := range c.ToolCalls {
		if tc.Name == toolName {
			return AssertionResult{
				Type:     aType,
				Passed:   true,
				Message:  fmt.Sprintf("tool %q was called", toolName),
				Expected: toolName,
			}
		}
	}
	return AssertionResult{
		Type:     aType,
		Passed:   false,
		Message:  fmt.Sprintf("tool %q was not called", toolName),
		Expected: toolName,
		Actual:   strings.Join(toolNames(c.ToolCalls), ", "),
	}
}

func checkToolNotCalled(aType, toolName string, c *CapturedRun) AssertionResult {
	for _, tc := range c.ToolCalls {
		if tc.Name == toolName {
			return AssertionResult{
				Type:     aType,
				Passed:   false,
				Message:  fmt.Sprintf("tool %q was called but should not have been", toolName),
				Expected: fmt.Sprintf("not %s", toolName),
				Actual:   toolName,
			}
		}
	}
	return AssertionResult{
		Type:     aType,
		Passed:   true,
		Message:  fmt.Sprintf("tool %q was not called", toolName),
		Expected: fmt.Sprintf("not %s", toolName),
	}
}

func assertToolSequenceIncludes(a Assertion, c *CapturedRun) AssertionResult {
	expected := resolveToolNames(a)
	if len(expected) == 0 {
		return AssertionResult{Type: a.Type, Passed: false, Message: "expected a list of tool names"}
	}

	called := toolNames(c.ToolCalls)
	idx := 0
	for _, name := range called {
		if idx < len(expected) && name == expected[idx] {
			idx++
		}
	}

	expectedStr := strings.Join(expected, " -> ")
	actualStr := strings.Join(called, " -> ")

	if idx == len(expected) {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("tool sequence %s found", expectedStr), Expected: expectedStr, Actual: actualStr}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("tool sequence not found: missing %q at position %d", expected[idx], idx), Expected: expectedStr, Actual: actualStr}
}

func assertMinToolCalls(a Assertion, c *CapturedRun) AssertionResult {
	min := resolveInt(a.Count, a.Expected)
	actual := len(c.ToolCalls)
	if actual >= min {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("%d tool calls (min %d)", actual, min), Expected: fmt.Sprintf(">= %d", min), Actual: fmt.Sprintf("%d", actual)}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("%d tool calls, below min %d", actual, min), Expected: fmt.Sprintf(">= %d", min), Actual: fmt.Sprintf("%d", actual)}
}

func assertMaxToolCalls(a Assertion, c *CapturedRun) AssertionResult {
	max := resolveInt(a.Count, a.Expected)
	actual := len(c.ToolCalls)
	if actual <= max {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("%d tool calls (max %d)", actual, max), Expected: fmt.Sprintf("<= %d", max), Actual: fmt.Sprintf("%d", actual)}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("%d tool calls, exceeded max %d", actual, max), Expected: fmt.Sprintf("<= %d", max), Actual: fmt.Sprintf("%d", actual)}
}

// --- Output helpers ---

// eachValue expands values list OR single expected into individual output checks.
func eachValue(a Assertion, c *CapturedRun, negate bool) []AssertionResult {
	vals := resolveValues(a)
	if len(vals) == 0 {
		return []AssertionResult{{Type: a.Type, Passed: false, Message: "no value(s) specified — use 'values: [...]' or 'expected: \"text\"'"}}
	}

	var results []AssertionResult
	for _, v := range vals {
		if negate {
			results = append(results, checkOutputNotContains(a.Type, v, c))
		} else {
			results = append(results, checkOutputContains(a.Type, v, c))
		}
	}
	return results
}

func resolveValues(a Assertion) []string {
	if len(a.Values) > 0 {
		return a.Values
	}
	if a.Expected != nil {
		switch v := a.Expected.(type) {
		case string:
			return []string{v}
		case []any:
			return toStringSlice(v)
		}
	}
	return nil
}

func checkOutputContains(aType, substring string, c *CapturedRun) AssertionResult {
	if strings.Contains(strings.ToLower(c.FinalOutput), strings.ToLower(substring)) {
		return AssertionResult{Type: aType, Passed: true, Message: fmt.Sprintf("output contains %q", substring), Expected: substring}
	}
	actual := c.FinalOutput
	if len(actual) > 200 {
		actual = actual[:200] + "..."
	}
	return AssertionResult{Type: aType, Passed: false, Message: fmt.Sprintf("output does not contain %q", substring), Expected: substring, Actual: actual}
}

func checkOutputNotContains(aType, substring string, c *CapturedRun) AssertionResult {
	if !strings.Contains(strings.ToLower(c.FinalOutput), strings.ToLower(substring)) {
		return AssertionResult{Type: aType, Passed: true, Message: fmt.Sprintf("output does not contain %q", substring), Expected: fmt.Sprintf("not %q", substring)}
	}
	return AssertionResult{Type: aType, Passed: false, Message: fmt.Sprintf("output contains %q but should not", substring), Expected: fmt.Sprintf("not %q", substring), Actual: substring}
}

func assertOutputMatches(a Assertion, c *CapturedRun) AssertionResult {
	pattern := a.Pattern
	if pattern == "" {
		pattern = fmt.Sprintf("%v", a.Expected)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("invalid regex %q: %v", pattern, err)}
	}
	if re.MatchString(c.FinalOutput) {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("output matches %q", pattern), Expected: pattern}
	}
	actual := c.FinalOutput
	if len(actual) > 200 {
		actual = actual[:200] + "..."
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("output does not match %q", pattern), Expected: pattern, Actual: actual}
}

// --- Budget assertions ---

func assertMaxTurns(a Assertion, c *CapturedRun) AssertionResult {
	max := resolveInt(a.Count, a.Expected)
	if c.TurnCount <= max {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("completed in %d turns (max %d)", c.TurnCount, max), Expected: fmt.Sprintf("<= %d", max), Actual: fmt.Sprintf("%d", c.TurnCount)}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("took %d turns, exceeded max %d", c.TurnCount, max), Expected: fmt.Sprintf("<= %d", max), Actual: fmt.Sprintf("%d", c.TurnCount)}
}

func assertMaxTokensUsed(a Assertion, c *CapturedRun) AssertionResult {
	max := resolveInt(a.Tokens, a.Expected)
	if c.TotalTokens <= max {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("used %d tokens (max %d)", c.TotalTokens, max), Expected: fmt.Sprintf("<= %d", max), Actual: fmt.Sprintf("%d", c.TotalTokens)}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("used %d tokens, exceeded max %d", c.TotalTokens, max), Expected: fmt.Sprintf("<= %d", max), Actual: fmt.Sprintf("%d", c.TotalTokens)}
}

func assertMaxCostUSD(a Assertion, c *CapturedRun) AssertionResult {
	max := resolveFloat(a.Amount, a.Expected)
	if c.TotalCost <= max {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("cost $%.4f (max $%.4f)", c.TotalCost, max), Expected: fmt.Sprintf("<= $%.4f", max), Actual: fmt.Sprintf("$%.4f", c.TotalCost)}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("cost $%.4f, exceeded max $%.4f", c.TotalCost, max), Expected: fmt.Sprintf("<= $%.4f", max), Actual: fmt.Sprintf("$%.4f", c.TotalCost)}
}

func assertMaxLatencySeconds(a Assertion, c *CapturedRun) AssertionResult {
	max := resolveFloat(a.Seconds, a.Expected)
	actual := c.Duration.Seconds()
	if actual <= max {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("completed in %.1fs (max %.1fs)", actual, max), Expected: fmt.Sprintf("<= %.1fs", max), Actual: fmt.Sprintf("%.1fs", actual)}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("took %.1fs, exceeded max %.1fs", actual, max), Expected: fmt.Sprintf("<= %.1fs", max), Actual: fmt.Sprintf("%.1fs", actual)}
}

// --- Quality assertions ---

func assertNoToolErrors(a Assertion, c *CapturedRun) AssertionResult {
	var errors []string
	for _, tc := range c.ToolCalls {
		if tc.Error != "" {
			errors = append(errors, fmt.Sprintf("%s: %s", tc.Name, tc.Error))
		}
	}
	if len(errors) == 0 {
		return AssertionResult{Type: a.Type, Passed: true, Message: "no tool errors"}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: fmt.Sprintf("%d tool error(s)", len(errors)), Actual: strings.Join(errors, "; ")}
}

func assertNoHallucinatedStats(a Assertion, c *CapturedRun) AssertionResult {
	// Check that numeric claims in the output can be traced to tool results.
	// Heuristic: extract numbers from output, check they appear in tool results.
	re := regexp.MustCompile(`\d+\.?\d*%|\d+\.?\d+`)
	outputNums := re.FindAllString(c.FinalOutput, -1)

	if len(outputNums) == 0 {
		return AssertionResult{Type: a.Type, Passed: true, Message: "no numeric stats in output to verify"}
	}

	// Collect all numbers from tool results
	var toolData strings.Builder
	for _, tc := range c.ToolCalls {
		toolData.WriteString(tc.Result)
		toolData.WriteString(" ")
	}
	toolText := toolData.String()

	var ungrounded []string
	for _, num := range outputNums {
		if !strings.Contains(toolText, num) {
			ungrounded = append(ungrounded, num)
		}
	}

	// Allow some tolerance — not every number needs to be in tool data
	// (e.g., "3 of 5 games" where 3 and 5 are derived counts)
	if len(ungrounded) > len(outputNums)/2 {
		return AssertionResult{
			Type:    a.Type,
			Passed:  false,
			Message: fmt.Sprintf("%d of %d numeric values not found in tool data", len(ungrounded), len(outputNums)),
			Actual:  strings.Join(ungrounded, ", "),
		}
	}

	return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("%d/%d numeric values grounded in tool data", len(outputNums)-len(ungrounded), len(outputNums))}
}

func assertCitesDataSource(a Assertion, c *CapturedRun) AssertionResult {
	// Check that the output references tool names or data from tool calls.
	if len(c.ToolCalls) == 0 {
		return AssertionResult{Type: a.Type, Passed: true, Message: "no tool calls to cite"}
	}

	outputLower := strings.ToLower(c.FinalOutput)
	cited := 0
	for _, tc := range c.ToolCalls {
		// Check if output references the tool name or key data from its result
		if strings.Contains(outputLower, strings.ToLower(tc.Name)) {
			cited++
			continue
		}
		// Check if significant chunks of tool result appear in output
		if tc.Result != "" && len(tc.Result) > 10 {
			// Take a sample from the result
			sample := tc.Result
			if len(sample) > 50 {
				sample = sample[:50]
			}
			if strings.Contains(outputLower, strings.ToLower(sample)) {
				cited++
			}
		}
	}

	if cited > 0 {
		return AssertionResult{Type: a.Type, Passed: true, Message: fmt.Sprintf("output references %d/%d tool data sources", cited, len(c.ToolCalls))}
	}
	return AssertionResult{Type: a.Type, Passed: false, Message: "output does not appear to cite any tool data sources"}
}

// --- Resolution helpers ---

// resolveInt returns the named field if non-zero, else tries to parse expected.
func resolveInt(named int, expected any) int {
	if named != 0 {
		return named
	}
	return toInt(expected)
}

// resolveFloat returns the named field if non-zero, else tries to parse expected.
func resolveFloat(named float64, expected any) float64 {
	if named != 0 {
		return named
	}
	return toFloat(expected)
}

// --- Shared helpers ---

func toolNames(calls []CapturedToolCall) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
	}
	return names
}

func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []any:
		out := make([]string, len(val))
		for i, item := range val {
			out[i] = fmt.Sprintf("%v", item)
		}
		return out
	case []string:
		return val
	default:
		return nil
	}
}

func toInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	default:
		return 0
	}
}

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
