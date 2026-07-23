package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ModelInfo contains merged information about a model from Ollama and the local registry.
type ModelInfo struct {
	Name       string
	OllamaName string
	Source     string
	Size       string
	PulledAt   time.Time
	LastUsed   time.Time
}

// ListModels returns models by merging Ollama's tag list with the local registry.
// If Ollama is unavailable, only registry data is returned.
func ListModels() ([]ModelInfo, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	// Build a lookup from ollama_name -> registry entry
	regByOllama := make(map[string]ModelEntry)
	for _, m := range reg.List() {
		regByOllama[m.OllamaName] = m
	}

	// Try fetching from Ollama
	ollamaModels, ollamaErr := fetchOllamaTags()

	var results []ModelInfo
	seen := make(map[string]bool)

	if ollamaErr == nil {
		for _, om := range ollamaModels {
			info := ModelInfo{
				Name:       om.Name,
				OllamaName: om.Name,
				Source:     "ollama",
				Size:       om.Size,
			}

			if entry, ok := regByOllama[om.Name]; ok {
				info.Name = entry.Name
				info.Source = entry.Source
				info.Size = entry.Size
				info.PulledAt = entry.PulledAt
				info.LastUsed = entry.LastUsed
			}

			results = append(results, info)
			seen[om.Name] = true
		}
	}

	// Add registry-only entries (models in registry but not in Ollama)
	for _, entry := range reg.List() {
		if !seen[entry.OllamaName] {
			results = append(results, ModelInfo{
				Name:       entry.Name,
				OllamaName: entry.OllamaName,
				Source:     entry.Source,
				Size:       entry.Size,
				PulledAt:   entry.PulledAt,
				LastUsed:   entry.LastUsed,
			})
		}
	}

	return results, nil
}

type ollamaModel struct {
	Name string
	Size string
}

func fetchOllamaTags() ([]ollamaModel, error) {
	endpoint := OllamaEndpoint()
	resp, err := http.Get(endpoint + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama tags returned status %d", resp.StatusCode)
	}

	var body struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize string `json:"parameter_size"`
			} `json:"details"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	var models []ollamaModel
	for _, m := range body.Models {
		size := m.Details.ParameterSize
		if size == "" {
			size = formatBytes(m.Size)
		}
		models = append(models, ollamaModel{Name: m.Name, Size: size})
	}

	return models, nil
}

func formatBytes(b int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
