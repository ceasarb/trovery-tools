package ollama

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
	"github.com/ceasarb/demigo-tools/internal/forge/agent/provider"
)

func TestNewDefaultHost(t *testing.T) {
	os.Unsetenv("OLLAMA_HOST")
	p, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.host != "http://localhost:11434" {
		t.Errorf("expected default host, got %q", p.host)
	}
}

func TestNewCustomHost(t *testing.T) {
	os.Setenv("OLLAMA_HOST", "http://myhost:1234")
	defer os.Unsetenv("OLLAMA_HOST")

	p, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.host != "http://myhost:1234" {
		t.Errorf("expected custom host, got %q", p.host)
	}
}

func TestNewStripsTrailingSlash(t *testing.T) {
	os.Setenv("OLLAMA_HOST", "http://myhost:1234/")
	defer os.Unsetenv("OLLAMA_HOST")

	p, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.host != "http://myhost:1234" {
		t.Errorf("expected no trailing slash, got %q", p.host)
	}
}

func TestCreateMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}

		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "llama3.1" {
			t.Errorf("expected model llama3.1, got %s", req.Model)
		}
		if req.Stream {
			t.Error("expected non-streaming request")
		}

		resp := ollamaResponse{
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "Hello from Llama!",
			},
			Done:            true,
			PromptEvalCount: 8,
			EvalCount:       4,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewWithClient(server.URL, server.Client())

	result, err := p.CreateMessage(
		[]provider.Message{
			{Role: "user", Content: []provider.Content{{Type: "text", Text: "Hi"}}},
		},
		nil,
		agentcfg.ModelConfig{Model: "llama3.1", MaxTokens: 1024},
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Content))
	}
	if result.Content[0].Text != "Hello from Llama!" {
		t.Errorf("expected 'Hello from Llama!', got %q", result.Content[0].Text)
	}
	if result.Usage.InputTokens != 8 {
		t.Errorf("expected 8 input tokens, got %d", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 4 {
		t.Errorf("expected 4 output tokens, got %d", result.Usage.OutputTokens)
	}
}

func TestCreateMessageWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Message: ollamaMessage{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{
						Function: ollamaFunctionCall{
							Name:      "get_weather",
							Arguments: map[string]interface{}{"city": "NYC"},
						},
					},
				},
			},
			Done:            true,
			PromptEvalCount: 12,
			EvalCount:       6,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewWithClient(server.URL, server.Client())

	result, err := p.CreateMessage(
		[]provider.Message{
			{Role: "user", Content: []provider.Content{{Type: "text", Text: "weather?"}}},
		},
		[]provider.ToolDef{
			{Name: "get_weather", Description: "Get weather", InputSchema: nil},
		},
		agentcfg.ModelConfig{Model: "llama3.1"},
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Content))
	}
	if result.Content[0].Type != "tool_use" {
		t.Errorf("expected tool_use, got %s", result.Content[0].Type)
	}
	if result.Content[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %s", result.Content[0].Name)
	}
	if result.StopReason != "tool_use" {
		t.Errorf("expected stop reason tool_use, got %s", result.StopReason)
	}
}

func TestCreateMessageStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("expected streaming request")
		}

		// Ollama streams as newline-delimited JSON
		chunks := []ollamaResponse{
			{Message: ollamaMessage{Role: "assistant", Content: "Hello"}, Done: false},
			{Message: ollamaMessage{Content: " world"}, Done: false},
			{Message: ollamaMessage{Content: ""}, Done: true, PromptEvalCount: 5, EvalCount: 3},
		}

		flusher := w.(http.Flusher)
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "%s\n", data)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewWithClient(server.URL, server.Client())

	var texts []string
	var gotDone bool

	result, err := p.CreateMessageStream(
		[]provider.Message{
			{Role: "user", Content: []provider.Content{{Type: "text", Text: "Hi"}}},
		},
		nil,
		agentcfg.ModelConfig{Model: "llama3.1"},
		"",
		func(event provider.StreamEvent) {
			switch event.Type {
			case "text":
				texts = append(texts, event.Text)
			case "done":
				gotDone = true
			}
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotDone {
		t.Error("expected done event")
	}
	if len(texts) != 2 {
		t.Fatalf("expected 2 text events, got %d", len(texts))
	}
	if texts[0] != "Hello" || texts[1] != " world" {
		t.Errorf("unexpected texts: %v", texts)
	}

	// Verify combined content
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Content))
	}
	if result.Content[0].Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", result.Content[0].Text)
	}
	if result.Usage.InputTokens != 5 {
		t.Errorf("expected 5 input tokens, got %d", result.Usage.InputTokens)
	}
}

func TestCreateMessageStreamWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunks := []ollamaResponse{
			{
				Message: ollamaMessage{
					Role: "assistant",
					ToolCalls: []ollamaToolCall{
						{Function: ollamaFunctionCall{Name: "search", Arguments: map[string]interface{}{"q": "test"}}},
					},
				},
				Done: false,
			},
			{Message: ollamaMessage{}, Done: true, PromptEvalCount: 10, EvalCount: 5},
		}

		flusher := w.(http.Flusher)
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "%s\n", data)
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewWithClient(server.URL, server.Client())

	var toolStarts []string

	result, err := p.CreateMessageStream(
		[]provider.Message{
			{Role: "user", Content: []provider.Content{{Type: "text", Text: "search for test"}}},
		},
		nil,
		agentcfg.ModelConfig{Model: "llama3.1"},
		"",
		func(event provider.StreamEvent) {
			if event.Type == "tool_use_start" {
				toolStarts = append(toolStarts, event.Name)
			}
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolStarts) != 1 {
		t.Fatalf("expected 1 tool start, got %d", len(toolStarts))
	}
	if toolStarts[0] != "search" {
		t.Errorf("expected search, got %s", toolStarts[0])
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Content))
	}
	if result.Content[0].Type != "tool_use" {
		t.Errorf("expected tool_use, got %s", result.Content[0].Type)
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		resp := map[string]interface{}{
			"models": []map[string]interface{}{
				{"name": "llama3.1:latest"},
				{"name": "mistral:7b"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewWithClient(server.URL, server.Client())

	models, err := p.ListModels()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0] != "llama3.1:latest" {
		t.Errorf("expected llama3.1:latest, got %s", models[0])
	}
}

func TestListModelsConnectionError(t *testing.T) {
	p := NewWithClient("http://localhost:1", &http.Client{})

	_, err := p.ListModels()
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer server.Close()

	p := NewWithClient(server.URL, server.Client())

	_, err := p.CreateMessage(
		[]provider.Message{
			{Role: "user", Content: []provider.Content{{Type: "text", Text: "Hi"}}},
		},
		nil,
		agentcfg.ModelConfig{Model: "nonexistent"},
		"",
	)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestConvertMessages(t *testing.T) {
	messages := []provider.Message{
		{
			Role: "user",
			Content: []provider.Content{
				{Type: "text", Text: "Hello"},
			},
		},
		{
			Role: "assistant",
			Content: []provider.Content{
				{Type: "text", Text: "Hi there!"},
				{Type: "tool_use", ID: "call_0", Name: "search", Input: map[string]interface{}{"q": "test"}},
			},
		},
		{
			Role: "user",
			Content: []provider.Content{
				{Type: "tool_result", ToolUseID: "call_0", Content: "result data"},
			},
		},
	}

	result := convertMessages(messages)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// User message
	if result[0].Role != "user" || result[0].Content != "Hello" {
		t.Errorf("unexpected user message: %+v", result[0])
	}

	// Assistant with tool call
	if result[1].Role != "assistant" {
		t.Errorf("expected assistant, got %s", result[1].Role)
	}
	if len(result[1].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[1].ToolCalls))
	}
	if result[1].ToolCalls[0].Function.Name != "search" {
		t.Errorf("expected search, got %s", result[1].ToolCalls[0].Function.Name)
	}

	// Tool result
	if result[2].Role != "tool" || result[2].Content != "result data" {
		t.Errorf("unexpected tool message: %+v", result[2])
	}
}

func TestConvertTools(t *testing.T) {
	tools := []provider.ToolDef{
		{
			Name:        "search",
			Description: "Search for things",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	result := convertTools(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Type != "function" {
		t.Errorf("expected type function, got %s", result[0].Type)
	}
	if result[0].Function.Name != "search" {
		t.Errorf("expected search, got %s", result[0].Function.Name)
	}
}

func TestBuildRequestOptions(t *testing.T) {
	p := NewWithClient("http://localhost:11434", http.DefaultClient)

	req := p.buildRequest(
		[]provider.Message{},
		nil,
		agentcfg.ModelConfig{
			Model:       "llama3.1",
			MaxTokens:   2048,
			Temperature: 0.7,
		},
		"",
		false,
	)

	if req.Options == nil {
		t.Fatal("expected options to be set")
	}
	if req.Options.Temperature != 0.7 {
		t.Errorf("expected temp 0.7, got %f", req.Options.Temperature)
	}
	if req.Options.NumPredict != 2048 {
		t.Errorf("expected num_predict 2048, got %d", req.Options.NumPredict)
	}
}

func TestToStringMap(t *testing.T) {
	// Test with nil
	m, err := toStringMap(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}

	// Test with map
	input := map[string]interface{}{"key": "value"}
	m, err = toStringMap(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m)
	}
}
