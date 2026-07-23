package safepath

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Resolve resolves a relative path against a base directory, ensuring
// the result stays within base. Returns an error if the path escapes.
func Resolve(base, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths not allowed: %s", rel)
	}

	resolved := filepath.Join(base, rel)
	resolved = filepath.Clean(resolved)

	// Check the resolved path is still under base
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolve base: %w", err)
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	relResult, err := filepath.Rel(absBase, absResolved)
	if err != nil {
		return "", fmt.Errorf("path escapes base directory: %s", rel)
	}

	if strings.HasPrefix(relResult, "..") {
		return "", fmt.Errorf("path escapes base directory: %s", rel)
	}

	return resolved, nil
}
