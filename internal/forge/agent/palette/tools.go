// Package palette provides the Palette tool provider for Trovery agents.
// It wraps the Go WebSocket client as MCP tools so the LLM can render UI
// components in the Palette Viewer via standard tool calls.
package palette

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	paletteclient "github.com/ceasarb/trovery-tools/internal/forge/palette/go-client"
	"github.com/ceasarb/trovery-tools/internal/forge/shared/protocol"
)

// ToolProvider wraps a Palette WebSocket client as MCP tools.
type ToolProvider struct {
	client       *paletteclient.Client
	promptTimeout time.Duration
}

// NewToolProvider creates a tool provider backed by the given Palette client.
func NewToolProvider(client *paletteclient.Client, promptTimeout time.Duration) *ToolProvider {
	return &ToolProvider{
		client:       client,
		promptTimeout: promptTimeout,
	}
}

// componentBehavior defines whether a component tool blocks for interaction.
type componentBehavior int

const (
	immediate componentBehavior = iota // returns immediately after render
	blocking                           // blocks until user interacts
	hybrid                             // immediate by default, blocks if await_interaction: true
)

// componentTool defines a per-component tool.
type componentTool struct {
	name        string
	component   string
	description string
	schema      json.RawMessage
	behavior    componentBehavior
}

var componentTools = []componentTool{
	{
		name:      "palette_tasklist",
		component: "TaskList",
		description: "Render an interactive task list. Users can add, check off, and delete tasks. Blocks until user interacts.",
		behavior:  blocking,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string", "description": "List title"},
				"tasks": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"id": {"type": "string"},
							"title": {"type": "string"},
							"completed": {"type": "boolean"},
							"priority": {"type": "string", "enum": ["high", "medium", "low"]}
						},
						"required": ["id", "title"]
					}
				},
				"id": {"type": "string", "description": "Optional component ID for later updates"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["tasks"]
		}`),
	},
	{
		name:      "palette_simpleform",
		component: "SimpleForm",
		description: "Render a form and wait for user submission. Returns submitted form data as key-value pairs.",
		behavior:  blocking,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string"},
				"fields": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"name": {"type": "string", "description": "Field key in submitted data"},
							"label": {"type": "string"},
							"type": {"type": "string", "enum": ["text", "number", "select", "checkbox"]},
							"required": {"type": "boolean"},
							"options": {"type": "array", "items": {"type": "string"}, "description": "For select fields"}
						},
						"required": ["name", "label", "type"]
					}
				},
				"submitLabel": {"type": "string", "description": "Submit button text"},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["fields"]
		}`),
	},
	{
		name:      "palette_cardgrid",
		component: "CardGrid",
		description: "Render a responsive grid of cards. Returns immediately after rendering.",
		behavior:  immediate,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string"},
				"cards": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"id": {"type": "string"},
							"title": {"type": "string"},
							"description": {"type": "string"},
							"status": {"type": "string"},
							"statusColor": {"type": "string"}
						},
						"required": ["id", "title"]
					}
				},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["cards"]
		}`),
	},
	{
		name:      "palette_table",
		component: "Table",
		description: "Render a sortable data table. Returns immediately unless await_interaction is true.",
		behavior:  hybrid,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string"},
				"columns": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"key": {"type": "string"},
							"label": {"type": "string"},
							"sortable": {"type": "boolean"}
						},
						"required": ["key", "label"]
					}
				},
				"rows": {"type": "array", "items": {"type": "object"}},
				"await_interaction": {"type": "boolean", "description": "If true, block until user clicks a row"},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["columns", "rows"]
		}`),
	},
	{
		name:      "palette_textblock",
		component: "TextBlock",
		description: "Render markdown-formatted text in the UI. Returns immediately.",
		behavior:  immediate,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {"type": "string", "description": "Markdown content"},
				"title": {"type": "string"},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["content"]
		}`),
	},
	{
		name:      "palette_codeblock",
		component: "CodeBlock",
		description: "Render syntax-highlighted code with a copy button. Returns immediately.",
		behavior:  immediate,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"code": {"type": "string", "description": "The source code to display"},
				"language": {"type": "string", "description": "Programming language for syntax highlighting"},
				"title": {"type": "string"},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["code"]
		}`),
	},
	{
		name:      "palette_progresstracker",
		component: "ProgressTracker",
		description: "Render a multi-step progress indicator. Returns immediately.",
		behavior:  immediate,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string"},
				"steps": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"label": {"type": "string"},
							"status": {"type": "string", "enum": ["pending", "active", "completed"]}
						},
						"required": ["label", "status"]
					}
				},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["steps"]
		}`),
	},
	{
		name:      "palette_calendar",
		component: "Calendar",
		description: "Render a month-view calendar with events. Returns immediately unless await_interaction is true.",
		behavior:  hybrid,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string"},
				"events": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"date": {"type": "string", "description": "YYYY-MM-DD"},
							"title": {"type": "string"},
							"color": {"type": "string"}
						},
						"required": ["date", "title"]
					}
				},
				"await_interaction": {"type": "boolean", "description": "If true, block until user clicks a date or event"},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["events"]
		}`),
	},
	{
		name:      "palette_filetree",
		component: "FileTree",
		description: "Render a hierarchical file browser. Returns immediately unless await_interaction is true.",
		behavior:  hybrid,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string"},
				"nodes": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"name": {"type": "string"},
							"type": {"type": "string", "enum": ["file", "directory"]},
							"children": {"type": "array", "description": "Nested nodes for directories"}
						},
						"required": ["name", "type"]
					}
				},
				"await_interaction": {"type": "boolean", "description": "If true, block until user selects a file"},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["nodes"]
		}`),
	},
	{
		name:      "palette_document",
		component: "Document",
		description: "Render a document with title, metadata, and markdown body. Returns immediately.",
		behavior:  immediate,
		schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string"},
				"metadata": {"type": "object", "description": "Key-value pairs displayed as document metadata"},
				"content": {"type": "string", "description": "Markdown body"},
				"id": {"type": "string"},
				"slot": {"type": "string", "enum": ["top", "main", "sidebar"], "description": "Layout region. Defaults to main."}
			},
			"required": ["title", "content"]
		}`),
	},
}

// Tools returns all 12 Palette tool definitions as MCP tools.
func (p *ToolProvider) Tools() []protocol.Tool {
	var tools []protocol.Tool

	// Per-component tools
	for _, ct := range componentTools {
		tools = append(tools, protocol.Tool{
			Name:        ct.name,
			Description: ct.description,
			InputSchema: ct.schema,
		})
	}

	// Generic tools
	tools = append(tools, protocol.Tool{
		Name:        "palette_update",
		Description: "Update an existing rendered component's properties. Props are merged with existing props.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "ID of the component to update"},
				"props": {"type": "object", "description": "New properties to merge"}
			},
			"required": ["id", "props"]
		}`),
	})

	tools = append(tools, protocol.Tool{
		Name:        "palette_clear",
		Description: "Remove rendered components from the UI. If id is provided, removes that component. If omitted, clears all.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "Optional ID of specific component to remove"}
			}
		}`),
	})

	return tools
}

// CallTool dispatches a tool call to the appropriate Palette client method.
func (p *ToolProvider) CallTool(ctx context.Context, name string, args interface{}) (*protocol.ToolCallResult, time.Duration, error) {
	start := time.Now()

	// Convert args to map
	argsMap, err := toMap(args)
	if err != nil {
		return nil, 0, fmt.Errorf("palette: invalid args: %w", err)
	}

	var resultJSON []byte

	switch name {
	case "palette_update":
		id, _ := argsMap["id"].(string)
		props, _ := argsMap["props"].(map[string]any)
		if id == "" || props == nil {
			return nil, 0, fmt.Errorf("palette_update requires 'id' and 'props'")
		}
		p.client.Update(id, props)
		resultJSON, _ = json.Marshal(map[string]any{"updated": id})

	case "palette_clear":
		id, _ := argsMap["id"].(string)
		p.client.Clear(id)
		resultJSON, _ = json.Marshal(map[string]any{"cleared": true})

	default:
		// Per-component tool
		ct := findComponentTool(name)
		if ct == nil {
			return nil, 0, fmt.Errorf("unknown palette tool: %s", name)
		}
		resultJSON, err = p.callComponentTool(ctx, ct, argsMap)
		if err != nil {
			return nil, 0, err
		}
	}

	duration := time.Since(start)
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			{Type: "text", Text: string(resultJSON)},
		},
	}, duration, nil
}

// Close releases the tool provider (does not close the client — lifecycle manages that).
func (p *ToolProvider) Close() {}

func (p *ToolProvider) callComponentTool(ctx context.Context, ct *componentTool, args map[string]any) ([]byte, error) {
	// Extract common fields
	id, _ := args["id"].(string)
	slot, _ := args["slot"].(string)
	awaitInteraction, _ := args["await_interaction"].(bool)

	// Build props — everything except id, slot, await_interaction
	props := make(map[string]any)
	for k, v := range args {
		if k != "id" && k != "slot" && k != "await_interaction" {
			props[k] = v
		}
	}

	// Build render options
	var opts []paletteclient.RenderOption
	if id != "" {
		opts = append(opts, paletteclient.WithID(id))
	}
	if slot != "" {
		opts = append(opts, paletteclient.WithSlot(slot))
	}

	// Determine if we should block
	shouldBlock := ct.behavior == blocking || (ct.behavior == hybrid && awaitInteraction)

	if shouldBlock {
		// Use Prompt — blocks until user interacts
		promptCtx := ctx
		if p.promptTimeout > 0 {
			var cancel context.CancelFunc
			promptCtx, cancel = context.WithTimeout(ctx, p.promptTimeout)
			defer cancel()
		}

		payload, err := p.client.Prompt(promptCtx, ct.component, props, opts...)
		if err != nil {
			if err == context.DeadlineExceeded {
				return json.Marshal(map[string]any{
					"timeout": true,
					"message": fmt.Sprintf("User did not respond within %s", p.promptTimeout),
				})
			}
			return nil, fmt.Errorf("palette prompt: %w", err)
		}

		// Return the interaction payload with the component ID
		result := map[string]any{
			"action":  payload["action"],
			"payload": payload,
		}
		if idVal, ok := payload["componentId"]; ok {
			result["id"] = idVal
		}
		return json.Marshal(result)
	}

	// Immediate — just render
	renderedID := p.client.Render(ct.component, props, opts...)
	return json.Marshal(map[string]any{
		"rendered": ct.component,
		"id":       renderedID,
	})
}

func findComponentTool(name string) *componentTool {
	for i := range componentTools {
		if componentTools[i].name == name {
			return &componentTools[i]
		}
	}
	return nil
}

func toMap(v interface{}) (map[string]any, error) {
	if v == nil {
		return map[string]any{}, nil
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
