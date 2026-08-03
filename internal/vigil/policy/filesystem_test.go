package policy

import (
	"testing"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

func TestFilesystemChecker_ReadOnlyViolation(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		ReadOnly: []string{".env*", "*.pem"},
	})

	changes := []session.FileChange{
		{Path: ".env.production", ChangeType: "modified"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) == 0 {
		t.Error("expected read-only violation for .env.production")
	}
	if violations[0].Rule != "filesystem.read_only" {
		t.Errorf("expected rule filesystem.read_only, got %q", violations[0].Rule)
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected error severity, got %q", violations[0].Severity)
	}
}

func TestFilesystemChecker_PemViolation(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		ReadOnly: []string{"*.pem"},
	})

	changes := []session.FileChange{
		{Path: "certs/server.pem", ChangeType: "modified"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) == 0 {
		t.Error("expected read-only violation for .pem file")
	}
}

func TestFilesystemChecker_NoWriteViolation(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		NoWrite: []string{"production/"},
	})

	changes := []session.FileChange{
		{Path: "production/deploy.yaml", ChangeType: "added"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) == 0 {
		t.Error("expected no-write violation for production/")
	}
	if violations[0].Rule != "filesystem.no_write" {
		t.Errorf("expected rule filesystem.no_write, got %q", violations[0].Rule)
	}
}

func TestFilesystemChecker_AllowedWriteWarning(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		AllowedWrite: []string{"src/", "tests/"},
	})

	changes := []session.FileChange{
		{Path: "scripts/hack.sh", ChangeType: "added"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) == 0 {
		t.Error("expected warning for write outside allowed paths")
	}
	if violations[0].Severity != "warning" {
		t.Errorf("expected warning severity, got %q", violations[0].Severity)
	}
}

func TestFilesystemChecker_AllowedWriteNoViolation(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		AllowedWrite: []string{"src/", "tests/"},
	})

	changes := []session.FileChange{
		{Path: "src/main.go", ChangeType: "modified"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) != 0 {
		t.Errorf("expected no violations for allowed path, got %d", len(violations))
	}
}

func TestFilesystemChecker_SkipsDeleted(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		ReadOnly: []string{".env*"},
	})

	changes := []session.FileChange{
		{Path: ".env.test", ChangeType: "deleted"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) != 0 {
		t.Errorf("expected no violations for deleted files, got %d", len(violations))
	}
}

func TestFilesystemChecker_NoPolicy(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{})

	changes := []session.FileChange{
		{Path: "anything.go", ChangeType: "modified"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) != 0 {
		t.Errorf("expected no violations with empty policy, got %d", len(violations))
	}
}

func TestFilesystemChecker_GitDirProtected(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		ReadOnly: []string{".git/"},
	})

	changes := []session.FileChange{
		{Path: ".git/config", ChangeType: "modified"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) == 0 {
		t.Error("expected violation for .git/ write")
	}
}

func TestFilesystemChecker_SecretsDir(t *testing.T) {
	checker := NewFilesystemChecker(config.FilesystemPolicy{
		ReadOnly: []string{"secrets/"},
	})

	changes := []session.FileChange{
		{Path: "secrets/api_key.txt", ChangeType: "added"},
	}
	violations := checker.CheckWrites(changes)
	if len(violations) == 0 {
		t.Error("expected violation for secrets/ write")
	}
}
