package model

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectOllama_Available(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{"version": "0.1.42"})
	}))
	defer srv.Close()

	t.Setenv("OLLAMA_HOST", srv.URL)

	status := DetectOllama()
	if !status.Available {
		t.Fatal("expected Ollama to be available")
	}
	if status.Version != "0.1.42" {
		t.Errorf("version = %q, want %q", status.Version, "0.1.42")
	}
	if status.Endpoint != srv.URL {
		t.Errorf("endpoint = %q, want %q", status.Endpoint, srv.URL)
	}
}

func TestDetectOllama_NotRunning(t *testing.T) {
	// Point at a port that is not listening
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	status := DetectOllama()
	if status.Available {
		t.Fatal("expected Ollama to be unavailable")
	}
}

func TestRequireOllama_Error(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	_, err := RequireOllama()
	if err == nil {
		t.Fatal("expected error when Ollama is not running")
	}
}

func TestOllamaEndpoint_Default(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "")
	ep := OllamaEndpoint()
	if ep != "http://localhost:11434" {
		t.Errorf("endpoint = %q, want default", ep)
	}
}

func TestOllamaEndpoint_EnvVar(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://custom:9999")
	ep := OllamaEndpoint()
	if ep != "http://custom:9999" {
		t.Errorf("endpoint = %q, want custom", ep)
	}
}
