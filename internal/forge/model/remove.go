package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Remove deletes a model from both Ollama and the local registry.
func Remove(name string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Resolve the Ollama name from registry if possible
	ollamaName := name
	if entry, err := reg.Get(name); err == nil {
		ollamaName = entry.OllamaName
	}

	// Delete from Ollama
	endpoint := OllamaEndpoint()
	body, _ := json.Marshal(map[string]string{"name": ollamaName})

	req, err := http.NewRequest(http.MethodDelete, endpoint+"/api/delete", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete from ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama delete returned status %d", resp.StatusCode)
	}

	// Remove from local registry (ignore error if not in registry)
	_ = reg.Remove(name)

	return nil
}
