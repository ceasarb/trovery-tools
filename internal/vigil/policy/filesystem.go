package policy

import (
	"fmt"
	"path/filepath"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
)

// FilesystemChecker validates file changes against filesystem policies.
type FilesystemChecker struct {
	policy config.FilesystemPolicy
}

// NewFilesystemChecker creates a new checker.
func NewFilesystemChecker(policy config.FilesystemPolicy) *FilesystemChecker {
	return &FilesystemChecker{policy: policy}
}

// CheckWrites validates file changes against the filesystem policy.
func (c *FilesystemChecker) CheckWrites(changes []session.FileChange) []session.PolicyViolation {
	var violations []session.PolicyViolation

	for _, fc := range changes {
		if fc.ChangeType == "deleted" {
			continue
		}

		// Check read-only violations.
		for _, pattern := range c.policy.ReadOnly {
			if matchGlob(pattern, fc.Path) {
				violations = append(violations, session.PolicyViolation{
					Rule:     "filesystem.read_only",
					Severity: "error",
					Message:  fmt.Sprintf("Write to read-only path: %s (matches %q)", fc.Path, pattern),
					File:     fc.Path,
				})
				break
			}
		}

		// Check no-write violations.
		for _, pattern := range c.policy.NoWrite {
			if matchGlob(pattern, fc.Path) {
				violations = append(violations, session.PolicyViolation{
					Rule:     "filesystem.no_write",
					Severity: "error",
					Message:  fmt.Sprintf("Write to restricted path: %s (matches %q)", fc.Path, pattern),
					File:     fc.Path,
				})
				break
			}
		}

		// Check allowed-write (if set, anything outside is a warning).
		if len(c.policy.AllowedWrite) > 0 {
			allowed := false
			for _, pattern := range c.policy.AllowedWrite {
				if matchGlob(pattern, fc.Path) {
					allowed = true
					break
				}
			}
			if !allowed {
				violations = append(violations, session.PolicyViolation{
					Rule:     "filesystem.allowed_write",
					Severity: "warning",
					Message:  fmt.Sprintf("Write outside allowed paths: %s", fc.Path),
					File:     fc.Path,
				})
			}
		}
	}

	return violations
}

// matchGlob matches a path against a glob pattern.
// Supports directory prefix matching (e.g., "src/" matches "src/main.go").
func matchGlob(pattern, path string) bool {
	// Direct match.
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	// Match against basename.
	base := filepath.Base(path)
	if matched, _ := filepath.Match(pattern, base); matched {
		return true
	}

	// Directory prefix match (pattern "src/" matches "src/foo.go").
	if len(pattern) > 0 && pattern[len(pattern)-1] == '/' {
		dir := pattern[:len(pattern)-1]
		if len(path) > len(dir) && path[:len(dir)] == dir && path[len(dir)] == '/' {
			return true
		}
		// Also match the dir itself.
		if path == dir {
			return true
		}
	}

	// Match pattern against each path component.
	for p := path; p != "" && p != "."; p = filepath.Dir(p) {
		if matched, _ := filepath.Match(pattern, filepath.Base(p)); matched {
			return true
		}
		if p == filepath.Dir(p) {
			break
		}
	}

	return false
}
