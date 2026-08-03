package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/provider"
)

const apiURL = "https://api.openai.com/v1/chat/completions"

// Provider implements the OpenAI API.
type Provider struct {
	apiKey string
}

// New creates a new OpenAI provider.
func New() (*Provider, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	return &Provider{apiKey: key}, nil
}

// OpenAI request/response types

type apiMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCalls  []apiToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type apiToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function apiFunctionCall `json:"function"`
}

type apiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiTool struct {
	Type     string       `json:"type"`
	Function apiFunction  `json:"function"`
}

type apiFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type apiRequest struct {
	Model         string         `json:"model"`
	Messages      []apiMessage   `json:"messages"`
	Tools         []apiTool      `json:"tools,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Temperature   *float64       `json:"temperature,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type apiStreamToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function apiFunctionCall `json:"function"`
}

type apiStreamDelta struct {
	Role      string              `json:"role,omitempty"`
	Content   json.RawMessage     `json:"content,omitempty"`
	ToolCalls []apiStreamToolCall `json:"tool_calls,omitempty"`
}

type apiResponse struct {
	Choices []struct {
		Message      apiMessage     `json:"message"`
		Delta        apiStreamDelta `json:"delta"`
		FinishReason string         `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

// CreateMessage sends a non-streaming request.
func (p *Provider) CreateMessage(messages []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string) (*provider.Response, error) {
	apiMsgs := convertMessages(messages)
	if system != "" {
		sysContent, _ := json.Marshal(system)
		apiMsgs = append([]apiMessage{{Role: "system", Content: sysContent}}, apiMsgs...)
	}
	apiTools := convertTools(tools)

	req := apiRequest{
		Model:     cfg.Model,
		Messages:  apiMsgs,
		Tools:     apiTools,
		MaxTokens: cfg.MaxTokens,
	}
	if cfg.Temperature > 0 {
		req.Temperature = &cfg.Temperature
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(body)
	if err != nil {
		return nil, err
	}

	return parseResponse(resp)
}

// CreateMessageStream sends a streaming request.
func (p *Provider) CreateMessageStream(messages []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string, handler provider.StreamHandler) (*provider.Response, error) {
	apiMsgs := convertMessages(messages)
	if system != "" {
		sysContent, _ := json.Marshal(system)
		apiMsgs = append([]apiMessage{{Role: "system", Content: sysContent}}, apiMsgs...)
	}
	apiTools := convertTools(tools)

	req := apiRequest{
		Model:         cfg.Model,
		Messages:      apiMsgs,
		Tools:         apiTools,
		MaxTokens:     cfg.MaxTokens,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	if cfg.Temperature > 0 {
		req.Temperature = &cfg.Temperature
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("api error %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	return p.parseSSE(httpResp.Body, handler)
}

func (p *Provider) doRequest(body []byte) (*apiResponse, error) {
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &apiResp, nil
}

func (p *Provider) parseSSE(r io.Reader, handler provider.StreamHandler) (*provider.Response, error) {
	scanner := bufio.NewScanner(r)

	var result provider.Response
	// Track tool calls being built across chunks, keyed by OpenAI's index field
	toolCalls := map[int]*provider.Content{}
	toolArgs := map[int]*strings.Builder{}
	toolOrder := []int{} // preserve ordering

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			handler(provider.StreamEvent{Type: "done"})
			break
		}

		var chunk apiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Usage comes in the final chunk when stream_options.include_usage is set
		if chunk.Usage != nil {
			result.Usage.InputTokens = chunk.Usage.PromptTokens
			result.Usage.OutputTokens = chunk.Usage.CompletionTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// Text content
		var deltaText string
		json.Unmarshal(delta.Content, &deltaText)
		if deltaText != "" {
			handler(provider.StreamEvent{Type: "text", Text: deltaText})
		}

		// Tool calls — use the Index field from OpenAI to track parallel calls
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if tc.ID != "" {
				// New tool call starting at this index
				toolCalls[idx] = &provider.Content{
					Type: "tool_use",
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				toolArgs[idx] = &strings.Builder{}
				toolOrder = append(toolOrder, idx)
				handler(provider.StreamEvent{
					Type: "tool_use_start",
					ID:   tc.ID,
					Name: tc.Function.Name,
				})
			}
			if tc.Function.Arguments != "" {
				if sb, ok := toolArgs[idx]; ok {
					sb.WriteString(tc.Function.Arguments)
				}
			}
		}
	}

	// Finalize tool calls in order
	for _, idx := range toolOrder {
		tc := toolCalls[idx]
		if sb, ok := toolArgs[idx]; ok {
			var input interface{}
			json.Unmarshal([]byte(sb.String()), &input)
			tc.Input = input
		}
		result.Content = append(result.Content, *tc)
	}

	return &result, nil
}

func parseResponse(resp *apiResponse) (*provider.Response, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	msg := resp.Choices[0].Message
	var content []provider.Content

	// Text content
	var text string
	json.Unmarshal(msg.Content, &text)
	if text != "" {
		content = append(content, provider.Content{Type: "text", Text: text})
	}

	// Tool calls
	for _, tc := range msg.ToolCalls {
		var input interface{}
		json.Unmarshal([]byte(tc.Function.Arguments), &input)
		content = append(content, provider.Content{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	result := &provider.Response{
		Content:    content,
		StopReason: resp.Choices[0].FinishReason,
	}
	if resp.Usage != nil {
		result.Usage = provider.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	return result, nil
}

// convertMessages translates our provider.Message to OpenAI format.
func convertMessages(messages []provider.Message) []apiMessage {
	var out []apiMessage
	for _, m := range messages {
		switch m.Role {
		case "user":
			// Check if this is tool results
			hasToolResults := false
			for _, c := range m.Content {
				if c.Type == "tool_result" {
					hasToolResults = true
					break
				}
			}
			if hasToolResults {
				for _, c := range m.Content {
					if c.Type == "tool_result" {
						contentJSON, _ := json.Marshal(c.Content)
						out = append(out, apiMessage{
							Role:       "tool",
							Content:    contentJSON,
							ToolCallID: c.ToolUseID,
						})
					}
				}
			} else {
				text := ""
				for _, c := range m.Content {
					if c.Type == "text" {
						text += c.Text
					}
				}
				contentJSON, _ := json.Marshal(text)
				out = append(out, apiMessage{
					Role:    "user",
					Content: contentJSON,
				})
			}

		case "assistant":
			msg := apiMessage{Role: "assistant"}
			for _, c := range m.Content {
				switch c.Type {
				case "text":
					contentJSON, _ := json.Marshal(c.Text)
					msg.Content = contentJSON
				case "tool_use":
					argsJSON, _ := json.Marshal(c.Input)
					msg.ToolCalls = append(msg.ToolCalls, apiToolCall{
						ID:   c.ID,
						Type: "function",
						Function: apiFunctionCall{
							Name:      c.Name,
							Arguments: string(argsJSON),
						},
					})
				}
			}
			out = append(out, msg)
		}
	}
	return out
}

// convertTools translates our provider.ToolDef to OpenAI format.
func convertTools(tools []provider.ToolDef) []apiTool {
	out := make([]apiTool, len(tools))
	for i, t := range tools {
		out[i] = apiTool{
			Type: "function",
			Function: apiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return out
}
