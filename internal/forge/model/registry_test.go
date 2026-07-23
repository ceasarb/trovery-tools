package model

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRegistry_LoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")

	reg, err := LoadRegistryFrom(path)
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}

	if len(reg.List()) != 0 {
		t.Fatal("expected empty registry")
	}

	now := time.Now().Truncate(time.Second)
	entry := ModelEntry{
		Name:       "mistral",
		OllamaName: "mistral:latest",
		Source:     "ollama",
		Size:       "7B",
		PulledAt:   now,
	}

	if err := reg.Add(entry); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Reload from disk
	reg2, err := LoadRegistryFrom(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if len(reg2.List()) != 1 {
		t.Fatalf("expected 1 model, got %d", len(reg2.List()))
	}

	got := reg2.List()[0]
	if got.Name != "mistral" {
		t.Errorf("name = %q, want %q", got.Name, "mistral")
	}
	if got.OllamaName != "mistral:latest" {
		t.Errorf("ollama_name = %q", got.OllamaName)
	}
}

func TestRegistry_AddUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")

	reg, _ := LoadRegistryFrom(path)

	reg.Add(ModelEntry{Name: "test", OllamaName: "test:latest", Size: "3B"})
	reg.Add(ModelEntry{Name: "test", OllamaName: "test:latest", Size: "7B"})

	if len(reg.List()) != 1 {
		t.Fatalf("expected 1 model after update, got %d", len(reg.List()))
	}
	if reg.List()[0].Size != "7B" {
		t.Errorf("size = %q, want %q after update", reg.List()[0].Size, "7B")
	}
}

func TestRegistry_Remove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")

	reg, _ := LoadRegistryFrom(path)
	reg.Add(ModelEntry{Name: "alpha", OllamaName: "alpha:latest"})
	reg.Add(ModelEntry{Name: "beta", OllamaName: "beta:latest"})

	if err := reg.Remove("alpha"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(reg.List()) != 1 {
		t.Fatalf("expected 1 model after remove, got %d", len(reg.List()))
	}
	if reg.List()[0].Name != "beta" {
		t.Errorf("remaining model = %q", reg.List()[0].Name)
	}
}

func TestRegistry_RemoveByOllamaName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")

	reg, _ := LoadRegistryFrom(path)
	reg.Add(ModelEntry{Name: "hf:org/model", OllamaName: "model:latest"})

	if err := reg.Remove("model:latest"); err != nil {
		t.Fatalf("remove by ollama name: %v", err)
	}

	if len(reg.List()) != 0 {
		t.Fatal("expected empty after remove")
	}
}

func TestRegistry_RemoveNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")

	reg, _ := LoadRegistryFrom(path)
	if err := reg.Remove("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent model")
	}
}

func TestRegistry_Get(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")

	reg, _ := LoadRegistryFrom(path)
	reg.Add(ModelEntry{Name: "test", OllamaName: "test:latest", Size: "3B"})

	// Get by display name
	m, err := reg.Get("test")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if m.Size != "3B" {
		t.Errorf("size = %q", m.Size)
	}

	// Get by ollama name
	m, err = reg.Get("test:latest")
	if err != nil {
		t.Fatalf("get by ollama name: %v", err)
	}
	if m.Name != "test" {
		t.Errorf("name = %q", m.Name)
	}

	// Not found
	_, err = reg.Get("nope")
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
}

func TestRegistry_UpdateLastUsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")

	reg, _ := LoadRegistryFrom(path)
	reg.Add(ModelEntry{Name: "test", OllamaName: "test:latest"})

	if err := reg.UpdateLastUsed("test"); err != nil {
		t.Fatalf("update last used: %v", err)
	}

	// Reload
	reg2, _ := LoadRegistryFrom(path)
	m := reg2.List()[0]
	if m.LastUsed.IsZero() {
		t.Fatal("expected last_used to be set")
	}
}

func TestRegistry_CreatesMissingDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "models.json")

	reg, err := LoadRegistryFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	reg.Add(ModelEntry{Name: "test", OllamaName: "test"})

	// Reload should work (directory was created)
	reg2, err := LoadRegistryFrom(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reg2.List()) != 1 {
		t.Fatal("expected 1 model after reload")
	}
}
