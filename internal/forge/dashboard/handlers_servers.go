package dashboard

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/config"
)

type serverListItem struct {
	Name      string `json:"name"`
	Language  string `json:"language"`
	Transport string `json:"transport"`
	Path      string `json:"path"`
}

type serverDetail struct {
	Name      string              `json:"name"`
	Language  string              `json:"language"`
	Transport string              `json:"transport"`
	Port      int                 `json:"port,omitempty"`
	Entry     string              `json:"entry"`
	Command   string              `json:"command"`
	Path      string              `json:"path"`
	Fixtures  string              `json:"fixtures,omitempty"`
}

// handleListServers scans the workspace servers/ directory for trove.toml files.
func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	serversDir := filepath.Join(s.workDir, "servers")
	entries, err := os.ReadDir(serversDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, listResponse{Data: []serverListItem{}, Total: 0})
			return
		}
		writeError(w, http.StatusInternalServerError, "read servers directory: "+err.Error())
		return
	}

	var items []serverListItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(serversDir, e.Name())
		cfg, err := config.LoadServerConfig(dir)
		if err != nil {
			continue // skip directories without valid trove.toml
		}
		items = append(items, serverListItem{
			Name:      cfg.Server.Name,
			Language:  detectLanguage(dir),
			Transport: cfg.Server.Transport,
			Path:      dir,
		})
	}

	if items == nil {
		items = []serverListItem{}
	}
	writeJSON(w, http.StatusOK, listResponse{Data: items, Total: len(items)})
}

// handleGetServer returns detail for a single server by name.
func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	serversDir := filepath.Join(s.workDir, "servers")

	entries, err := os.ReadDir(serversDir)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(serversDir, e.Name())
		cfg, err := config.LoadServerConfig(dir)
		if err != nil || cfg.Server.Name != name {
			continue
		}

		detail := serverDetail{
			Name:      cfg.Server.Name,
			Language:  detectLanguage(dir),
			Transport: cfg.Server.Transport,
			Port:      cfg.Server.Port,
			Entry:     cfg.Server.Entry,
			Command:   cfg.Server.Command,
			Path:      dir,
			Fixtures:  cfg.Testing.Fixtures,
		}
		writeJSON(w, http.StatusOK, detailResponse{Data: detail})
		return
	}

	writeError(w, http.StatusNotFound, "server not found")
}

// detectLanguage guesses the language from files in a server directory.
func detectLanguage(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "typescript"
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	return "unknown"
}
