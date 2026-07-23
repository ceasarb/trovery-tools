package palette

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	paletteclient "github.com/ceasarb/demigo-tools/internal/forge/palette/go-client"
)

// mockPaletteServer simulates a Palette WS server for testing.
type mockPaletteServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	mu       sync.Mutex
	conn     *websocket.Conn
	received []map[string]any
}

func newMockPaletteServer() *mockPaletteServer {
	ms := &mockPaletteServer{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
	ms.server = httptest.NewServer(http.HandlerFunc(ms.handler))
	return ms
}

func (ms *mockPaletteServer) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := ms.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ms.mu.Lock()
	ms.conn = conn
	ms.mu.Unlock()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		ms.mu.Lock()
		ms.received = append(ms.received, msg)
		ms.mu.Unlock()
	}
}

func (ms *mockPaletteServer) wsURL() string {
	return "ws" + strings.TrimPrefix(ms.server.URL, "http")
}

func (ms *mockPaletteServer) close() {
	ms.mu.Lock()
	if ms.conn != nil {
		ms.conn.Close()
	}
	ms.mu.Unlock()
	ms.server.Close()
}

func (ms *mockPaletteServer) waitForMessages(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ms.mu.Lock()
		count := len(ms.received)
		ms.mu.Unlock()
		if count >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (ms *mockPaletteServer) getReceived() []map[string]any {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	out := make([]map[string]any, len(ms.received))
	copy(out, ms.received)
	return out
}

func (ms *mockPaletteServer) sendToClient(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.conn.WriteMessage(websocket.TextMessage, data)
}

func setupProvider(t *testing.T) (*ToolProvider, *mockPaletteServer) {
	t.Helper()
	ms := newMockPaletteServer()
	client := paletteclient.New(ms.wsURL(), paletteclient.WithAutoReconnect(false))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	tp := NewToolProvider(client, 5*time.Minute)
	return tp, ms
}

func TestTools_Returns12(t *testing.T) {
	client := paletteclient.New("ws://localhost:4321")
	tp := NewToolProvider(client, 5*time.Minute)

	tools := tp.Tools()
	if len(tools) != 12 {
		t.Errorf("got %d tools, want 12", len(tools))
	}

	// Verify names
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{
		"palette_tasklist", "palette_simpleform", "palette_cardgrid",
		"palette_table", "palette_textblock", "palette_codeblock",
		"palette_progresstracker", "palette_calendar", "palette_filetree",
		"palette_document", "palette_update", "palette_clear",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestTools_SchemasAreValidJSON(t *testing.T) {
	client := paletteclient.New("ws://localhost:4321")
	tp := NewToolProvider(client, 5*time.Minute)

	for _, tool := range tp.Tools() {
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("tool %s has invalid schema JSON: %v", tool.Name, err)
		}
	}
}

func TestCallTool_ImmediateRender(t *testing.T) {
	tp, ms := setupProvider(t)
	defer ms.close()

	result, dur, err := tp.CallTool(context.Background(), "palette_textblock", map[string]any{
		"content": "Hello world",
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}
	if dur == 0 {
		t.Error("expected non-zero duration")
	}

	// Parse result
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &parsed); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if parsed["rendered"] != "TextBlock" {
		t.Errorf("rendered = %v, want TextBlock", parsed["rendered"])
	}
	if parsed["id"] == nil || parsed["id"] == "" {
		t.Error("expected non-empty ID")
	}

	// Verify message was sent to server (hello + render)
	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout waiting for render message")
	}
	msgs := ms.getReceived()
	render := msgs[1]
	if render["type"] != "render" {
		t.Errorf("type = %v, want render", render["type"])
	}
	if render["component"] != "TextBlock" {
		t.Errorf("component = %v, want TextBlock", render["component"])
	}
}

func TestCallTool_WithSlot(t *testing.T) {
	tp, ms := setupProvider(t)
	defer ms.close()

	_, _, err := tp.CallTool(context.Background(), "palette_codeblock", map[string]any{
		"code":     "fmt.Println()",
		"language": "go",
		"slot":     "sidebar",
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout")
	}
	msgs := ms.getReceived()
	render := msgs[1]
	if render["slot"] != "sidebar" {
		t.Errorf("slot = %v, want sidebar", render["slot"])
	}
}

func TestCallTool_Update(t *testing.T) {
	tp, ms := setupProvider(t)
	defer ms.close()

	result, _, err := tp.CallTool(context.Background(), "palette_update", map[string]any{
		"id":    "comp-1",
		"props": map[string]any{"tasks": []any{}},
	})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result.Content[0].Text), &parsed)
	if parsed["updated"] != "comp-1" {
		t.Errorf("updated = %v, want comp-1", parsed["updated"])
	}

	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout")
	}
	msgs := ms.getReceived()
	update := msgs[1]
	if update["type"] != "update" {
		t.Errorf("type = %v, want update", update["type"])
	}
}

func TestCallTool_Clear(t *testing.T) {
	tp, ms := setupProvider(t)
	defer ms.close()

	result, _, err := tp.CallTool(context.Background(), "palette_clear", map[string]any{})
	if err != nil {
		t.Fatalf("CallTool error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result.Content[0].Text), &parsed)
	if parsed["cleared"] != true {
		t.Errorf("cleared = %v, want true", parsed["cleared"])
	}
}

func TestCallTool_UnknownTool(t *testing.T) {
	tp, ms := setupProvider(t)
	defer ms.close()

	_, _, err := tp.CallTool(context.Background(), "palette_unknown", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestCallTool_BlockingPromptTimeout(t *testing.T) {
	ms := newMockPaletteServer()
	defer ms.close()

	client := paletteclient.New(ms.wsURL(), paletteclient.WithAutoReconnect(false))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Very short timeout
	tp := NewToolProvider(client, 100*time.Millisecond)

	result, _, err := tp.CallTool(context.Background(), "palette_simpleform", map[string]any{
		"fields": []any{},
	})
	if err != nil {
		t.Fatalf("expected timeout result, not error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal([]byte(result.Content[0].Text), &parsed)
	if parsed["timeout"] != true {
		t.Errorf("timeout = %v, want true", parsed["timeout"])
	}
}
