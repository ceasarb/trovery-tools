package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")

	idx, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(idx.Servers) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx.Servers))
	}
}

func TestLoadExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	data := `{"servers":[{"name":"weather","transport":"stdio","path":"/tmp/weather","published_at":"2026-01-01T00:00:00Z"}]}`
	os.WriteFile(path, []byte(data), 0o644)

	idx, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(idx.Servers) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(idx.Servers))
	}
	if idx.Servers[0].Name != "weather" {
		t.Errorf("name = %q, want %q", idx.Servers[0].Name, "weather")
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "registry.json")

	idx := &Index{
		Servers: []Entry{
			{Name: "test-server", Transport: "stdio", Path: "/tmp/test"},
		},
		path: path,
	}

	if err := idx.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var loaded Index
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(loaded.Servers) != 1 {
		t.Errorf("loaded entries = %d, want 1", len(loaded.Servers))
	}
}

func TestPublish(t *testing.T) {
	// Create a mock server directory with demi.toml
	serverDir := t.TempDir()
	toml := `[server]
name = "weather-api"
entry = "server.py"
command = "uv run weather-api"
transport = "stdio"

[registry]
description = "Weather data provider"
tags = ["weather", "api", "data"]
categories = ["data"]
author = "Test Author"
license = "MIT"
min_mcp_version = "1.0"
`
	os.WriteFile(filepath.Join(serverDir, "demi.toml"), []byte(toml), 0o644)

	regPath := filepath.Join(t.TempDir(), "registry.json")
	idx, _ := Load(regPath)

	entry, err := idx.Publish(serverDir)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if entry.Name != "weather-api" {
		t.Errorf("name = %q, want %q", entry.Name, "weather-api")
	}
	if entry.Description != "Weather data provider" {
		t.Errorf("description = %q", entry.Description)
	}
	if len(entry.Tags) != 3 {
		t.Errorf("tags count = %d, want 3", len(entry.Tags))
	}
	if entry.Author != "Test Author" {
		t.Errorf("author = %q", entry.Author)
	}
	if entry.Transport != "stdio" {
		t.Errorf("transport = %q", entry.Transport)
	}
	if entry.PublishedAt == "" {
		t.Error("published_at should be set")
	}

	if len(idx.Servers) != 1 {
		t.Errorf("index size = %d, want 1", len(idx.Servers))
	}
}

func TestPublishUpdatesExisting(t *testing.T) {
	serverDir := t.TempDir()
	toml := `[server]
name = "weather-api"
command = "uv run weather"
transport = "stdio"

[registry]
description = "Updated description"
`
	os.WriteFile(filepath.Join(serverDir, "demi.toml"), []byte(toml), 0o644)

	regPath := filepath.Join(t.TempDir(), "registry.json")
	idx, _ := Load(regPath)

	// Publish twice
	idx.Publish(serverDir)
	idx.Publish(serverDir)

	if len(idx.Servers) != 1 {
		t.Errorf("expected 1 entry after double publish, got %d", len(idx.Servers))
	}
	if idx.Servers[0].Description != "Updated description" {
		t.Errorf("description not updated: %q", idx.Servers[0].Description)
	}
}

func TestInfo(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "weather", Transport: "stdio"},
			{Name: "search", Transport: "http"},
		},
	}

	if e := idx.Info("weather"); e == nil {
		t.Error("expected to find weather")
	} else if e.Transport != "stdio" {
		t.Errorf("transport = %q", e.Transport)
	}

	if e := idx.Info("nonexistent"); e != nil {
		t.Error("expected nil for nonexistent")
	}
}

func TestRemove(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "weather"},
			{Name: "search"},
		},
	}

	if !idx.Remove("weather") {
		t.Error("expected Remove to return true")
	}
	if len(idx.Servers) != 1 {
		t.Errorf("expected 1 entry, got %d", len(idx.Servers))
	}

	if idx.Remove("nonexistent") {
		t.Error("expected Remove to return false for nonexistent")
	}
}

func TestSearchExactName(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "weather-api", Description: "Weather data"},
			{Name: "search-api", Description: "Search engine"},
			{Name: "my-weather", Description: "Another weather thing"},
		},
	}

	results := idx.Search("weather-api")
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Entry.Name != "weather-api" {
		t.Errorf("first result = %q, want weather-api", results[0].Entry.Name)
	}
	// Exact match should score highest
	if results[0].Score < 100 {
		t.Errorf("exact match score = %d, expected >= 100", results[0].Score)
	}
}

func TestSearchPartialName(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "weather-api"},
			{Name: "search-api"},
		},
	}

	results := idx.Search("weather")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Name != "weather-api" {
		t.Errorf("result = %q", results[0].Entry.Name)
	}
}

func TestSearchByTag(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "server-a", Tags: []string{"weather", "api"}},
			{Name: "server-b", Tags: []string{"search"}},
		},
	}

	results := idx.Search("weather")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Name != "server-a" {
		t.Errorf("result = %q", results[0].Entry.Name)
	}
}

func TestSearchByDescription(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "alpha", Description: "Provides weather forecasts"},
			{Name: "beta", Description: "Manages user auth"},
		},
	}

	results := idx.Search("forecast")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Name != "alpha" {
		t.Errorf("result = %q", results[0].Entry.Name)
	}
}

func TestSearchByCategory(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "server-a", Categories: []string{"data", "analytics"}},
			{Name: "server-b", Categories: []string{"auth"}},
		},
	}

	results := idx.Search("data")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Entry.Name != "server-a" {
		t.Errorf("result = %q", results[0].Entry.Name)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
	}

	results := idx.Search("")
	if len(results) != 3 {
		t.Errorf("expected all 3 entries, got %d", len(results))
	}
}

func TestSearchNoResults(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "weather"},
		},
	}

	results := idx.Search("nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "Weather-API", Tags: []string{"WEATHER"}},
		},
	}

	results := idx.Search("weather")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (case insensitive), got %d", len(results))
	}
}

func TestSearchRanking(t *testing.T) {
	idx := &Index{
		Servers: []Entry{
			{Name: "unrelated", Description: "has weather in description"},
			{Name: "weather", Description: "exact name match"},
			{Name: "tag-only", Tags: []string{"weather"}},
		},
	}

	results := idx.Search("weather")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Exact name match should rank first
	if results[0].Entry.Name != "weather" {
		t.Errorf("first result = %q, want exact name match 'weather'", results[0].Entry.Name)
	}
}

func TestPublishNoConfig(t *testing.T) {
	idx := &Index{path: filepath.Join(t.TempDir(), "registry.json")}

	_, err := idx.Publish(t.TempDir())
	if err == nil {
		t.Error("expected error for dir without demi.toml")
	}
}
