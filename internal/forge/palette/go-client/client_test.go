package paletteclient

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
)

// mockServer simulates a Palette WS server (hub-and-spoke simplified to 1:1).
type mockServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	mu       sync.Mutex
	conn     *websocket.Conn
	received []map[string]any
}

func newMockServer() *mockServer {
	ms := &mockServer{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
	ms.server = httptest.NewServer(http.HandlerFunc(ms.handler))
	return ms
}

func (ms *mockServer) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := ms.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ms.mu.Lock()
	ms.conn = conn
	ms.mu.Unlock()

	// Read loop — store received messages
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

func (ms *mockServer) wsURL() string {
	return "ws" + strings.TrimPrefix(ms.server.URL, "http")
}

func (ms *mockServer) close() {
	ms.mu.Lock()
	if ms.conn != nil {
		ms.conn.Close()
	}
	ms.mu.Unlock()
	ms.server.Close()
}

func (ms *mockServer) getReceived() []map[string]any {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	out := make([]map[string]any, len(ms.received))
	copy(out, ms.received)
	return out
}

func (ms *mockServer) sendToClient(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.conn == nil {
		return nil
	}
	return ms.conn.WriteMessage(websocket.TextMessage, data)
}

// waitForMessages waits until the server has received at least n messages.
func (ms *mockServer) waitForMessages(n int, timeout time.Duration) bool {
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

// --- Tests ---

func TestConnect_Handshake(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	if client.State() != Connected {
		t.Errorf("state = %v, want Connected", client.State())
	}

	// Wait for the hello message
	if !ms.waitForMessages(1, 2*time.Second) {
		t.Fatal("timeout waiting for sdk-hello message")
	}

	msgs := ms.getReceived()
	if len(msgs) == 0 {
		t.Fatal("no messages received by server")
	}

	hello := msgs[0]
	if hello["type"] != "sdk-hello" {
		t.Errorf("first message type = %v, want %q", hello["type"], "sdk-hello")
	}
	if hello["version"] != "0.1.0" {
		t.Errorf("version = %v, want %q", hello["version"], "0.1.0")
	}
}

func TestRender_SendsCorrectJSON(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	id := client.Render("TaskList", map[string]any{
		"tasks": []any{},
	}, WithSlot("sidebar"))

	if id == "" {
		t.Error("expected non-empty ID")
	}

	if !strings.HasPrefix(id, "component-") {
		t.Errorf("id = %q, expected component-* prefix", id)
	}

	// Wait for hello + render messages
	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout waiting for render message")
	}

	msgs := ms.getReceived()
	render := msgs[1] // [0] is sdk-hello

	if render["type"] != "render" {
		t.Errorf("type = %v, want %q", render["type"], "render")
	}
	if render["component"] != "TaskList" {
		t.Errorf("component = %v, want %q", render["component"], "TaskList")
	}
	if render["slot"] != "sidebar" {
		t.Errorf("slot = %v, want %q", render["slot"], "sidebar")
	}
	if render["id"] != id {
		t.Errorf("id = %v, want %q", render["id"], id)
	}
}

func TestUpdate_SendsCorrectJSON(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	client.Update("task-1", map[string]any{"tasks": []any{"a", "b"}})

	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout waiting for update message")
	}

	msgs := ms.getReceived()
	update := msgs[1]

	if update["type"] != "update" {
		t.Errorf("type = %v, want %q", update["type"], "update")
	}
	if update["id"] != "task-1" {
		t.Errorf("id = %v, want %q", update["id"], "task-1")
	}
}

func TestClear_SpecificID(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	client.Clear("task-1")

	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout waiting for clear message")
	}

	msgs := ms.getReceived()
	clear := msgs[1]

	if clear["type"] != "clear" {
		t.Errorf("type = %v, want %q", clear["type"], "clear")
	}
	if clear["id"] != "task-1" {
		t.Errorf("id = %v, want %q", clear["id"], "task-1")
	}
}

func TestClear_All(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	client.Clear("")

	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout waiting for clear message")
	}

	msgs := ms.getReceived()
	clear := msgs[1]

	if clear["type"] != "clear" {
		t.Errorf("type = %v, want %q", clear["type"], "clear")
	}
	// ID should not be present for clear-all
	if _, exists := clear["id"]; exists {
		t.Error("expected 'id' to be absent for clear-all")
	}
}

func TestPrompt_BlocksAndReturns(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	// Start prompt in goroutine (it blocks)
	type result struct {
		payload map[string]any
		err     error
	}
	ch := make(chan result, 1)

	go func() {
		payload, err := client.Prompt(ctx, "SimpleForm", map[string]any{
			"fields": []any{},
		})
		ch <- result{payload, err}
	}()

	// Wait for the render message with _promptId
	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout waiting for prompt render message")
	}

	msgs := ms.getReceived()
	render := msgs[1]

	// Extract the _promptId from the render props
	props, ok := render["props"].(map[string]any)
	if !ok {
		t.Fatal("props is not a map")
	}
	promptID, ok := props["_promptId"].(string)
	if !ok || promptID == "" {
		t.Fatal("_promptId missing from render props")
	}
	componentID := render["id"].(string)

	// Simulate user submitting the form
	err := ms.sendToClient(map[string]any{
		"type":        "interaction",
		"componentId": componentID,
		"component":   "SimpleForm",
		"action":      "submit",
		"payload": map[string]any{
			"_promptId": promptID,
			"name":      "Alice",
			"email":     "alice@test.com",
		},
	})
	if err != nil {
		t.Fatalf("send interaction failed: %v", err)
	}

	// Wait for prompt to resolve
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("prompt error: %v", r.err)
		}
		if r.payload["name"] != "Alice" {
			t.Errorf("payload.name = %v, want %q", r.payload["name"], "Alice")
		}
		if r.payload["email"] != "alice@test.com" {
			t.Errorf("payload.email = %v, want %q", r.payload["email"], "alice@test.com")
		}
		// _promptId should be stripped
		if _, exists := r.payload["_promptId"]; exists {
			t.Error("_promptId should be stripped from returned payload")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for prompt result")
	}
}

func TestPrompt_Timeout(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer connectCancel()

	if err := client.Connect(connectCtx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	// Use a very short timeout
	promptCtx, promptCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer promptCancel()

	_, err := client.Prompt(promptCtx, "SimpleForm", map[string]any{"fields": []any{}})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestOnInteraction_FiresHandler(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	var received *InteractionMessage
	done := make(chan struct{})

	client.OnInteraction(func(msg *InteractionMessage) {
		received = msg
		close(done)
	})

	// Send an interaction from "viewer"
	err := ms.sendToClient(map[string]any{
		"type":        "interaction",
		"componentId": "task-1",
		"component":   "TaskList",
		"action":      "toggle",
		"payload":     map[string]any{"taskId": "1"},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	select {
	case <-done:
		if received.ComponentID != "task-1" {
			t.Errorf("componentId = %q, want %q", received.ComponentID, "task-1")
		}
		if received.Action != "toggle" {
			t.Errorf("action = %q, want %q", received.Action, "toggle")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for interaction handler")
	}
}

func TestOnMessage_FiresHandler(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	var received *UserMessage
	done := make(chan struct{})

	client.OnMessage(func(msg *UserMessage) {
		received = msg
		close(done)
	})

	err := ms.sendToClient(map[string]any{
		"type":      "user-message",
		"content":   "Hello agent",
		"timestamp": 1706547600000,
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	select {
	case <-done:
		if received.Content != "Hello agent" {
			t.Errorf("content = %q, want %q", received.Content, "Hello agent")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message handler")
	}
}

func TestMessageQueue_FlushesOnConnect(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	// Queue messages while disconnected
	client.Render("TextBlock", map[string]any{"content": "hello"})
	client.Render("CodeBlock", map[string]any{"code": "fmt.Println()"})

	// Now connect
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	// Should receive: hello + 2 queued renders
	if !ms.waitForMessages(3, 2*time.Second) {
		t.Fatalf("expected 3 messages (hello + 2 renders), got %d", len(ms.getReceived()))
	}

	msgs := ms.getReceived()
	if msgs[0]["type"] != "sdk-hello" {
		t.Errorf("msg[0] type = %v, want sdk-hello", msgs[0]["type"])
	}
	if msgs[1]["type"] != "render" {
		t.Errorf("msg[1] type = %v, want render", msgs[1]["type"])
	}
	if msgs[2]["type"] != "render" {
		t.Errorf("msg[2] type = %v, want render", msgs[2]["type"])
	}
}

func TestRender_WithCustomID(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	id := client.Render("ProgressTracker", map[string]any{"steps": []any{}}, WithID("my-progress"))

	if id != "my-progress" {
		t.Errorf("id = %q, want %q", id, "my-progress")
	}

	if !ms.waitForMessages(2, 2*time.Second) {
		t.Fatal("timeout")
	}

	msgs := ms.getReceived()
	if msgs[1]["id"] != "my-progress" {
		t.Errorf("server received id = %v, want %q", msgs[1]["id"], "my-progress")
	}
}

func TestClose_RejectsPendingPrompts(t *testing.T) {
	ms := newMockServer()
	defer ms.close()

	client := New(ms.wsURL(), WithAutoReconnect(false))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	type result struct {
		payload map[string]any
		err     error
	}
	ch := make(chan result, 1)

	go func() {
		payload, err := client.Prompt(context.Background(), "SimpleForm", map[string]any{"fields": []any{}})
		ch <- result{payload, err}
	}()

	// Give prompt time to register
	time.Sleep(50 * time.Millisecond)

	// Close the client — should reject the pending prompt
	client.Close()

	select {
	case r := <-ch:
		if r.err == nil {
			t.Fatal("expected error from rejected prompt, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for prompt rejection")
	}
}

func TestGenerateID_Format(t *testing.T) {
	client := New("ws://localhost:4321")

	id1 := client.generateID()
	id2 := client.generateID()

	if !strings.HasPrefix(id1, "component-1-") {
		t.Errorf("first id = %q, expected component-1-* prefix", id1)
	}
	if !strings.HasPrefix(id2, "component-2-") {
		t.Errorf("second id = %q, expected component-2-* prefix", id2)
	}
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
}

func TestGeneratePromptID_Format(t *testing.T) {
	client := New("ws://localhost:4321")

	id := client.generatePromptID()

	if !strings.HasPrefix(id, "prompt-") {
		t.Errorf("promptId = %q, expected prompt-* prefix", id)
	}

	// Should have format: prompt-{base36}-{random6}
	parts := strings.SplitN(id, "-", 3)
	if len(parts) != 3 {
		t.Errorf("promptId = %q, expected 3 parts separated by -", id)
	}
}
