package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// OllamaStatus represents the state of a local Ollama instance.
type OllamaStatus struct {
	Available bool
	Version   string
	Endpoint  string // from OLLAMA_HOST or default
}

// OllamaEndpoint returns the Ollama API endpoint from OLLAMA_HOST or the default.
func OllamaEndpoint() string {
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return host
	}
	return "http://localhost:11434"
}

// DetectOllama checks whether Ollama is running and returns its status.
func DetectOllama() *OllamaStatus {
	endpoint := OllamaEndpoint()
	status := &OllamaStatus{Endpoint: endpoint}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint + "/api/version")
	if err != nil {
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return status
	}

	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return status
	}

	status.Available = true
	status.Version = body.Version
	return status
}

// RequireOllama returns an error if Ollama is not running.
func RequireOllama() (*OllamaStatus, error) {
	s := DetectOllama()
	if !s.Available {
		return s, fmt.Errorf("ollama is not running — start it with: ollama serve")
	}
	return s, nil
}
