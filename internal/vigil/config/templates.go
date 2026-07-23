package config

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.yaml
var templateFS embed.FS

// TemplateData holds values for template rendering.
type TemplateData struct {
	Name string
}

// AvailableTemplates returns the list of template names.
func AvailableTemplates() []string {
	return []string{"default", "python", "typescript"}
}

// RenderTemplate renders a config template by name.
func RenderTemplate(name string, data TemplateData) ([]byte, error) {
	filename := fmt.Sprintf("templates/%s.yaml", name)
	content, err := templateFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("template %q not found (available: default, python, typescript)", name)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering template: %w", err)
	}

	return buf.Bytes(), nil
}
