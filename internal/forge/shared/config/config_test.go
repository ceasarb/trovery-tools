package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServerConfig(t *testing.T) {
	dir := t.TempDir()

	toml := `[server]
name = "weather-api"
entry = "src/weather_api/server.py"
command = "uv run weather-api"
transport = "stdio"

[testing]
fixtures = "tests/fixtures"
`
	os.WriteFile(filepath.Join(dir, "demi.toml"), []byte(toml), 0o644)

	cfg, err := LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("LoadServerConfig: %v", err)
	}

	if cfg.Server.Name != "weather-api" {
		t.Errorf("Name = %q", cfg.Server.Name)
	}
	if cfg.Server.Command != "uv run weather-api" {
		t.Errorf("Command = %q", cfg.Server.Command)
	}
	if cfg.Server.Transport != "stdio" {
		t.Errorf("Transport = %q", cfg.Server.Transport)
	}
	if cfg.Testing.Fixtures != "tests/fixtures" {
		t.Errorf("Fixtures = %q", cfg.Testing.Fixtures)
	}
}

func TestLoadServerConfigHTTP(t *testing.T) {
	dir := t.TempDir()

	toml := `[server]
name = "http-server"
command = "uv run http-server"
transport = "http"
port = 8000

[testing]
fixtures = "tests/fixtures"
`
	os.WriteFile(filepath.Join(dir, "demi.toml"), []byte(toml), 0o644)

	cfg, err := LoadServerConfig(dir)
	if err != nil {
		t.Fatalf("LoadServerConfig: %v", err)
	}

	if cfg.Server.Port != 8000 {
		t.Errorf("Port = %d, want 8000", cfg.Server.Port)
	}
}

func TestLoadServerConfigMissing(t *testing.T) {
	_, err := LoadServerConfig(t.TempDir())
	if err == nil {
		t.Error("expected error for missing demi.toml")
	}
}

func TestLoadServerConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "demi.toml"), []byte("{{invalid toml"), 0o644)

	_, err := LoadServerConfig(dir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}
