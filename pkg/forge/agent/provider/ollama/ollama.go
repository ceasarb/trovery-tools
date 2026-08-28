package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	agentcfg "github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
	"github.com/ceasarb/trovery-tools/pkg/forge/agent/provider"
)

const defaultHost = "http://localhost:11434"

// Provider implements the Ollama local model API.
type Provider struct {
	host   string
	client *http.Client
}

// New creates a new Ollama provider.
func New() (*Provider, error) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = defaultHost
	}
	// Strip trailing slash
	host = strings.TrimRight(host, "/")
	// Local generation on modest hardware can legitimately take a long time,
	// so the backstop is very generous — but a wedged local server still
	// cannot hang the caller forever.
	return &Provider{
		host:   host,
		client: provider.NewHTTPClient(30 * time.Minute),
	}, nil
}

// NewWithClient creates a provider with a custom HTTP client (for testing).
func NewWithClient(host string, client *http.Client) *Provider {
	return &Provider{host: host, client: client}
}

// Ollama API types

type ollamaFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ollamaTool struct {
	Type     string         `json:"type"`
	Function ollamaFunction `json:"function"`
}

type ollamaMessage struct {
	Role      string             `json:"role"`
	Content   string             `json:"content,omitempty"`
	ToolCalls []ollamaToolCall   `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	EvalCount          int           `json:"eval_count"`
}

// CreateMessage sends a non-streaming request to Ollama.
func (p *Provider) CreateMessage(messages []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string) (*provider.Response, error) {
	req := p.buildRequest(messages, tools, cfg, system, false)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(body)
	if err != nil {
		return nil, err
	}

	return p.parseResponse(resp), nil
}

// CreateMessageStream sends a streaming request to Ollama.
func (p *Provider) CreateMessageStream(messages []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string, handler provider.StreamHandler) (*provider.Response, error) {
	req := p.buildRequest(messages, tools, cfg, system, true)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", p.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed (is Ollama running at %s?): %w", p.host, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	return p.parseStream(httpResp.Body, handler)
}

func (p *Provider) buildRequest(messages []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string, stream bool) ollamaRequest {
	msgs := convertMessages(messages)
	if system != "" {
		msgs = append([]ollamaMessage{{Role: "system", Content: system}}, msgs...)
	}
	req := ollamaRequest{
		Model:    cfg.Model,
		Messages: msgs,
		Tools:    convertTools(tools),
		Stream:   stream,
	}

	if cfg.Temperature > 0 || cfg.MaxTokens > 0 {
		opts := &ollamaOptions{}
		if cfg.Temperature > 0 {
			opts.Temperature = cfg.Temperature
		}
		if cfg.MaxTokens > 0 {
			opts.NumPredict = cfg.MaxTokens
		}
		req.Options = opts
	}

	return req
}

func (p *Provider) doRequest(body []byte) (*ollamaResponse, error) {
	httpReq, err := http.NewRequest("POST", p.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed (is Ollama running at %s?): %w", p.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	return &ollamaResp, nil
}

func (p *Provider) parseResponse(resp *ollamaResponse) *provider.Response {
	var content []provider.Content

	if resp.Message.Content != "" {
		content = append(content, provider.Content{
			Type: "text",
			Text: resp.Message.Content,
		})
	}

	for i, tc := range resp.Message.ToolCalls {
		content = append(content, provider.Content{
			Type:  "tool_use",
			ID:    fmt.Sprintf("call_%d", i),
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}

	stopReason := "end_turn"
	if len(resp.Message.ToolCalls) > 0 {
		stopReason = "tool_use"
	}

	return &provider.Response{
		Content:    content,
		StopReason: stopReason,
		Usage: provider.Usage{
			InputTokens:  resp.PromptEvalCount,
			OutputTokens: resp.EvalCount,
		},
	}
}

func (p *Provider) parseStream(r io.Reader, handler provider.StreamHandler) (*provider.Response, error) {
	scanner := bufio.NewScanner(r)

	var result provider.Response
	var fullText strings.Builder
	var allToolCalls []ollamaToolCall
	toolCallEmitted := false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var chunk ollamaResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		// Text content
		if chunk.Message.Content != "" {
			fullText.WriteString(chunk.Message.Content)
			handler(provider.StreamEvent{
				Type: "text",
				Text: chunk.Message.Content,
			})
		}

		// Tool calls arrive in the message field
		if len(chunk.Message.ToolCalls) > 0 && !toolCallEmitted {
			allToolCalls = append(allToolCalls, chunk.Message.ToolCalls...)
			for i, tc := range chunk.Message.ToolCalls {
				id := fmt.Sprintf("call_%d", len(allToolCalls)-len(chunk.Message.ToolCalls)+i)
				handler(provider.StreamEvent{
					Type: "tool_use_start",
					ID:   id,
					Name: tc.Function.Name,
				})
			}
			toolCallEmitted = true
		}

		// Final chunk
		if chunk.Done {
			result.Usage.InputTokens = chunk.PromptEvalCount
			result.Usage.OutputTokens = chunk.EvalCount
			handler(provider.StreamEvent{Type: "done"})
			break
		}
	}

	// Build content
	if fullText.Len() > 0 {
		result.Content = append(result.Content, provider.Content{
			Type: "text",
			Text: fullText.String(),
		})
	}

	for i, tc := range allToolCalls {
		result.Content = append(result.Content, provider.Content{
			Type:  "tool_use",
			ID:    fmt.Sprintf("call_%d", i),
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}

	return &result, nil
}

// ListModels queries Ollama for available local models.
func (p *Provider) ListModels() ([]string, error) {
	resp, err := p.client.Get(p.host + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("ollama not reachable at %s: %w", p.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama tags endpoint returned %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode model list: %w", err)
	}

	names := make([]string, len(result.Models))
	for i, m := range result.Models {
		names[i] = m.Name
	}
	return names, nil
}

func convertMessages(messages []provider.Message) []ollamaMessage {
	var out []ollamaMessage

	for _, m := range messages {
		switch m.Role {
		case "user":
			// Check for tool results
			hasToolResults := false
			for _, c := range m.Content {
				if c.Type == "tool_result" {
					hasToolResults = true
					break
				}
			}
			if hasToolResults {
				// Ollama expects tool results as role "tool"
				for _, c := range m.Content {
					if c.Type == "tool_result" {
						out = append(out, ollamaMessage{
							Role:    "tool",
							Content: c.Content,
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
				out = append(out, ollamaMessage{
					Role:    "user",
					Content: text,
				})
			}

		case "assistant":
			msg := ollamaMessage{Role: "assistant"}
			for _, c := range m.Content {
				switch c.Type {
				case "text":
					msg.Content = c.Text
				case "tool_use":
					args, _ := toStringMap(c.Input)
					msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
						Function: ollamaFunctionCall{
							Name:      c.Name,
							Arguments: args,
						},
					})
				}
			}
			out = append(out, msg)
		}
	}

	return out
}

func convertTools(tools []provider.ToolDef) []ollamaTool {
	out := make([]ollamaTool, len(tools))
	for i, t := range tools {
		out[i] = ollamaTool{
			Type: "function",
			Function: ollamaFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return out
}

// toStringMap converts an interface{} to map[string]interface{}.
func toStringMap(v interface{}) (map[string]interface{}, error) {
	if v == nil {
		return map[string]interface{}{}, nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m, nil
	}
	// Try JSON round-trip
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
