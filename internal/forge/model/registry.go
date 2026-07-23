package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ModelEntry represents a single model tracked in the local registry.
type ModelEntry struct {
	Name       string    `json:"name"`
	OllamaName string   `json:"ollama_name"`
	Source     string    `json:"source"`
	Size       string    `json:"size"`
	PulledAt   time.Time `json:"pulled_at"`
	LastUsed   time.Time `json:"last_used,omitempty"`
}

// Registry manages the local model registry stored at ~/.demi/forge/models.json.
type Registry struct {
	Models []ModelEntry `json:"models"`
	path   string
}

// registryPath returns the default registry file path.
// It is a variable to allow test overrides.
var registryPath = defaultRegistryPath

func defaultRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".demi/forge", "models.json"), nil
}

// LoadRegistry reads the registry from ~/.demi/forge/models.json.
// If the file does not exist, it returns an empty registry.
func LoadRegistry() (*Registry, error) {
	p, err := registryPath()
	if err != nil {
		return nil, err
	}
	return LoadRegistryFrom(p)
}

// LoadRegistryFrom reads the registry from a specific path.
func LoadRegistryFrom(path string) (*Registry, error) {
	r := &Registry{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}

	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return r, nil
}

// Save writes the registry back to disk, creating the directory if needed.
func (r *Registry) Save() error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	return os.WriteFile(r.path, data, 0o644)
}

// Add inserts or updates a model entry in the registry.
func (r *Registry) Add(entry ModelEntry) error {
	for i, m := range r.Models {
		if m.Name == entry.Name {
			r.Models[i] = entry
			return r.Save()
		}
	}
	r.Models = append(r.Models, entry)
	return r.Save()
}

// Remove deletes a model entry by name.
func (r *Registry) Remove(name string) error {
	for i, m := range r.Models {
		if m.Name == name || m.OllamaName == name {
			r.Models = append(r.Models[:i], r.Models[i+1:]...)
			return r.Save()
		}
	}
	return fmt.Errorf("model not found in registry: %s", name)
}

// Get retrieves a model entry by name.
func (r *Registry) Get(name string) (*ModelEntry, error) {
	for _, m := range r.Models {
		if m.Name == name || m.OllamaName == name {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("model not found: %s", name)
}

// List returns all model entries.
func (r *Registry) List() []ModelEntry {
	return r.Models
}

// UpdateLastUsed updates the last_used timestamp for a model.
func (r *Registry) UpdateLastUsed(name string) error {
	for i, m := range r.Models {
		if m.Name == name || m.OllamaName == name {
			r.Models[i].LastUsed = time.Now()
			return r.Save()
		}
	}
	return fmt.Errorf("model not found: %s", name)
}
