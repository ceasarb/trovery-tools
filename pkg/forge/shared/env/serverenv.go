package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// ResolveServerEnv loads env vars for an MCP server.
// Resolution order (first wins):
//  1. Process environment (shell exports + any prior LoadDotenv call)
//  2. Per-server .env file at serverDir/.env
//  3. Ancestor .env files (walks up from serverDir to filesystem root)
//
// Returns a slice of "KEY=VALUE" strings for injection into the subprocess,
// and an error if any required vars are missing.
func ResolveServerEnv(serverDir string, required []string) ([]string, error) {
	// Collect all .env files: server-level first, then ancestors (closest wins)
	envFiles := collectEnvFiles(serverDir)

	// Merge all .env files into a single map (first file wins per key)
	fileVars := make(map[string]string)
	for i := len(envFiles) - 1; i >= 0; i-- {
		vars, _ := godotenv.Read(envFiles[i])
		for k, v := range vars {
			fileVars[k] = v
		}
	}

	// Resolve each required var
	var missing []string
	var envPairs []string

	for _, key := range required {
		val := os.Getenv(key)
		if val == "" {
			if v, ok := fileVars[key]; ok {
				val = v
			}
		}
		if val == "" {
			missing = append(missing, key)
			continue
		}
		envPairs = append(envPairs, fmt.Sprintf("%s=%s", key, val))
	}

	if len(missing) > 0 {
		serverEnvFile := filepath.Join(serverDir, ".env")
		return nil, fmt.Errorf("missing required env vars: %s\n  Set them in:\n    • %s\n    • workspace .env\n    • shell environment",
			strings.Join(missing, ", "), serverEnvFile)
	}

	// Also pass through any other vars from .env files that aren't in required
	for k, v := range fileVars {
		alreadyIncluded := false
		for _, req := range required {
			if req == k {
				alreadyIncluded = true
				break
			}
		}
		if !alreadyIncluded {
			envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return envPairs, nil
}

// collectEnvFiles walks up from dir to the filesystem root, returning
// all .env file paths found (closest first).
func collectEnvFiles(dir string) []string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	var paths []string
	for {
		candidate := filepath.Join(abs, ".env")
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return paths
}
