package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/config"
)

// Entry represents a single server in the registry index.
type Entry struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Categories    []string `json:"categories,omitempty"`
	Author        string   `json:"author,omitempty"`
	License       string   `json:"license,omitempty"`
	MinMCPVersion string   `json:"min_mcp_version,omitempty"`
	Homepage      string   `json:"homepage,omitempty"`
	Transport     string   `json:"transport"`
	Command       string   `json:"command,omitempty"`
	Path          string   `json:"path"`
	PublishedAt   string   `json:"published_at"`
}

// Index is the local registry index stored at ~/.trove/forge/registry.json.
type Index struct {
	Servers []Entry `json:"servers"`
	path    string
}

// DefaultPath returns the default registry index path (~/.trove/forge/registry.json).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".trove/forge", "registry.json"), nil
}

// Load reads the registry index from disk. Returns an empty index if the file doesn't exist.
func Load(path string) (*Index, error) {
	idx := &Index{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}

	if err := json.Unmarshal(data, idx); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}

	return idx, nil
}

// Save writes the registry index to disk.
func (idx *Index) Save() error {
	dir := filepath.Dir(idx.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	return os.WriteFile(idx.path, data, 0o644)
}

// Publish adds or updates a server entry in the index from its trove.toml config.
func (idx *Index) Publish(serverDir string) (*Entry, error) {
	cfg, err := config.LoadServerConfig(serverDir)
	if err != nil {
		return nil, fmt.Errorf("load server config: %w", err)
	}

	absPath, err := filepath.Abs(serverDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	entry := Entry{
		Name:          cfg.Server.Name,
		Description:   cfg.Registry.Description,
		Tags:          cfg.Registry.Tags,
		Categories:    cfg.Registry.Categories,
		Author:        cfg.Registry.Author,
		License:       cfg.Registry.License,
		MinMCPVersion: cfg.Registry.MinMCPVersion,
		Homepage:      cfg.Registry.Homepage,
		Transport:     cfg.Server.Transport,
		Command:       cfg.Server.Command,
		Path:          absPath,
		PublishedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// Update existing or append
	found := false
	for i, e := range idx.Servers {
		if e.Name == entry.Name {
			idx.Servers[i] = entry
			found = true
			break
		}
	}
	if !found {
		idx.Servers = append(idx.Servers, entry)
	}

	return &entry, nil
}

// Remove deletes a server entry from the index by name. Returns true if found.
func (idx *Index) Remove(name string) bool {
	for i, e := range idx.Servers {
		if e.Name == name {
			idx.Servers = append(idx.Servers[:i], idx.Servers[i+1:]...)
			return true
		}
	}
	return false
}

// Info returns the entry for a server by name, or nil if not found.
func (idx *Index) Info(name string) *Entry {
	for _, e := range idx.Servers {
		if e.Name == name {
			return &e
		}
	}
	return nil
}

// SearchResult pairs an entry with a relevance score.
type SearchResult struct {
	Entry Entry
	Score int
}

// Search finds servers matching the query string against name, description, tags, and categories.
// Results are sorted by relevance score (highest first).
func (idx *Index) Search(query string) []SearchResult {
	if query == "" {
		// Return all entries with equal score
		results := make([]SearchResult, len(idx.Servers))
		for i, e := range idx.Servers {
			results[i] = SearchResult{Entry: e, Score: 1}
		}
		return results
	}

	q := strings.ToLower(query)
	var results []SearchResult

	for _, e := range idx.Servers {
		score := 0

		// Name match (highest weight)
		nameLower := strings.ToLower(e.Name)
		if nameLower == q {
			score += 100 // exact match
		} else if strings.Contains(nameLower, q) {
			score += 50 // partial name match
		}

		// Description match
		if strings.Contains(strings.ToLower(e.Description), q) {
			score += 20
		}

		// Tag match
		for _, tag := range e.Tags {
			if strings.ToLower(tag) == q {
				score += 30 // exact tag match
			} else if strings.Contains(strings.ToLower(tag), q) {
				score += 15
			}
		}

		// Category match
		for _, cat := range e.Categories {
			if strings.ToLower(cat) == q {
				score += 25 // exact category match
			} else if strings.Contains(strings.ToLower(cat), q) {
				score += 10
			}
		}

		if score > 0 {
			results = append(results, SearchResult{Entry: e, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}
