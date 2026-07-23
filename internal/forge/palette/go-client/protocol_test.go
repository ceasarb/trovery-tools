package paletteclient

import (
	"encoding/json"
	"testing"
)

func TestParseViewerMessage_Interaction(t *testing.T) {
	data := `{
		"type": "interaction",
		"componentId": "task-list-1",
		"component": "TaskList",
		"action": "toggle",
		"payload": {"taskId": "1", "completed": true}
	}`

	msg, err := ParseViewerMessage([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interaction, ok := msg.(*InteractionMessage)
	if !ok {
		t.Fatalf("expected *InteractionMessage, got %T", msg)
	}

	if interaction.ComponentID != "task-list-1" {
		t.Errorf("componentId = %q, want %q", interaction.ComponentID, "task-list-1")
	}
	if interaction.Component != "TaskList" {
		t.Errorf("component = %q, want %q", interaction.Component, "TaskList")
	}
	if interaction.Action != "toggle" {
		t.Errorf("action = %q, want %q", interaction.Action, "toggle")
	}
	if interaction.Payload["taskId"] != "1" {
		t.Errorf("payload.taskId = %v, want %q", interaction.Payload["taskId"], "1")
	}
}

func TestParseViewerMessage_UserMessage(t *testing.T) {
	data := `{
		"type": "user-message",
		"content": "Hello agent",
		"timestamp": 1706547600000
	}`

	msg, err := ParseViewerMessage([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}

	if um.Content != "Hello agent" {
		t.Errorf("content = %q, want %q", um.Content, "Hello agent")
	}
	if um.Timestamp != 1706547600000 {
		t.Errorf("timestamp = %d, want %d", um.Timestamp, 1706547600000)
	}
}

func TestParseViewerMessage_Connected(t *testing.T) {
	data := `{"type": "connected", "viewerId": "viewer-abc123"}`

	msg, err := ParseViewerMessage([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cm, ok := msg.(*ConnectionMessage)
	if !ok {
		t.Fatalf("expected *ConnectionMessage, got %T", msg)
	}

	if cm.ViewerID != "viewer-abc123" {
		t.Errorf("viewerId = %q, want %q", cm.ViewerID, "viewer-abc123")
	}
}

func TestParseViewerMessage_SDKDisconnected(t *testing.T) {
	data := `{"type": "sdk-disconnected", "sdkId": "sdk-xyz789"}`

	msg, err := ParseViewerMessage([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dm, ok := msg.(*SDKDisconnectedMessage)
	if !ok {
		t.Fatalf("expected *SDKDisconnectedMessage, got %T", msg)
	}

	if dm.SDKID != "sdk-xyz789" {
		t.Errorf("sdkId = %q, want %q", dm.SDKID, "sdk-xyz789")
	}
}

func TestRenderMessage_JSON(t *testing.T) {
	msg := RenderMessage{
		Type:      "render",
		ID:        "task-1",
		Component: "TaskList",
		Props:     map[string]any{"tasks": []any{}},
		Slot:      "main",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["type"] != "render" {
		t.Errorf("type = %v, want %q", parsed["type"], "render")
	}
	if parsed["id"] != "task-1" {
		t.Errorf("id = %v, want %q", parsed["id"], "task-1")
	}
	if parsed["component"] != "TaskList" {
		t.Errorf("component = %v, want %q", parsed["component"], "TaskList")
	}
	if parsed["slot"] != "main" {
		t.Errorf("slot = %v, want %q", parsed["slot"], "main")
	}
}

func TestClearMessage_OmitsEmptyID(t *testing.T) {
	msg := ClearMessage{Type: "clear"}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, exists := parsed["id"]; exists {
		t.Error("expected 'id' to be omitted for clear-all, but it was present")
	}
}

func TestRenderMessage_OmitsEmptySlot(t *testing.T) {
	msg := RenderMessage{
		Type:      "render",
		ID:        "x",
		Component: "TextBlock",
		Props:     map[string]any{"content": "hi"},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, exists := parsed["slot"]; exists {
		t.Error("expected 'slot' to be omitted when empty, but it was present")
	}
}
