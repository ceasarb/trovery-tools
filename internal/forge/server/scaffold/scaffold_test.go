package scaffold

import (
	"strings"
	"testing"
)

func TestRunPythonStdio(t *testing.T) {
	files, err := Run(Options{
		Name:        "weather-api",
		Language:    Python,
		Transport:   Stdio,
		Description: "A weather service",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Check expected files exist
	expected := map[string]bool{
		"pyproject.toml":                true,
		"src/weather_api/__init__.py":   true,
		"src/weather_api/server.py":     true,
		"tests/__init__.py":             true,
		"tests/fixtures/test_tools.yaml": true,
		"README.md":                     true,
		"trove.toml":                     true,
	}

	for _, f := range files {
		delete(expected, f.Path)
	}

	for missing := range expected {
		t.Errorf("missing file: %s", missing)
	}
}

func TestRunTypescriptStdio(t *testing.T) {
	files, err := Run(Options{
		Name:        "data-query",
		Language:    TypeScript,
		Transport:   Stdio,
		Description: "A data service",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	expected := map[string]bool{
		"package.json":                   true,
		"tsconfig.json":                  true,
		"src/server.ts":                  true,
		"tests/fixtures/test_tools.yaml": true,
		"README.md":                      true,
		"trove.toml":                      true,
	}

	for _, f := range files {
		delete(expected, f.Path)
	}

	for missing := range expected {
		t.Errorf("missing file: %s", missing)
	}
}

func TestRunPythonHTTP(t *testing.T) {
	files, err := Run(Options{
		Name:      "http-server",
		Language:  Python,
		Transport: HTTP,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify HTTP transport shows up in trove.toml
	for _, f := range files {
		if f.Path == "trove.toml" {
			if !strings.Contains(f.Content, `transport = "http"`) {
				t.Error("trove.toml missing http transport")
			}
			if !strings.Contains(f.Content, "port = 8000") {
				t.Error("trove.toml missing port")
			}
		}
		if f.Path == "src/http_server/server.py" {
			if !strings.Contains(f.Content, "streamable-http") {
				t.Error("server.py should use streamable-http transport")
			}
		}
	}
}

func TestRunUnsupportedTemplate(t *testing.T) {
	_, err := Run(Options{
		Name:      "test",
		Language:  "rust",
		Transport: Stdio,
	})
	if err == nil {
		t.Error("expected error for unsupported template")
	}
}

func TestRunTemplateContent(t *testing.T) {
	files, err := Run(Options{
		Name:        "my-server",
		Language:    Python,
		Transport:   Stdio,
		Description: "Test server",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, f := range files {
		// No unrendered template variables should remain
		if strings.Contains(f.Content, "{{.") {
			t.Errorf("%s contains unrendered template variable", f.Path)
		}

		// Check service name substitution
		if f.Path == "trove.toml" {
			if !strings.Contains(f.Content, `name = "my-server"`) {
				t.Errorf("trove.toml missing server name")
			}
			if !strings.Contains(f.Content, `command = "uv run my-server"`) {
				t.Errorf("trove.toml missing correct command")
			}
		}

		if f.Path == "src/my_server/server.py" {
			if !strings.Contains(f.Content, `transport="stdio"`) {
				t.Error("server.py should set stdio transport")
			}
			if !strings.Contains(f.Content, "show_banner=False") {
				t.Error("server.py should disable banner")
			}
		}
	}
}

func TestToSnake(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"weather-api", "weather_api"},
		{"MyServer", "my_server"},
		{"simple", "simple"},
		{"data-query", "data_query"},
	}

	for _, tc := range tests {
		got := toSnake(tc.in)
		if got != tc.want {
			t.Errorf("toSnake(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
