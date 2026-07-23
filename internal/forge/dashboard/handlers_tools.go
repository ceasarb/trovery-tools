package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/ceasarb/demigo-tools/internal/forge/server/harness"
	"github.com/ceasarb/demigo-tools/internal/forge/shared/config"
)

type toolListItem struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type toolCallRequest struct {
	Server    string          `json:"server"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolCallResponse struct {
	Result     interface{} `json:"result"`
	DurationMs int64       `json:"duration_ms"`
}

// handleListTools lists tools from a running MCP server specified by ?server=name.
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	serverName := r.URL.Query().Get("server")
	if serverName == "" {
		writeError(w, http.StatusBadRequest, "server query parameter required")
		return
	}

	cfg, dir, err := s.findServerConfig(serverName)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found: "+err.Error())
		return
	}

	cmd, args := parseCommand(cfg.Server.Command)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := harness.Start(ctx, cmd, args, dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start server: "+err.Error())
		return
	}
	defer client.Close()

	tools, err := client.ListTools(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list tools: "+err.Error())
		return
	}

	items := make([]toolListItem, 0, len(tools))
	for _, t := range tools {
		items = append(items, toolListItem{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	writeJSON(w, http.StatusOK, listResponse{Data: items, Total: len(items)})
}

// handleCallTool invokes a tool on an MCP server.
func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request) {
	var req toolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Server == "" || req.Tool == "" {
		writeError(w, http.StatusBadRequest, "server and tool fields required")
		return
	}

	cfg, dir, err := s.findServerConfig(req.Server)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found: "+err.Error())
		return
	}

	cmd, args := parseCommand(cfg.Server.Command)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := harness.Start(ctx, cmd, args, dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start server: "+err.Error())
		return
	}
	defer client.Close()

	var arguments interface{}
	if len(req.Arguments) > 0 {
		json.Unmarshal(req.Arguments, &arguments)
	}

	result, elapsed, err := client.CallTool(ctx, req.Tool, arguments)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "call tool: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detailResponse{
		Data: toolCallResponse{
			Result:     result,
			DurationMs: elapsed.Milliseconds(),
		},
	})
}

// findServerConfig locates a server by name in the workspace servers/ directory.
func (s *Server) findServerConfig(name string) (*config.ServerConfig, string, error) {
	serversDir := filepath.Join(s.workDir, "servers")
	entries, err := filepath.Glob(filepath.Join(serversDir, "*", "demi.toml"))
	if err != nil {
		return nil, "", err
	}

	for _, entry := range entries {
		dir := filepath.Dir(entry)
		cfg, err := config.LoadServerConfig(dir)
		if err != nil {
			continue
		}
		if cfg.Server.Name == name {
			return cfg, dir, nil
		}
	}

	return nil, "", fmt.Errorf("server %q not found", name)
}

// parseCommand splits a command string into the executable and arguments.
func parseCommand(command string) (string, []string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command, nil
	}
	return parts[0], parts[1:]
}
