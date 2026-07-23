package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ceasarb/demigo-tools/internal/forge/agent/config"
)

func testConfig() *config.AgentConfig {
	return &config.AgentConfig{
		Name: "test-agent",
		Model: config.ModelConfig{
			Provider:    "anthropic",
			APIKeyEnv:   "ANTHROPIC_API_KEY",
			Model:       "claude-haiku-4-5-20251001",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		System: "You are a test assistant.",
		Servers: []config.ServerRef{
			{
				Name:    "weather",
				Path:    "/tmp/weather",
				Command: "uv run weather",
			},
			{
				Name: "search-api",
				URL:  "https://search.example.com/mcp",
			},
		},
		Settings: config.AgentSettings{
			MaxToolCalls: 25,
			TimeoutSecs:  120,
			Namespacing:  "auto",
		},
	}
}

func testConfigOpenAI() *config.AgentConfig {
	cfg := testConfig()
	cfg.Model.Provider = "openai"
	cfg.Model.APIKeyEnv = "OPENAI_API_KEY"
	cfg.Model.Model = "gpt-4o"
	return cfg
}

func testConfigNoServers() *config.AgentConfig {
	cfg := testConfig()
	cfg.Servers = nil
	return cfg
}

// --- Python Export ---

func TestExportPython_Anthropic(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	exp := New(cfg, FormatPython, dir)
	result, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if result.Format != FormatPython {
		t.Errorf("Format = %q, want %q", result.Format, FormatPython)
	}
	if len(result.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(result.Files))
	}

	// Verify agent.py exists and contains expected content
	script := readFile(t, dir, "agent.py")
	assertContains(t, script, "from anthropic import Anthropic")
	assertContains(t, script, "test-agent")
	assertContains(t, script, "You are a test assistant.")
	assertContains(t, script, `"weather"`)
	assertContains(t, script, "start_servers")
	assertContains(t, script, "search-api")

	// Verify requirements.txt
	reqs := readFile(t, dir, "requirements.txt")
	assertContains(t, reqs, "anthropic")
}

func TestExportPython_OpenAI(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigOpenAI()

	exp := New(cfg, FormatPython, dir)
	result, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if len(result.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(result.Files))
	}

	script := readFile(t, dir, "agent.py")
	assertContains(t, script, "from openai import OpenAI")
	assertContains(t, script, "gpt-4o")
	assertNotContains(t, script, "anthropic")

	reqs := readFile(t, dir, "requirements.txt")
	assertContains(t, reqs, "openai")
	assertNotContains(t, reqs, "anthropic")
}

func TestExportPython_NoServers(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigNoServers()

	exp := New(cfg, FormatPython, dir)
	_, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	script := readFile(t, dir, "agent.py")
	assertNotContains(t, script, "start_servers")
	assertNotContains(t, script, "SERVER_COMMANDS")
}

// --- FastAPI Export ---

func TestExportFastAPI(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	exp := New(cfg, FormatFastAPI, dir)
	result, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if result.Format != FormatFastAPI {
		t.Errorf("Format = %q, want %q", result.Format, FormatFastAPI)
	}
	if len(result.Files) != 3 {
		t.Errorf("Files count = %d, want 3", len(result.Files))
	}

	app := readFile(t, dir, "app.py")
	assertContains(t, app, "FastAPI")
	assertContains(t, app, "/invoke")
	assertContains(t, app, "/invoke/stream")
	assertContains(t, app, "/health")

	reqs := readFile(t, dir, "requirements.txt")
	assertContains(t, reqs, "fastapi")
	assertContains(t, reqs, "uvicorn")

	readme := readFile(t, dir, "README.md")
	assertContains(t, readme, "test-agent")
}

// --- Docker Export ---

func TestExportDocker(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	exp := New(cfg, FormatDocker, dir)
	result, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if result.Format != FormatDocker {
		t.Errorf("Format = %q, want %q", result.Format, FormatDocker)
	}
	if len(result.Files) != 6 {
		t.Errorf("Files count = %d, want 6", len(result.Files))
	}

	// Verify Dockerfile
	dockerfile := readFile(t, dir, "Dockerfile")
	assertContains(t, dockerfile, "python:3.13-slim")
	assertContains(t, dockerfile, "uvicorn")

	// Verify docker-compose.yml includes bundled server
	compose := readFile(t, dir, "docker-compose.yml")
	assertContains(t, compose, "weather")
	assertContains(t, compose, "ANTHROPIC_API_KEY")

	// Verify .dockerignore
	dockerignore := readFile(t, dir, ".dockerignore")
	assertContains(t, dockerignore, "__pycache__")

	// Verify .env.example
	envExample := readFile(t, dir, ".env.example")
	assertContains(t, envExample, "ANTHROPIC_API_KEY")
}

func TestExportDocker_ClassifiesServers(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	exp := New(cfg, FormatDocker, dir)
	bundled, external := exp.classifyServers()

	if len(bundled) != 1 {
		t.Errorf("bundled count = %d, want 1", len(bundled))
	}
	if len(external) != 1 {
		t.Errorf("external count = %d, want 1", len(external))
	}

	if bundled[0].Name != "weather" {
		t.Errorf("bundled[0].Name = %q, want %q", bundled[0].Name, "weather")
	}
	if external[0].Name != "search-api" {
		t.Errorf("external[0].Name = %q, want %q", external[0].Name, "search-api")
	}
}

func TestExportDocker_NoServers(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigNoServers()

	exp := New(cfg, FormatDocker, dir)
	_, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	compose := readFile(t, dir, "docker-compose.yml")
	assertNotContains(t, compose, "depends_on")
}

// --- MCP Client Export ---

func TestExportMCPClient(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	exp := New(cfg, FormatMCPClient, dir)
	result, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if result.Format != FormatMCPClient {
		t.Errorf("Format = %q, want %q", result.Format, FormatMCPClient)
	}
	if len(result.Files) != 1 {
		t.Errorf("Files count = %d, want 1", len(result.Files))
	}

	raw := readFile(t, dir, "claude_desktop_config.json")

	var desktopCfg mcpDesktopConfig
	if err := json.Unmarshal([]byte(raw), &desktopCfg); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	// Check bundled server
	weather, ok := desktopCfg.MCPServers["weather"]
	if !ok {
		t.Fatal("missing weather server entry")
	}
	if weather.Command != "uv" {
		t.Errorf("weather.Command = %q, want %q", weather.Command, "uv")
	}
	if len(weather.Args) != 2 || weather.Args[0] != "run" || weather.Args[1] != "weather" {
		t.Errorf("weather.Args = %v, want [run weather]", weather.Args)
	}

	// Check external server
	search, ok := desktopCfg.MCPServers["search-api"]
	if !ok {
		t.Fatal("missing search-api server entry")
	}
	if search.Env["URL"] != "https://search.example.com/mcp" {
		t.Errorf("search-api URL = %q", search.Env["URL"])
	}
}

func TestExportMCPClient_NoServers(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfigNoServers()

	exp := New(cfg, FormatMCPClient, dir)
	_, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	raw := readFile(t, dir, "claude_desktop_config.json")
	var desktopCfg mcpDesktopConfig
	if err := json.Unmarshal([]byte(raw), &desktopCfg); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	if len(desktopCfg.MCPServers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(desktopCfg.MCPServers))
	}
}

// --- Validation ---

func TestIsValidFormat(t *testing.T) {
	valid := []string{"python", "fastapi", "docker", "mcp-client"}
	for _, f := range valid {
		if !IsValidFormat(f) {
			t.Errorf("IsValidFormat(%q) = false, want true", f)
		}
	}

	invalid := []string{"ruby", "terraform", "", "Python"}
	for _, f := range invalid {
		if IsValidFormat(f) {
			t.Errorf("IsValidFormat(%q) = true, want false", f)
		}
	}
}

func TestExportUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	exp := New(cfg, Format("unknown"), dir)
	_, err := exp.Export()
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestExportCreatesOutputDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "output")
	cfg := testConfig()

	exp := New(cfg, FormatPython, dir)
	_, err := exp.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("output dir not created: %v", err)
	}
}

// --- Helpers ---

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func assertContains(t *testing.T, content, substr string) {
	t.Helper()
	if !strings.Contains(content, substr) {
		t.Errorf("expected content to contain %q, but it didn't.\nContent (first 500 chars):\n%s", substr, truncate(content, 500))
	}
}

func assertNotContains(t *testing.T, content, substr string) {
	t.Helper()
	if strings.Contains(content, substr) {
		t.Errorf("expected content to NOT contain %q, but it did", substr)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
