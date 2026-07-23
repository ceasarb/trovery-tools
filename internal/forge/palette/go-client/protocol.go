// Package paletteclient provides a Go WebSocket client for the Palette UI protocol.
//
// It allows AI agents written in Go to render interactive UI components
// in a Palette Viewer by sending render/update/clear/prompt messages
// over a WebSocket connection.
package paletteclient

import "encoding/json"

// --- SDK → Viewer Messages ---

// RenderMessage tells the viewer to render a new component or replace an existing one.
type RenderMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id"`
	Component string         `json:"component"`
	Props     map[string]any `json:"props"`
	Slot      string         `json:"slot,omitempty"`
}

// UpdateMessage merges new props into an existing component.
type UpdateMessage struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Props map[string]any `json:"props"`
}

// ClearMessage removes a component (by ID) or all components (if ID is empty).
type ClearMessage struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

// ActivityMessage streams real-time agent status events to the viewer.
type ActivityMessage struct {
	Type      string `json:"type"`      // always "activity"
	Event     string `json:"event"`     // user_message, thinking, tool_start, tool_result, text, error
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// AgentInfoMessage sends agent metadata to the viewer on connect.
type AgentInfoMessage struct {
	Type        string          `json:"type"`        // always "agent_info"
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Model       string          `json:"model,omitempty"`
	SubAgents   []SubAgentInfo  `json:"subAgents,omitempty"`
}

// SubAgentInfo describes a child agent exposed as a tool.
type SubAgentInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// --- Viewer → SDK Messages ---

// InteractionMessage is sent when a user interacts with a rendered component.
type InteractionMessage struct {
	Type        string         `json:"type"`
	ComponentID string         `json:"componentId"`
	Component   string         `json:"component"`
	Action      string         `json:"action"`
	Payload     map[string]any `json:"payload"`
}

// UserMessage is sent from the viewer's chat input.
type UserMessage struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// ConnectionMessage is sent when a viewer connects.
type ConnectionMessage struct {
	Type     string `json:"type"`
	ViewerID string `json:"viewerId"`
}

// SDKDisconnectedMessage is broadcast to viewers when an SDK client disconnects.
type SDKDisconnectedMessage struct {
	Type  string `json:"type"`
	SDKID string `json:"sdkId"`
}

// --- Handshake Messages ---

// SDKHelloMessage identifies this client as an SDK on connect.
type SDKHelloMessage struct {
	Type    string `json:"type"`
	Version string `json:"version,omitempty"`
}

// --- Message Parsing ---

// messageEnvelope is used for initial type detection when parsing incoming messages.
type messageEnvelope struct {
	Type string `json:"type"`
}

// ParseViewerMessage parses a raw JSON message from the server into a typed struct.
// Returns one of: *InteractionMessage, *UserMessage, *ConnectionMessage, *SDKDisconnectedMessage, or error.
func ParseViewerMessage(data []byte) (any, error) {
	var env messageEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}

	switch env.Type {
	case "interaction":
		var msg InteractionMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case "user-message":
		var msg UserMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case "connected":
		var msg ConnectionMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case "sdk-disconnected":
		var msg SDKDisconnectedMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	default:
		// Unknown message type — return the raw envelope
		return &env, nil
	}
}
