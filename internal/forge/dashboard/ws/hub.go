package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
)

// EventType identifies the kind of event being broadcast.
type EventType string

const (
	EventAgentText      EventType = "agent:text"
	EventAgentToolStart EventType = "agent:tool_start"
	EventAgentToolEnd   EventType = "agent:tool_end"
	EventAgentDone      EventType = "agent:done"
	EventEvalScenario   EventType = "eval:scenario"
	EventEvalAssertion  EventType = "eval:assertion"
	EventEvalDone       EventType = "eval:done"
	EventSessionStart   EventType = "session:start"
	EventSessionEnd     EventType = "session:end"
)

// Event is a message broadcast through the hub.
type Event struct {
	Type    EventType      `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Client represents a single WebSocket connection subscribed to events.
type Client struct {
	ID     string
	Topics []EventType // empty means subscribe to all
	Send   chan []byte
	hub    *Hub
}

// Hub manages WebSocket client connections and event broadcasting.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan Event
	register   chan *Client
	unregister chan *Client
	logger     *slog.Logger
}

// NewHub creates a new event hub. Call Run() to start processing.
func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

// Run starts the hub's event loop. Should be called in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Debug("client connected", "id", client.ID, "total", h.ClientCount())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			h.logger.Debug("client disconnected", "id", client.ID, "total", h.ClientCount())

		case event := <-h.broadcast:
			data, err := json.Marshal(event)
			if err != nil {
				h.logger.Error("marshal event", "error", err)
				continue
			}

			h.mu.RLock()
			for client := range h.clients {
				if !client.isSubscribed(event.Type) {
					continue
				}
				select {
				case client.Send <- data:
				default:
					// Client buffer full — drop and disconnect
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, client)
					close(client.Send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	client.hub = h
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Publish sends an event to all subscribed clients.
func (h *Hub) Publish(event Event) {
	h.broadcast <- event
}

// ClientCount returns the current number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// isSubscribed checks if the client should receive this event type.
func (c *Client) isSubscribed(eventType EventType) bool {
	if len(c.Topics) == 0 {
		return true // no filter means all events
	}
	for _, t := range c.Topics {
		if t == eventType {
			return true
		}
	}
	return false
}
