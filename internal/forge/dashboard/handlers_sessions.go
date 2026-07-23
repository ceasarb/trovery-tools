package dashboard

import (
	"net/http"
	"time"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/storage"
)

type sessionListItem struct {
	ID             string     `json:"id"`
	AgentName      string     `json:"agent_name"`
	Provider       string     `json:"provider"`
	Model          string     `json:"model"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	TotalTurns     int        `json:"total_turns"`
	TotalTokensIn  int        `json:"total_tokens_in"`
	TotalTokensOut int        `json:"total_tokens_out"`
	TotalCostUSD   float64    `json:"total_cost_usd"`
	Summary        *string    `json:"summary"`
}

type turnJSON struct {
	ID         string         `json:"id"`
	TurnNumber int            `json:"turn_number"`
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	TokensIn   int            `json:"tokens_in"`
	TokensOut  int            `json:"tokens_out"`
	CostUSD    float64        `json:"cost_usd"`
	CreatedAt  time.Time      `json:"created_at"`
	ToolCalls  []toolCallJSON `json:"tool_calls"`
}

type toolCallJSON struct {
	ID         string  `json:"id"`
	ToolName   string  `json:"tool_name"`
	ServerName string  `json:"server_name"`
	Arguments  *string `json:"arguments,omitempty"`
	Result     *string `json:"result,omitempty"`
	DurationMs *int64  `json:"duration_ms"`
	Error      *string `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type sessionDetailJSON struct {
	Session sessionListItem `json:"session"`
	Turns   []turnJSON      `json:"turns"`
}

// handleListSessions returns sessions with optional agent filter and pagination.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	agent := r.URL.Query().Get("agent")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	sessions, err := s.sessStore.ListSessions(agent, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list sessions: "+err.Error())
		return
	}

	items := make([]sessionListItem, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, toSessionListItem(sess))
	}

	writeJSON(w, http.StatusOK, listResponse{Data: items, Total: len(items)})
}

// handleGetSession returns a session with all turns and tool calls.
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sess, err := s.sessStore.GetSession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get session: "+err.Error())
		return
	}
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	turns, err := s.sessStore.GetTurnsBySession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get turns: "+err.Error())
		return
	}

	turnItems := make([]turnJSON, 0, len(turns))
	for _, t := range turns {
		calls, err := s.sessStore.GetToolCallsByTurn(t.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "get tool calls: "+err.Error())
			return
		}

		callItems := make([]toolCallJSON, 0, len(calls))
		for _, c := range calls {
			callItems = append(callItems, toolCallJSON{
				ID:         c.ID,
				ToolName:   c.ToolName,
				ServerName: c.ServerName,
				Arguments:  c.ArgumentsJSON,
				Result:     c.ResultJSON,
				DurationMs: c.DurationMs,
				Error:      c.Error,
				CreatedAt:  c.CreatedAt,
			})
		}

		turnItems = append(turnItems, turnJSON{
			ID:         t.ID,
			TurnNumber: t.TurnNumber,
			Role:       t.Role,
			Content:    t.Content,
			TokensIn:   t.TokensIn,
			TokensOut:  t.TokensOut,
			CostUSD:    t.CostUSD,
			CreatedAt:  t.CreatedAt,
			ToolCalls:  callItems,
		})
	}

	detail := sessionDetailJSON{
		Session: toSessionListItem(*sess),
		Turns:   turnItems,
	}

	writeJSON(w, http.StatusOK, detailResponse{Data: detail})
}

// handleExportSession returns the full session as a JSON dump.
func (s *Server) handleExportSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sess, err := s.sessStore.GetSession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get session: "+err.Error())
		return
	}
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	turns, err := s.sessStore.GetTurnsBySession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get turns: "+err.Error())
		return
	}

	allCalls, err := s.sessStore.GetToolCallsBySession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get tool calls: "+err.Error())
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=session-"+id+".json")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session":    sess,
		"turns":      turns,
		"tool_calls": allCalls,
	})
}

func toSessionListItem(sess storage.Session) sessionListItem {
	return sessionListItem{
		ID:             sess.ID,
		AgentName:      sess.AgentName,
		Provider:       sess.Provider,
		Model:          sess.Model,
		StartedAt:      sess.StartedAt,
		FinishedAt:     sess.FinishedAt,
		TotalTurns:     sess.TotalTurns,
		TotalTokensIn:  sess.TotalTokensIn,
		TotalTokensOut: sess.TotalTokensOut,
		TotalCostUSD:   sess.TotalCostUSD,
		Summary:        sess.Summary,
	}
}
