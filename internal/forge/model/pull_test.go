package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestTranslateHFName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"mistral", "mistral"},
		{"llama3", "llama3"},
		{"hf:mistralai/Mistral-7B-v0.1", "mistral-7b-v0.1"},
		{"hf:meta-llama/Llama-2-7b-chat-hf", "llama-2-7b-chat-hf"},
		{"hf:TheBloke/Some-Model-GGUF", "some-model-gguf"},
		{"hf:model-no-org", "model-no-org"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := TranslateHFName(tc.input)
			if got != tc.want {
				t.Errorf("TranslateHFName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPull_ProgressCallback(t *testing.T) {
	// Mock Ollama server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pull":
			w.Header().Set("Content-Type", "application/x-ndjson")
			lines := []map[string]any{
				{"status": "pulling manifest"},
				{"status": "downloading", "completed": int64(50), "total": int64(100)},
				{"status": "downloading", "completed": int64(100), "total": int64(100)},
				{"status": "success"},
			}
			for _, line := range lines {
				data, _ := json.Marshal(line)
				fmt.Fprintf(w, "%s\n", data)
			}
		case "/api/show":
			json.NewEncoder(w).Encode(map[string]any{
				"details": map[string]string{"parameter_size": "7B"},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)

	// Override registry path so we don't pollute the real one
	origPath := registryPath
	defer func() { registryPath = origPath }()
	tmpDir := t.TempDir()
	registryPath = func() (string, error) {
		return filepath.Join(tmpDir, "models.json"), nil
	}

	var progressCalls int
	err := Pull("testmodel", func(status string, completed, total int64) {
		progressCalls++
	})

	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if progressCalls == 0 {
		t.Error("expected progress callback to be called")
	}
}

func TestPull_HuggingFaceSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pull":
			w.Header().Set("Content-Type", "application/x-ndjson")
			data, _ := json.Marshal(map[string]string{"status": "success"})
			fmt.Fprintf(w, "%s\n", data)
		case "/api/show":
			json.NewEncoder(w).Encode(map[string]any{
				"details": map[string]string{"parameter_size": "7B"},
			})
		}
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)

	origPath := registryPath
	defer func() { registryPath = origPath }()
	tmpDir := t.TempDir()
	registryPath = func() (string, error) {
		return filepath.Join(tmpDir, "models.json"), nil
	}

	err := Pull("hf:mistralai/Mistral-7B-v0.1", nil)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// Verify registry entry
	reg, _ := LoadRegistryFrom(filepath.Join(tmpDir, "models.json"))
	models := reg.List()
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Source != "huggingface" {
		t.Errorf("source = %q, want %q", models[0].Source, "huggingface")
	}
	if models[0].OllamaName != "mistral-7b-v0.1" {
		t.Errorf("ollama_name = %q, want %q", models[0].OllamaName, "mistral-7b-v0.1")
	}
}

func TestPull_OllamaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)

	err := Pull("badmodel", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
