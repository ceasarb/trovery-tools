package export

import (
	"encoding/json"
	"fmt"
	"strings"
)

// mcpServerEntry represents a single server in claude_desktop_config.json.
type mcpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// mcpDesktopConfig is the top-level structure for Claude Desktop config.
type mcpDesktopConfig struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

func (e *Exporter) exportMCPClient() (*ExportResult, error) {
	servers := make(map[string]mcpServerEntry)

	for _, s := range e.config.Servers {
		if s.URL != "" {
			// External servers can't be expressed as command-based entries;
			// include them as a comment-like entry with the URL in env.
			servers[s.Name] = mcpServerEntry{
				Command: "",
				Args:    nil,
				Env:     map[string]string{"URL": s.URL},
			}
			continue
		}

		// Parse the command string into command + args
		parts := strings.Fields(s.Command)
		cmd := ""
		var args []string
		if len(parts) > 0 {
			cmd = parts[0]
			args = parts[1:]
		}

		servers[s.Name] = mcpServerEntry{
			Command: cmd,
			Args:    args,
			Env:     map[string]string{},
		}
	}

	cfg := mcpDesktopConfig{MCPServers: servers}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal mcp config: %w", err)
	}

	if err := e.writeFile("claude_desktop_config.json", string(data)+"\n"); err != nil {
		return nil, err
	}

	return &ExportResult{
		Format: FormatMCPClient,
		Files: []ExportedFile{
			{Path: "claude_desktop_config.json", Description: "Claude Desktop MCP server config"},
		},
	}, nil
}
