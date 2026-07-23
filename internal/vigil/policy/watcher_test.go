package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
)

func TestFileWatcher_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()

	scanner, _ := NewSecretsScanner(config.SecretsPolicy{}, dir)
	fw, err := NewFileWatcher(dir, scanner)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := fw.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Create a file.
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\n"), 0644)
	time.Sleep(300 * time.Millisecond) // Wait for debounce.

	changes, _ := fw.Stop()
	if len(changes) == 0 {
		t.Error("expected at least one file change detected")
	}
}

func TestFileWatcher_DetectsSecretInRealTime(t *testing.T) {
	dir := t.TempDir()

	policy := config.DefaultConfig().Policies.Secrets
	scanner, _ := NewSecretsScanner(policy, dir)
	fw, err := NewFileWatcher(dir, scanner)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := fw.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Write a file with a secret.
	os.WriteFile(filepath.Join(dir, "config.go"), []byte("password = mysecret123\n"), 0644)
	time.Sleep(300 * time.Millisecond)

	_, violations := fw.Stop()
	if len(violations) == 0 {
		t.Error("expected real-time secret violation")
	}
}

func TestFileWatcher_IgnoresGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)

	scanner, _ := NewSecretsScanner(config.SecretsPolicy{}, dir)
	fw, err := NewFileWatcher(dir, scanner)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := fw.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Write inside .git — should be ignored.
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644)
	time.Sleep(300 * time.Millisecond)

	changes, _ := fw.Stop()
	for _, c := range changes {
		if c.Path == ".git/HEAD" || c.Path == filepath.Join(".git", "HEAD") {
			t.Error("expected .git changes to be ignored")
		}
	}
}

func TestFileWatcher_IgnoresNodeModules(t *testing.T) {
	dir := t.TempDir()
	nmDir := filepath.Join(dir, "node_modules")
	os.MkdirAll(nmDir, 0755)

	scanner, _ := NewSecretsScanner(config.SecretsPolicy{}, dir)
	fw, err := NewFileWatcher(dir, scanner)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := fw.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	os.WriteFile(filepath.Join(nmDir, "pkg.js"), []byte("module.exports = {};\n"), 0644)
	time.Sleep(300 * time.Millisecond)

	changes, _ := fw.Stop()
	for _, c := range changes {
		if c.Path == "node_modules/pkg.js" || c.Path == filepath.Join("node_modules", "pkg.js") {
			t.Error("expected node_modules changes to be ignored")
		}
	}
}

func TestFileWatcher_Debounce(t *testing.T) {
	dir := t.TempDir()

	scanner, _ := NewSecretsScanner(config.SecretsPolicy{}, dir)
	fw, err := NewFileWatcher(dir, scanner)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := fw.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Rapidly write to the same file.
	path := filepath.Join(dir, "rapid.txt")
	for i := 0; i < 5; i++ {
		os.WriteFile(path, []byte("update\n"), 0644)
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)

	changes, _ := fw.Stop()
	// Should have exactly 1 entry for the file (deduped by path).
	count := 0
	for _, c := range changes {
		if c.Path == "rapid.txt" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 deduped change for rapid.txt, got %d", count)
	}
}
