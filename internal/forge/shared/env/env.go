package env

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDotenv searches for .env files and loads them into the process environment.
// It walks up from CWD to filesystem root, loading every .env file found.
// Files loaded later do NOT override values already set in the environment
// or by earlier files (closest .env wins).
// Silently does nothing if no .env files are found.
func LoadDotenv() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	// Walk up from CWD, collecting .env paths (closest first)
	var paths []string
	dir, _ := filepath.Abs(cwd)
	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	for _, p := range paths {
		// godotenv.Load does NOT override existing env vars
		godotenv.Load(p) //nolint:errcheck
	}
}
