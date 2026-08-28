package paletteclient

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectionState represents the current state of the WebSocket connection.
type ConnectionState int

const (
	Disconnected ConnectionState = iota
	Connecting
	Connected
)

func (s ConnectionState) String() string {
	switch s {
	case Disconnected:
		return "disconnected"
	case Connecting:
		return "connecting"
	case Connected:
		return "connected"
	default:
		return "unknown"
	}
}

// RenderOption configures a Render call.
type RenderOption func(*renderOpts)

type renderOpts struct {
	id   string
	slot string
}

// WithID sets a custom component ID instead of auto-generating one.
func WithID(id string) RenderOption {
	return func(o *renderOpts) { o.id = id }
}

// WithSlot sets the layout slot ("top", "main", "sidebar"). Defaults to "main".
func WithSlot(slot string) RenderOption {
	return func(o *renderOpts) { o.slot = slot }
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithAutoReconnect enables or disables auto-reconnect (default: true).
func WithAutoReconnect(enabled bool) ClientOption {
	return func(c *Client) { c.autoReconnect = enabled }
}

// WithMaxReconnectDelay sets the maximum delay between reconnect attempts.
func WithMaxReconnectDelay(d time.Duration) ClientOption {
	return func(c *Client) { c.maxReconnectDelay = d }
}

// pendingPrompt tracks a blocking Prompt() call waiting for user interaction.
type pendingPrompt struct {
	componentID string
	ch          chan promptResult
}

type promptResult struct {
	payload map[string]any
	err     error
}

// Client is a Go WebSocket client that speaks the Palette protocol.
type Client struct {
	url               string
	autoReconnect     bool
	maxReconnectDelay time.Duration

	mu                sync.Mutex
	writeMu           sync.Mutex // guards WebSocket writes (separate from state mu)
	conn              *websocket.Conn
	state             ConnectionState
	reconnectAttempts int
	componentCounter  atomic.Int64
	messageQueue      []json.RawMessage
	stopReconnect     chan struct{}

	// Event handlers
	interactionHandlers []func(*InteractionMessage)
	messageHandlers     []func(*UserMessage)
	connectedHandlers   []func()
	disconnectedHandlers []func()
	errorHandlers       []func(error)

	// Pending prompts keyed by promptId
	pendingPrompts map[string]*pendingPrompt

	// Lifecycle
	done     chan struct{}
	closeOnce sync.Once
}

// New creates a new Palette client for the given WebSocket URL.
func New(url string, opts ...ClientOption) *Client {
	c := &Client{
		url:               url,
		autoReconnect:     true,
		maxReconnectDelay: 30 * time.Second,
		state:             Disconnected,
		pendingPrompts:    make(map[string]*pendingPrompt),
		done:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// State returns the current connection state.
func (c *Client) State() ConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Connect establishes the WebSocket connection and sends the SDK hello.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.state == Connected {
		c.mu.Unlock()
		return nil
	}
	c.state = Connecting
	c.mu.Unlock()

	return c.dial(ctx)
}

func (c *Client) dial(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.url, nil)
	if err != nil {
		c.mu.Lock()
		c.state = Disconnected
		c.mu.Unlock()
		return fmt.Errorf("palette: connect failed: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.state = Connected
	c.reconnectAttempts = 0
	c.mu.Unlock()

	// Send SDK hello
	hello := SDKHelloMessage{Type: "sdk-hello", Version: "0.1.0"}
	if err := c.sendJSON(hello); err != nil {
		conn.Close()
		c.mu.Lock()
		c.state = Disconnected
		c.conn = nil
		c.mu.Unlock()
		return fmt.Errorf("palette: handshake failed: %w", err)
	}

	// Flush queued messages
	c.flushQueue()

	// Emit connected event
	c.mu.Lock()
	handlers := make([]func(), len(c.connectedHandlers))
	copy(handlers, c.connectedHandlers)
	c.mu.Unlock()
	for _, h := range handlers {
		h()
	}

	// Start read loop, bound to this connection — it must never read the
	// shared c.conn, which Close and reconnect rewrite concurrently.
	go c.readLoop(conn)

	return nil
}

// Close disconnects and releases all resources.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)

		c.mu.Lock()
		c.autoReconnect = false
		if c.stopReconnect != nil {
			close(c.stopReconnect)
			c.stopReconnect = nil
		}
		conn := c.conn
		c.conn = nil
		c.state = Disconnected

		// Reject all pending prompts
		for id, p := range c.pendingPrompts {
			p.ch <- promptResult{err: fmt.Errorf("palette: client closed")}
			delete(c.pendingPrompts, id)
		}
		c.mu.Unlock()

		if conn != nil {
			// writeMu serializes this close frame with in-flight sends —
			// gorilla/websocket allows at most one concurrent writer.
			c.writeMu.Lock()
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			c.writeMu.Unlock()
			conn.Close()
		}
	})
}

// --- Render Methods ---

// Render sends a component to the viewer. Returns the component ID.
func (c *Client) Render(component string, props map[string]any, opts ...RenderOption) string {
	o := renderOpts{}
	for _, fn := range opts {
		fn(&o)
	}
	id := o.id
	if id == "" {
		id = c.generateID()
	}

	msg := RenderMessage{
		Type:      "render",
		ID:        id,
		Component: component,
		Props:     props,
		Slot:      o.slot,
	}
	c.send(msg)
	return id
}

// Update merges new props into an existing component.
func (c *Client) Update(id string, props map[string]any) {
	msg := UpdateMessage{
		Type:  "update",
		ID:    id,
		Props: props,
	}
	c.send(msg)
}

// Clear removes a component by ID. If id is empty, clears all components.
func (c *Client) Clear(id string) {
	msg := ClearMessage{
		Type: "clear",
		ID:   id,
	}
	c.send(msg)
}

// SendActivity sends a real-time activity event to the viewer.
func (c *Client) SendActivity(event, content string) {
	msg := ActivityMessage{
		Type:      "activity",
		Event:     event,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}
	c.send(msg)
}

// SendAgentInfo sends agent metadata to the viewer so it can display
// agent identity and capabilities in the idle state.
func (c *Client) SendAgentInfo(name, description, model string, subAgents []SubAgentInfo) {
	msg := AgentInfoMessage{
		Type:        "agent_info",
		Name:        name,
		Description: description,
		Model:       model,
		SubAgents:   subAgents,
	}
	c.send(msg)
}

// Prompt renders a component and blocks until the user interacts with it.
// The returned map contains the interaction payload.
// The context controls the timeout — use context.WithTimeout for prompt_timeout.
func (c *Client) Prompt(ctx context.Context, component string, props map[string]any, opts ...RenderOption) (map[string]any, error) {
	o := renderOpts{}
	for _, fn := range opts {
		fn(&o)
	}
	id := o.id
	if id == "" {
		id = c.generateID()
	}
	promptID := c.generatePromptID()

	// Register pending prompt before sending
	ch := make(chan promptResult, 1)
	c.mu.Lock()
	c.pendingPrompts[promptID] = &pendingPrompt{
		componentID: id,
		ch:          ch,
	}
	c.mu.Unlock()

	// Inject _promptId into props so the viewer echoes it back
	propsWithPrompt := make(map[string]any, len(props)+1)
	for k, v := range props {
		propsWithPrompt[k] = v
	}
	propsWithPrompt["_promptId"] = promptID

	msg := RenderMessage{
		Type:      "render",
		ID:        id,
		Component: component,
		Props:     propsWithPrompt,
		Slot:      o.slot,
	}
	c.send(msg)

	// Wait for interaction or context cancellation
	select {
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		// Clear the prompt component after receiving response
		c.Clear(id)
		return result.payload, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pendingPrompts, promptID)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("palette: client closed")
	}
}

// --- Event Methods ---

// OnInteraction registers a handler for component interactions. Returns an unsubscribe function.
func (c *Client) OnInteraction(handler func(*InteractionMessage)) func() {
	c.mu.Lock()
	c.interactionHandlers = append(c.interactionHandlers, handler)
	idx := len(c.interactionHandlers) - 1
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if idx < len(c.interactionHandlers) {
			c.interactionHandlers = append(c.interactionHandlers[:idx], c.interactionHandlers[idx+1:]...)
		}
	}
}

// OnMessage registers a handler for user chat messages. Returns an unsubscribe function.
func (c *Client) OnMessage(handler func(*UserMessage)) func() {
	c.mu.Lock()
	c.messageHandlers = append(c.messageHandlers, handler)
	idx := len(c.messageHandlers) - 1
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if idx < len(c.messageHandlers) {
			c.messageHandlers = append(c.messageHandlers[:idx], c.messageHandlers[idx+1:]...)
		}
	}
}

// OnConnected registers a handler called when the connection is established.
func (c *Client) OnConnected(handler func()) func() {
	c.mu.Lock()
	c.connectedHandlers = append(c.connectedHandlers, handler)
	idx := len(c.connectedHandlers) - 1
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if idx < len(c.connectedHandlers) {
			c.connectedHandlers = append(c.connectedHandlers[:idx], c.connectedHandlers[idx+1:]...)
		}
	}
}

// OnDisconnected registers a handler called when the connection is lost.
func (c *Client) OnDisconnected(handler func()) func() {
	c.mu.Lock()
	c.disconnectedHandlers = append(c.disconnectedHandlers, handler)
	idx := len(c.disconnectedHandlers) - 1
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if idx < len(c.disconnectedHandlers) {
			c.disconnectedHandlers = append(c.disconnectedHandlers[:idx], c.disconnectedHandlers[idx+1:]...)
		}
	}
}

// OnError registers a handler for connection errors.
func (c *Client) OnError(handler func(error)) func() {
	c.mu.Lock()
	c.errorHandlers = append(c.errorHandlers, handler)
	idx := len(c.errorHandlers) - 1
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if idx < len(c.errorHandlers) {
			c.errorHandlers = append(c.errorHandlers[:idx], c.errorHandlers[idx+1:]...)
		}
	}
}

// --- Internal ---

func (c *Client) readLoop(conn *websocket.Conn) {
	defer func() {
		// Tear down shared state only if this loop's connection is still the
		// current one — after a reconnect, an obsolete loop exiting must not
		// nil out the new connection or trigger another reconnect.
		c.mu.Lock()
		current := c.conn == conn
		wasConnected := current && c.state == Connected
		if current {
			c.conn = nil
			c.state = Disconnected
		}
		c.mu.Unlock()

		conn.Close()

		if wasConnected {
			c.mu.Lock()
			handlers := make([]func(), len(c.disconnectedHandlers))
			copy(handlers, c.disconnectedHandlers)
			c.mu.Unlock()
			for _, h := range handlers {
				h()
			}
		}

		if c.autoReconnect && wasConnected {
			c.scheduleReconnect()
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			// Check if this is a normal close
			select {
			case <-c.done:
				return
			default:
			}
			c.emitError(fmt.Errorf("palette: read error: %w", err))
			return
		}

		msg, err := ParseViewerMessage(data)
		if err != nil {
			c.emitError(fmt.Errorf("palette: parse error: %w", err))
			continue
		}

		switch m := msg.(type) {
		case *InteractionMessage:
			c.handleInteraction(m)
		case *UserMessage:
			c.handleUserMessage(m)
		case *ConnectionMessage:
			// Viewer connected notification — no action needed
		case *SDKDisconnectedMessage:
			// Another SDK disconnected — no action needed
		}
	}
}

func (c *Client) handleInteraction(msg *InteractionMessage) {
	// Check if this is a prompt response
	if promptID, ok := msg.Payload["_promptId"].(string); ok {
		c.mu.Lock()
		pending, exists := c.pendingPrompts[promptID]
		if exists {
			delete(c.pendingPrompts, promptID)
		}
		c.mu.Unlock()

		if exists {
			// Strip _promptId from payload
			payload := make(map[string]any, len(msg.Payload))
			for k, v := range msg.Payload {
				if k != "_promptId" {
					payload[k] = v
				}
			}
			pending.ch <- promptResult{payload: payload}
			return
		}
	}

	// Regular interaction — dispatch to handlers
	c.mu.Lock()
	handlers := make([]func(*InteractionMessage), len(c.interactionHandlers))
	copy(handlers, c.interactionHandlers)
	c.mu.Unlock()
	for _, h := range handlers {
		h(msg)
	}
}

func (c *Client) handleUserMessage(msg *UserMessage) {
	c.mu.Lock()
	handlers := make([]func(*UserMessage), len(c.messageHandlers))
	copy(handlers, c.messageHandlers)
	c.mu.Unlock()
	for _, h := range handlers {
		h(msg)
	}
}

func (c *Client) emitError(err error) {
	c.mu.Lock()
	handlers := make([]func(error), len(c.errorHandlers))
	copy(handlers, c.errorHandlers)
	c.mu.Unlock()
	for _, h := range handlers {
		h(err)
	}
}

func (c *Client) send(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.emitError(fmt.Errorf("palette: marshal error: %w", err))
		return
	}

	c.mu.Lock()
	if c.state == Connected && c.conn != nil {
		conn := c.conn
		c.mu.Unlock()
		c.writeMu.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		c.writeMu.Unlock()
		if err != nil {
			c.emitError(fmt.Errorf("palette: write error: %w", err))
		}
		return
	}
	// Queue for later
	c.messageQueue = append(c.messageQueue, data)
	c.mu.Unlock()
}

func (c *Client) sendJSON(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	c.writeMu.Lock()
	err = conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()
	return err
}

func (c *Client) flushQueue() {
	c.mu.Lock()
	queue := c.messageQueue
	c.messageQueue = nil
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	for _, data := range queue {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			c.emitError(fmt.Errorf("palette: flush error: %w", err))
			return
		}
	}
}

func (c *Client) scheduleReconnect() {
	c.mu.Lock()
	// Calculate delay with exponential backoff
	delay := time.Duration(math.Min(
		float64(time.Second)*math.Pow(2, float64(c.reconnectAttempts)),
		float64(c.maxReconnectDelay),
	))
	c.reconnectAttempts++
	c.stopReconnect = make(chan struct{})
	stop := c.stopReconnect
	c.mu.Unlock()

	go func() {
		select {
		case <-time.After(delay):
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := c.dial(ctx); err != nil {
				c.emitError(err)
				// dial's readLoop won't be started on error, so schedule again
				c.mu.Lock()
				shouldRetry := c.autoReconnect
				c.mu.Unlock()
				if shouldRetry {
					c.scheduleReconnect()
				}
			}
		case <-stop:
			return
		case <-c.done:
			return
		}
	}()
}

// --- ID Generation (matches TypeScript SDK format) ---

func (c *Client) generateID() string {
	n := c.componentCounter.Add(1)
	return "component-" + strconv.FormatInt(n, 10) + "-" + strconv.FormatInt(time.Now().UnixMilli(), 36)
}

func (c *Client) generatePromptID() string {
	return "prompt-" + strconv.FormatInt(time.Now().UnixMilli(), 36) + "-" + randomBase36(6)
}

func randomBase36(n int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
