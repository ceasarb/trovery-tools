package ws

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestHubRegisterUnregister(t *testing.T) {
	hub := NewHub(slog.Default())
	go hub.Run()

	client := &Client{
		ID:   "test-1",
		Send: make(chan []byte, 256),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond) // let goroutine process

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub(slog.Default())
	go hub.Run()

	client := &Client{
		ID:   "test-1",
		Send: make(chan []byte, 256),
	}
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.Publish(Event{
		Type:    EventAgentText,
		Payload: map[string]any{"text": "hello"},
	})

	select {
	case msg := <-client.Send:
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if event.Type != EventAgentText {
			t.Errorf("type = %q, want %q", event.Type, EventAgentText)
		}
		if event.Payload["text"] != "hello" {
			t.Errorf("payload text = %v", event.Payload["text"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}

	hub.Unregister(client)
}

func TestHubTopicFiltering(t *testing.T) {
	hub := NewHub(slog.Default())
	go hub.Run()

	// Client subscribed only to eval events
	evalClient := &Client{
		ID:     "eval-only",
		Topics: []EventType{EventEvalScenario, EventEvalDone},
		Send:   make(chan []byte, 256),
	}

	// Client subscribed to all events
	allClient := &Client{
		ID:   "all",
		Send: make(chan []byte, 256),
	}

	hub.Register(evalClient)
	hub.Register(allClient)
	time.Sleep(10 * time.Millisecond)

	// Publish agent event — should only reach allClient
	hub.Publish(Event{Type: EventAgentText, Payload: map[string]any{"text": "hi"}})
	time.Sleep(10 * time.Millisecond)

	select {
	case <-allClient.Send:
		// expected
	case <-time.After(time.Second):
		t.Fatal("allClient should receive agent event")
	}

	select {
	case msg := <-evalClient.Send:
		t.Errorf("evalClient should NOT receive agent event, got: %s", string(msg))
	case <-time.After(50 * time.Millisecond):
		// expected — no message
	}

	// Publish eval event — should reach both
	hub.Publish(Event{Type: EventEvalScenario, Payload: map[string]any{"name": "test"}})
	time.Sleep(10 * time.Millisecond)

	select {
	case <-evalClient.Send:
		// expected
	case <-time.After(time.Second):
		t.Fatal("evalClient should receive eval event")
	}

	select {
	case <-allClient.Send:
		// expected
	case <-time.After(time.Second):
		t.Fatal("allClient should receive eval event")
	}

	hub.Unregister(evalClient)
	hub.Unregister(allClient)
}

func TestHubMultipleClients(t *testing.T) {
	hub := NewHub(slog.Default())
	go hub.Run()

	clients := make([]*Client, 5)
	for i := range clients {
		clients[i] = &Client{
			ID:   string(rune('A' + i)),
			Send: make(chan []byte, 256),
		}
		hub.Register(clients[i])
	}
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 5 {
		t.Errorf("expected 5 clients, got %d", hub.ClientCount())
	}

	hub.Publish(Event{Type: EventAgentDone, Payload: map[string]any{"done": true}})
	time.Sleep(10 * time.Millisecond)

	for i, c := range clients {
		select {
		case <-c.Send:
			// expected
		case <-time.After(time.Second):
			t.Errorf("client %d did not receive broadcast", i)
		}
	}

	for _, c := range clients {
		hub.Unregister(c)
	}
}

func TestClientIsSubscribed(t *testing.T) {
	// No topics = subscribe to all
	c1 := &Client{Topics: nil}
	if !c1.isSubscribed(EventAgentText) {
		t.Error("nil topics should subscribe to all")
	}

	// Specific topics
	c2 := &Client{Topics: []EventType{EventEvalDone}}
	if c2.isSubscribed(EventAgentText) {
		t.Error("should not be subscribed to agent:text")
	}
	if !c2.isSubscribed(EventEvalDone) {
		t.Error("should be subscribed to eval:done")
	}
}

func TestEventTypes(t *testing.T) {
	// Verify all event type constants are unique
	types := []EventType{
		EventAgentText, EventAgentToolStart, EventAgentToolEnd, EventAgentDone,
		EventEvalScenario, EventEvalAssertion, EventEvalDone,
		EventSessionStart, EventSessionEnd,
	}

	seen := make(map[EventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}
}
