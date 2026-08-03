package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pythonRequirementsDockerfile = `FROM python:3.13-slim AS builder
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

FROM python:3.13-slim
RUN useradd -r -s /bin/false trove
WORKDIR /app
COPY --from=builder /usr/local/lib/python3.13/site-packages /usr/local/lib/python3.13/site-packages
COPY . .
USER trove
`

const pythonPyprojectDockerfile = `FROM python:3.13-slim
WORKDIR /app
COPY . .
RUN pip install --no-cache-dir .
USER nobody
`

const typescriptDockerfile = `FROM node:22-slim AS builder
WORKDIR /app
COPY package*.json .
RUN npm ci --production

FROM node:22-slim
RUN useradd -r -s /bin/false trove
WORKDIR /app
COPY --from=builder /app/node_modules ./node_modules
COPY . .
USER trove
`

// Language represents the detected server language.
type Language string

const (
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
)

// DetectLanguage examines the server directory for language markers.
// It checks for requirements.txt or pyproject.toml (Python) and
// package.json (TypeScript/Node).
func DetectLanguage(dir string) (Language, error) {
	pythonMarkers := []string{"requirements.txt", "pyproject.toml"}
	for _, f := range pythonMarkers {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return LangPython, nil
		}
	}

	tsMarkers := []string{"package.json", "tsconfig.json"}
	for _, f := range tsMarkers {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return LangTypeScript, nil
		}
	}

	return "", fmt.Errorf("cannot detect server language in %s (no requirements.txt, pyproject.toml, or package.json found)", dir)
}

// GenerateDockerfile returns the Dockerfile content for the given language,
// with an optional CMD appended from the server command.
func GenerateDockerfile(lang Language, serverCommand string) string {
	return GenerateDockerfileForDir(lang, serverCommand, "")
}

// GenerateDockerfileForDir returns the Dockerfile content, choosing the
// right template based on what files exist in the server directory.
func GenerateDockerfileForDir(lang Language, serverCommand string, dir string) string {
	var base string
	switch lang {
	case LangPython:
		base = pickPythonDockerfile(dir)
	case LangTypeScript:
		base = typescriptDockerfile
	default:
		base = pickPythonDockerfile(dir)
	}

	if serverCommand != "" {
		parts := strings.Fields(serverCommand)
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = fmt.Sprintf("%q", p)
		}
		base += fmt.Sprintf("CMD [%s]\n", strings.Join(quoted, ", "))
	}

	return base
}

func pickPythonDockerfile(dir string) string {
	if dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
			return pythonRequirementsDockerfile
		}
	}
	return pythonPyprojectDockerfile
}

// WriteDockerfile writes a Dockerfile.trove to the given directory.
func WriteDockerfile(dir string, lang Language, serverCommand string) (string, error) {
	content := GenerateDockerfileForDir(lang, serverCommand, dir)
	path := filepath.Join(dir, "Dockerfile.trove")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write Dockerfile.trove: %w", err)
	}
	return path, nil
}
