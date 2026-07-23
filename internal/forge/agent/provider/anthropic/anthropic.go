package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/provider"
)

const (
	apiURL     = "https://api.anthropic.com/v1/messages"
	maxRetries = 3
)

// Provider implements the Anthropic API.
type Provider struct {
	apiKey string
}

// New creates a new Anthropic provider.
func New() (*Provider, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	return &Provider{apiKey: key}, nil
}

// doRequest executes an HTTP request with retry on 429/529 status codes.
// Returns the response body reader (caller must close) or an error.
func (p *Provider) doRequest(body []byte) (*http.Response, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", p.apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("api request: %w", err)
		}

		// Retry on rate limit (429) or overloaded (529)
		if (resp.StatusCode == 429 || resp.StatusCode == 529) && attempt < maxRetries {
			wait := retryDelay(resp, attempt)
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "  ⏳ Rate limited, retrying in %s (attempt %d/%d)\n", wait.Round(time.Second), attempt+1, maxRetries)
			time.Sleep(wait)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("rate limited after %d retries", maxRetries)
}

// retryDelay determines how long to wait before retrying.
// Uses the Retry-After header if present, otherwise exponential backoff.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	// Exponential backoff: 15s, 30s, 60s
	return time.Duration(15<<attempt) * time.Second
}

type apiRequest struct {
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	System      string            `json:"system,omitempty"`
	Messages    []provider.Message `json:"messages"`
	Tools       []provider.ToolDef `json:"tools,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
}

type apiResponse struct {
	Content    []provider.Content `json:"content"`
	Usage      provider.Usage     `json:"usage"`
	StopReason string             `json:"stop_reason"`
}

// CreateMessage sends a non-streaming request to Anthropic.
func (p *Provider) CreateMessage(messages []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string) (*provider.Response, error) {
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	req := apiRequest{
		Model:     cfg.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
		Tools:     tools,
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
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &provider.Response{
		Content:    apiResp.Content,
		Usage:      apiResp.Usage,
		StopReason: apiResp.StopReason,
	}, nil
}

// CreateMessageStream sends a streaming request to Anthropic.
func (p *Provider) CreateMessageStream(messages []provider.Message, tools []provider.ToolDef, cfg agentcfg.ModelConfig, system string, handler provider.StreamHandler) (*provider.Response, error) {
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	req := apiRequest{
		Model:     cfg.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
		Tools:     tools,
		Stream:    true,
	}
	if cfg.Temperature > 0 {
		req.Temperature = &cfg.Temperature
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Retry loop for both HTTP-level and SSE-level overload errors
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := p.doRequest(body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(bodyBytes))
		}

		result, err := p.parseSSE(resp.Body, handler)
		resp.Body.Close()

		if err != nil && strings.Contains(err.Error(), "overloaded_error") && attempt < maxRetries {
			wait := time.Duration(15<<attempt) * time.Second
			fmt.Fprintf(os.Stderr, "  ⏳ API overloaded, retrying in %s (attempt %d/%d)\n", wait.Round(time.Second), attempt+1, maxRetries)
			time.Sleep(wait)
			continue
		}

		return result, err
	}

	return nil, fmt.Errorf("api overloaded after %d retries", maxRetries)
}

func (p *Provider) parseSSE(r io.Reader, handler provider.StreamHandler) (*provider.Response, error) {
	scanner := bufio.NewScanner(r)
	// Increase buffer for large SSE events (e.g., many tool definitions)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var result provider.Response
	var currentContent []provider.Content
	var currentToolInput strings.Builder
	var currentToolID string
	var currentToolName string
	var eventCount int

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock *struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Text string `json:"text"`
			} `json:"content_block"`
			Delta *struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Message *struct {
				Usage provider.Usage `json:"usage"`
			} `json:"message"`
			Usage *provider.Usage `json:"usage"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			fmt.Fprintf(os.Stderr, "  [debug] SSE parse error: %v (data: %.200s)\n", err, data)
			continue
		}

		eventCount++

		switch event.Type {
		case "error":
			msg := "unknown error"
			if event.Error != nil {
				msg = fmt.Sprintf("%s: %s", event.Error.Type, event.Error.Message)
			}
			return nil, fmt.Errorf("api stream error: %s", msg)

		case "content_block_start":
			if event.ContentBlock != nil {
				if event.ContentBlock.Type == "tool_use" {
					currentToolID = event.ContentBlock.ID
					currentToolName = event.ContentBlock.Name
					currentToolInput.Reset()
					handler(provider.StreamEvent{
						Type: "tool_use_start",
						ID:   currentToolID,
						Name: currentToolName,
					})
				}
			}

		case "content_block_delta":
			if event.Delta != nil {
				switch event.Delta.Type {
				case "text_delta":
					handler(provider.StreamEvent{
						Type: "text",
						Text: event.Delta.Text,
					})
				case "input_json_delta":
					currentToolInput.WriteString(event.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if currentToolID != "" {
				var input interface{} = map[string]interface{}{}
				if s := currentToolInput.String(); s != "" {
					json.Unmarshal([]byte(s), &input)
				}
				currentContent = append(currentContent, provider.Content{
					Type:  "tool_use",
					ID:    currentToolID,
					Name:  currentToolName,
					Input: input,
				})
				currentToolID = ""
				currentToolName = ""
			}

		case "message_start":
			if event.Message != nil {
				result.Usage = event.Message.Usage
			}

		case "message_delta":
			if event.Delta != nil {
				// stop_reason is in delta for streaming
			}
			if event.Usage != nil {
				result.Usage.OutputTokens = event.Usage.OutputTokens
			}

		case "message_stop":
			handler(provider.StreamEvent{Type: "done"})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading SSE stream: %w", err)
	}

	if eventCount == 0 {
		return nil, fmt.Errorf("empty SSE stream: no events received from API")
	}

	result.Content = currentContent
	return &result, nil
}
