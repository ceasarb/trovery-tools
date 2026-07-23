package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "my-project")

	path, err := Init(name, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Check marker file exists
	if _, err := os.Stat(filepath.Join(path, MarkerFile)); err != nil {
		t.Errorf("marker file missing: %v", err)
	}

	// Check directories
	for _, d := range []string{"servers", "agents"} {
		if _, err := os.Stat(filepath.Join(path, d)); err != nil {
			t.Errorf("%s directory missing: %v", d, err)
		}
	}

	// Check README and .gitignore
	for _, f := range []string{"README.md", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(path, f)); err != nil {
			t.Errorf("%s missing: %v", f, err)
		}
	}
}

func TestInitNoServers(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "agent-only")

	path, err := Init(name, true)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// servers/ should NOT exist
	if _, err := os.Stat(filepath.Join(path, "servers")); !os.IsNotExist(err) {
		t.Errorf("servers/ should not exist with --no-servers")
	}

	// agents/ should exist
	if _, err := os.Stat(filepath.Join(path, "agents")); err != nil {
		t.Errorf("agents/ should exist: %v", err)
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()

	// Create workspace
	_, err := Init(dir, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create nested directory
	nested := filepath.Join(dir, "servers", "weather")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Find from nested should walk up to workspace root
	ws, err := Find(nested)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ws == nil {
		t.Fatal("Find returned nil, expected workspace")
	}
	if ws.Root != dir {
		t.Errorf("Root = %q, want %q", ws.Root, dir)
	}
}

func TestFindNotFound(t *testing.T) {
	dir := t.TempDir() // no marker file

	ws, err := Find(dir)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ws != nil {
		t.Errorf("expected nil, got workspace at %q", ws.Root)
	}
}

func TestDetectOutputDir(t *testing.T) {
	dir := t.TempDir()

	// Create workspace
	_, err := Init(dir, false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Change to workspace root
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	serverDir, err := DetectOutputDir("server")
	if err != nil {
		t.Fatalf("DetectOutputDir: %v", err)
	}
	if filepath.Base(serverDir) != "servers" {
		t.Errorf("server output dir = %q, want servers/", serverDir)
	}

	agentDir, err := DetectOutputDir("agent")
	if err != nil {
		t.Fatalf("DetectOutputDir: %v", err)
	}
	if filepath.Base(agentDir) != "agents" {
		t.Errorf("agent output dir = %q, want agents/", agentDir)
	}
}
