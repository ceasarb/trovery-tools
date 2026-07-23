package ws

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // permissive for local dev
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 4096
)

// Upgrade upgrades an HTTP connection to WebSocket and registers the client with the hub.
// It returns the client for sending direct messages, or nil on upgrade failure.
func Upgrade(hub *Hub, w http.ResponseWriter, r *http.Request, topics []EventType, logger *slog.Logger) *Client {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("websocket upgrade", "error", err)
		return nil
	}

	client := &Client{
		ID:     uuid.New().String()[:8],
		Topics: topics,
		Send:   make(chan []byte, 256),
	}

	hub.Register(client)

	go writePump(conn, client, logger)
	go readPump(conn, client, hub, logger)

	return client
}

// readPump reads messages from the WebSocket connection.
// When the connection closes, it unregisters the client.
func readPump(conn *websocket.Conn, client *Client, hub *Hub, logger *slog.Logger) {
	defer func() {
		hub.Unregister(client)
		conn.Close()
	}()

	conn.SetReadLimit(maxMsgSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Debug("websocket read error", "id", client.ID, "error", err)
			}
			break
		}
	}
}

// writePump sends messages from the client's Send channel to the WebSocket connection.
func writePump(conn *websocket.Conn, client *Client, logger *slog.Logger) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logger.Debug("websocket write error", "id", client.ID, "error", err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
