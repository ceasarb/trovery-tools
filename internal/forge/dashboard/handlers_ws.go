package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	agentcfg "github.com/ceasarb/trovery-tools/internal/forge/agent/config"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/runtime"
	"github.com/ceasarb/trovery-tools/internal/forge/agent/servermgr"
	"github.com/ceasarb/trovery-tools/internal/forge/dashboard/ws"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// handleWSEvents is a general event stream — clients receive all hub events.
func (s *Server) handleWSEvents(w http.ResponseWriter, r *http.Request) {
	if !s.checkWSAuth(w, r) {
		return
	}
	ws.Upgrade(s.hub, w, r, nil, s.logger)
}

// handleWSEval subscribes to eval-related events only.
func (s *Server) handleWSEval(w http.ResponseWriter, r *http.Request) {
	if !s.checkWSAuth(w, r) {
		return
	}
	topics := []ws.EventType{
		ws.EventEvalScenario,
		ws.EventEvalAssertion,
		ws.EventEvalDone,
	}
	ws.Upgrade(s.hub, w, r, topics, s.logger)
}

// chatRequest is a message sent by the browser to the chat playground.
type chatRequest struct {
	Agent   string `json:"agent"`
	Message string `json:"message"`
}

// checkWSAuth validates the WebSocket token query parameter if auth is configured.
func (s *Server) checkWSAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.authCfg == nil || s.authCfg.Disabled {
		return true
	}
	qToken := r.URL.Query().Get("token")
	if qToken == "" || qToken != s.authCfg.Token {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return false
	}
	return true
}

// handleWSChat creates an ephemeral agent runtime per WebSocket session.
// The browser sends JSON messages, the server streams agent events back.
func (s *Server) handleWSChat(w http.ResponseWriter, r *http.Request) {
	if !s.checkWSAuth(w, r) {
		return
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("chat ws upgrade", "error", err)
		return
	}
	defer conn.Close()

	s.logger.Debug("chat playground connected")

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.Debug("chat ws read error", "error", err)
			}
			break
		}

		var req chatRequest
		if err := json.Unmarshal(msgBytes, &req); err != nil {
			writeWSEvent(conn, "error", map[string]any{"error": "invalid JSON"})
			continue
		}

		if req.Message == "" {
			writeWSEvent(conn, "error", map[string]any{"error": "message is required"})
			continue
		}

		s.handleChatMessage(conn, req)
	}
}

func (s *Server) handleChatMessage(conn *websocket.Conn, req chatRequest) {
	// Resolve agent config
	agentDir, err := s.resolveAgentDir(req.Agent)
	if err != nil {
		writeWSEvent(conn, "error", map[string]any{"error": "agent not found: " + req.Agent})
		return
	}

	cfg, err := agentcfg.Load(agentDir)
	if err != nil {
		writeWSEvent(conn, "error", map[string]any{"error": "load agent: " + err.Error()})
		return
	}

	// Initialize provider
	prov, err := initDashboardProvider(cfg)
	if err != nil {
		writeWSEvent(conn, "error", map[string]any{"error": "provider init: " + err.Error()})
		return
	}

	// Start MCP servers
	mgr := servermgr.NewManager()
	mgr.SetAgentToolWirer(&dashboardAgentToolWirer{})
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	for _, srv := range cfg.Servers {
		srvCtx, srvCancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := mgr.StartServer(srvCtx, srv, agentDir)
		srvCancel()
		if err != nil {
			s.logger.Warn("chat: start server failed", "server", srv.Name, "error", err)
		}
	}

	// Create session and wire output to WebSocket events
	sess := runtime.NewSession(cfg, prov, mgr)
	sess.Output = runtime.SilentOutput()

	sess.Output.OnText = func(text string) {
		writeWSEvent(conn, string(ws.EventAgentText), map[string]any{"text": text})
		// Also publish to hub for live monitor
		s.hub.Publish(ws.Event{
			Type:    ws.EventAgentText,
			Payload: map[string]any{"agent": cfg.Name, "text": text},
		})
	}
	sess.Output.OnToolStart = func(name string) {
		writeWSEvent(conn, string(ws.EventAgentToolStart), map[string]any{"tool": name})
		s.hub.Publish(ws.Event{
			Type:    ws.EventAgentToolStart,
			Payload: map[string]any{"agent": cfg.Name, "tool": name},
		})
	}
	sess.Output.OnToolResult = func(name, summary string, elapsed time.Duration) {
		writeWSEvent(conn, string(ws.EventAgentToolEnd), map[string]any{"tool": name, "summary": summary})
		s.hub.Publish(ws.Event{
			Type:    ws.EventAgentToolEnd,
			Payload: map[string]any{"agent": cfg.Name, "tool": name, "summary": summary},
		})
	}

	// Publish session start
	s.hub.Publish(ws.Event{
		Type:    ws.EventSessionStart,
		Payload: map[string]any{"agent": cfg.Name},
	})

	if err := sess.SendMessage(ctx, req.Message); err != nil {
		writeWSEvent(conn, "error", map[string]any{"error": err.Error()})
		s.hub.Publish(ws.Event{
			Type:    ws.EventSessionEnd,
			Payload: map[string]any{"agent": cfg.Name, "error": err.Error()},
		})
		return
	}

	// Send done event
	writeWSEvent(conn, string(ws.EventAgentDone), map[string]any{
		"tokens_in":  sess.TotalInput,
		"tokens_out": sess.TotalOutput,
		"tool_calls": sess.ToolCalls,
		"cost_usd":   sess.EstimatedCost(),
		"model":      cfg.Model.Model,
	})

	s.hub.Publish(ws.Event{
		Type: ws.EventSessionEnd,
		Payload: map[string]any{
			"agent":     cfg.Name,
			"tokens_in": sess.TotalInput,
			"cost_usd":  sess.EstimatedCost(),
		},
	})
}

// writeWSEvent sends a typed JSON event over the WebSocket.
func writeWSEvent(conn *websocket.Conn, eventType string, data map[string]any) {
	msg := map[string]any{
		"type":    eventType,
		"payload": data,
	}
	payload, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, payload)
}
