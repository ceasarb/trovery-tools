package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ProgressFunc is called with status updates during a pull operation.
type ProgressFunc func(status string, completed, total int64)

// TranslateHFName converts a HuggingFace model reference to an Ollama-compatible name.
// Input format: "hf:org/model-name" -> best-effort Ollama name.
func TranslateHFName(name string) string {
	if !strings.HasPrefix(name, "hf:") {
		return name
	}

	raw := strings.TrimPrefix(name, "hf:")

	// Strip org prefix if present (e.g., "mistralai/Mistral-7B-v0.1" -> "mistral-7b-v0.1")
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		raw = raw[idx+1:]
	}

	return strings.ToLower(raw)
}

// Pull downloads a model through the Ollama API.
// It accepts either plain Ollama names ("mistral") or HuggingFace refs ("hf:mistralai/Mistral-7B-v0.1").
func Pull(name string, onProgress ProgressFunc) error {
	ollamaName := TranslateHFName(name)
	endpoint := OllamaEndpoint()

	// Request pull
	body, _ := json.Marshal(map[string]any{
		"name":   ollamaName,
		"stream": true,
	})

	resp, err := http.Post(endpoint+"/api/pull", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	// Stream NDJSON progress
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var line struct {
			Status    string `json:"status"`
			Completed int64  `json:"completed"`
			Total     int64  `json:"total"`
			Error     string `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Error != "" {
			return fmt.Errorf("pull error: %s", line.Error)
		}
		if onProgress != nil {
			onProgress(line.Status, line.Completed, line.Total)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading pull stream: %w", err)
	}

	// Get model info for registry
	info, err := showModel(endpoint, ollamaName)
	if err != nil {
		return fmt.Errorf("get model info after pull: %w", err)
	}

	// Determine display name and source
	displayName := ollamaName
	source := "ollama"
	if strings.HasPrefix(name, "hf:") {
		displayName = name
		source = "huggingface"
	}

	// Register in local registry
	reg, err := LoadRegistry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	return reg.Add(ModelEntry{
		Name:       displayName,
		OllamaName: ollamaName,
		Source:     source,
		Size:       info.Size,
		PulledAt:   time.Now(),
	})
}

type modelShowInfo struct {
	Size string
}

func showModel(endpoint, name string) (*modelShowInfo, error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	resp, err := http.Post(endpoint+"/api/show", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("show returned status %d", resp.StatusCode)
	}

	var result struct {
		Details struct {
			ParameterSize string `json:"parameter_size"`
		} `json:"details"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	size := result.Details.ParameterSize
	if size == "" {
		size = "unknown"
	}

	return &modelShowInfo{Size: size}, nil
}
