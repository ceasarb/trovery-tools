package dashboard

import (
	"net/http"
	"os"
	"path/filepath"

	agentconfig "github.com/ceasarb/demigo-tools/internal/forge/agent/config"
)

type agentListItem struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Path     string `json:"path"`
}

type agentDetail struct {
	Name         string                   `json:"name"`
	Provider     string                   `json:"provider"`
	Model        string                   `json:"model"`
	SystemPrompt string                   `json:"system_prompt"`
	Servers      []agentconfig.ServerRef  `json:"servers"`
	Settings     agentconfig.AgentSettings `json:"settings"`
	Path         string                   `json:"path"`
}

// handleListAgents scans the workspace agents/ directory for agent.yaml files.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agentsDir := filepath.Join(s.workDir, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, listResponse{Data: []agentListItem{}, Total: 0})
			return
		}
		writeError(w, http.StatusInternalServerError, "read agents directory: "+err.Error())
		return
	}

	var items []agentListItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(agentsDir, e.Name())
		cfg, err := agentconfig.Load(dir)
		if err != nil {
			continue // skip directories without valid agent.yaml
		}
		items = append(items, agentListItem{
			Name:     cfg.Name,
			Provider: cfg.Model.Provider,
			Model:    cfg.Model.Model,
			Path:     dir,
		})
	}

	if items == nil {
		items = []agentListItem{}
	}
	writeJSON(w, http.StatusOK, listResponse{Data: items, Total: len(items)})
}

// handleGetAgent returns detail for a single agent by name.
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	agentsDir := filepath.Join(s.workDir, "agents")

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(agentsDir, e.Name())
		cfg, err := agentconfig.Load(dir)
		if err != nil || cfg.Name != name {
			continue
		}

		servers := cfg.Servers
		if servers == nil {
			servers = []agentconfig.ServerRef{}
		}

		detail := agentDetail{
			Name:         cfg.Name,
			Provider:     cfg.Model.Provider,
			Model:        cfg.Model.Model,
			SystemPrompt: cfg.System,
			Servers:      servers,
			Settings:     cfg.Settings,
			Path:         dir,
		}
		writeJSON(w, http.StatusOK, detailResponse{Data: detail})
		return
	}

	writeError(w, http.StatusNotFound, "agent not found")
}
