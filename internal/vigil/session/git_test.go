package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v failed: %s %v", args, out, err)
		}
	}

	// Create initial commit.
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-m", "initial")

	return dir
}

func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s %v", args, out, err)
	}
}

func TestIsGitRepo(t *testing.T) {
	dir := setupGitRepo(t)
	if !IsGitRepo(dir) {
		t.Error("expected git repo")
	}

	notGit := t.TempDir()
	if IsGitRepo(notGit) {
		t.Error("expected not a git repo")
	}
}

func TestSnapshot(t *testing.T) {
	dir := setupGitRepo(t)

	snap, err := Snapshot(dir)
	if err != nil {
		t.Fatalf("snapshot error: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.HeadSHA == "" {
		t.Error("expected non-empty HEAD SHA")
	}
	if snap.Branch == "" {
		t.Error("expected non-empty branch")
	}
}

func TestSnapshot_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	snap, err := Snapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap != nil {
		t.Error("expected nil snapshot for non-git repo")
	}
}

func TestSnapshot_DirtyFiles(t *testing.T) {
	dir := setupGitRepo(t)

	// Create an untracked file.
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

	snap, err := Snapshot(dir)
	if err != nil {
		t.Fatalf("snapshot error: %v", err)
	}
	if len(snap.DirtyFiles) == 0 {
		t.Error("expected dirty files")
	}
}

func TestDiffSince(t *testing.T) {
	dir := setupGitRepo(t)

	snap, _ := Snapshot(dir)

	// Make a change and commit.
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0644)
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-m", "add file")

	changes, err := DiffSince(dir, snap)
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}
	if len(changes) == 0 {
		t.Error("expected file changes")
	}

	found := false
	for _, c := range changes {
		if c.Path == "new.go" {
			found = true
			if c.Source != "committed" {
				t.Errorf("expected source 'committed', got %q", c.Source)
			}
		}
	}
	if !found {
		t.Error("expected new.go in changes")
	}
}

func TestDiffSince_UncommittedChanges(t *testing.T) {
	dir := setupGitRepo(t)

	snap, _ := Snapshot(dir)

	// Unstaged: modify a tracked file without staging.
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified"), 0644)

	// Staged: create and stage a file.
	os.WriteFile(filepath.Join(dir, "staged.go"), []byte("package staged\n"), 0644)
	gitExec(t, dir, "add", "staged.go")

	// Untracked: create a file without adding.
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("hello"), 0644)

	changes, err := DiffSince(dir, snap)
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}

	sources := make(map[string]string)
	for _, c := range changes {
		sources[c.Path] = c.Source
	}

	if sources["README.md"] != "unstaged" {
		t.Errorf("expected README.md source 'unstaged', got %q", sources["README.md"])
	}
	if sources["staged.go"] != "staged" {
		t.Errorf("expected staged.go source 'staged', got %q", sources["staged.go"])
	}
	if sources["untracked.txt"] != "untracked" {
		t.Errorf("expected untracked.txt source 'untracked', got %q", sources["untracked.txt"])
	}
}

func TestDiffSince_DeduplicatesCommittedOverWorking(t *testing.T) {
	dir := setupGitRepo(t)

	snap, _ := Snapshot(dir)

	// Commit a file, then also modify it in the working tree.
	os.WriteFile(filepath.Join(dir, "both.go"), []byte("package both\n"), 0644)
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-m", "add both.go")

	// Now modify it again (unstaged).
	os.WriteFile(filepath.Join(dir, "both.go"), []byte("package both\n// changed\n"), 0644)

	changes, err := DiffSince(dir, snap)
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}

	count := 0
	for _, c := range changes {
		if c.Path == "both.go" {
			count++
			if c.Source != "committed" {
				t.Errorf("expected source 'committed' (priority), got %q", c.Source)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 entry for both.go, got %d", count)
	}
}
