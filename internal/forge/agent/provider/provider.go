package provider

import (
	agentcfg "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
)

// Message represents a conversation message.
type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// Content is a content block within a message.
type Content struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"` // for tool_result
}

// ToolDef defines a tool for the model.
type ToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Response from the model.
type Response struct {
	Content    []Content
	Usage      Usage
	StopReason string
}

// StreamHandler is called for each streaming chunk.
type StreamHandler func(event StreamEvent)

// StreamEvent represents a streaming event.
type StreamEvent struct {
	Type string // "text", "tool_use_start", "tool_use_input", "done"
	Text string
	ID   string // tool use ID
	Name string // tool name
}

// Provider is the interface for model providers.
type Provider interface {
	// CreateMessage sends a message and returns the response.
	CreateMessage(messages []Message, tools []ToolDef, cfg agentcfg.ModelConfig, system string) (*Response, error)

	// CreateMessageStream sends a message with streaming.
	CreateMessageStream(messages []Message, tools []ToolDef, cfg agentcfg.ModelConfig, system string, handler StreamHandler) (*Response, error)
}
