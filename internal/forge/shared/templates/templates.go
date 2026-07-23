package templates

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Context holds the variables for template rendering.
type Context struct {
	ServiceName   string
	ToolPrefix    string
	PythonPackage string
	Description   string
	Author        string
	Transport     string
}

// File represents a rendered template output.
type File struct {
	Path    string
	Content string
}

// Render processes a template string with the given context.
func Render(tmpl string, ctx Context) (string, error) {
	funcMap := template.FuncMap{
		"lower":      strings.ToLower,
		"upper":      strings.ToUpper,
		"title":      strings.Title,
		"replace":    strings.ReplaceAll,
		"snakeCase":  toSnakeCase,
		"kebabCase":  toKebabCase,
		"camelCase":  toCamelCase,
		"pascalCase": toPascalCase,
	}

	t, err := template.New("").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// WriteFiles writes rendered files to the given base directory.
func WriteFiles(baseDir string, files []File) error {
	for _, f := range files {
		path := filepath.Join(baseDir, f.Path)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
		if err := os.WriteFile(path, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return nil
}

func toSnakeCase(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "-", "_")
}

func toKebabCase(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", "-")
}

func toCamelCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	if len(parts) > 0 && len(parts[0]) > 0 {
		parts[0] = strings.ToLower(parts[0][:1]) + parts[0][1:]
	}
	return strings.Join(parts, "")
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
