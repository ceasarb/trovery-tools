package export

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/ceasarb/trovery-tools/pkg/forge/agent/config"
)

// Format identifies the export target.
type Format string

const (
	FormatPython    Format = "python"
	FormatFastAPI   Format = "fastapi"
	FormatDocker    Format = "docker"
	FormatMCPClient Format = "mcp-client"
)

// ValidFormats returns all supported export formats.
func ValidFormats() []Format {
	return []Format{FormatPython, FormatFastAPI, FormatDocker, FormatMCPClient}
}

// IsValidFormat checks whether f is a recognized format.
func IsValidFormat(f string) bool {
	for _, v := range ValidFormats() {
		if string(v) == f {
			return true
		}
	}
	return false
}

// ExportResult describes the files produced by an export.
type ExportResult struct {
	Format Format
	Files  []ExportedFile
}

// ExportedFile is a single file written during export.
type ExportedFile struct {
	Path        string
	Description string
}

// Exporter generates deployment artifacts from an agent config.
type Exporter struct {
	config *config.AgentConfig
	format Format
	outDir string
}

// New creates an Exporter for the given config, format, and output directory.
func New(cfg *config.AgentConfig, format Format, outDir string) *Exporter {
	return &Exporter{
		config: cfg,
		format: format,
		outDir: outDir,
	}
}

// Export generates the deployment artifacts and writes them to disk.
func (e *Exporter) Export() (*ExportResult, error) {
	if err := os.MkdirAll(e.outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	switch e.format {
	case FormatPython:
		return e.exportPython()
	case FormatFastAPI:
		return e.exportFastAPI()
	case FormatDocker:
		return e.exportDocker()
	case FormatMCPClient:
		return e.exportMCPClient()
	default:
		return nil, fmt.Errorf("unsupported format: %s", e.format)
	}
}

// classifyServers separates servers into bundled (local command) and external (URL).
func (e *Exporter) classifyServers() (bundled, external []config.ServerRef) {
	for _, s := range e.config.Servers {
		if s.URL != "" {
			external = append(external, s)
		} else {
			bundled = append(bundled, s)
		}
	}
	return
}

// renderTemplate renders a Go text/template string with the given data.
func renderTemplate(name, tmpl string, data any) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// writeFile writes content to a file inside the exporter's output directory.
func (e *Exporter) writeFile(relPath, content string) error {
	path := filepath.Join(e.outDir, relPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
