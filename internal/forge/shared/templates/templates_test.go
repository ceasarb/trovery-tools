package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRender(t *testing.T) {
	ctx := Context{
		ServiceName:   "weather-api",
		ToolPrefix:    "weather_api",
		PythonPackage: "weather_api",
		Description:   "A weather service",
		Author:        "test",
		Transport:     "stdio",
	}

	tmpl := `name = "{{.ServiceName}}"
package = "{{.PythonPackage}}"
desc = "{{.Description}}"`

	result, err := Render(tmpl, ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if result != `name = "weather-api"
package = "weather_api"
desc = "A weather service"` {
		t.Errorf("unexpected result:\n%s", result)
	}
}

func TestRenderTemplateFunctions(t *testing.T) {
	ctx := Context{ServiceName: "my-cool-server"}

	tests := []struct {
		tmpl string
		want string
	}{
		{`{{snakeCase .ServiceName}}`, "my_cool_server"},
		{`{{kebabCase .ServiceName}}`, "my-cool-server"},
		{`{{upper .ServiceName}}`, "MY-COOL-SERVER"},
		{`{{lower .ServiceName}}`, "my-cool-server"},
	}

	for _, tc := range tests {
		result, err := Render(tc.tmpl, ctx)
		if err != nil {
			t.Errorf("Render(%q): %v", tc.tmpl, err)
			continue
		}
		if result != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.tmpl, result, tc.want)
		}
	}
}

func TestRenderInvalidTemplate(t *testing.T) {
	ctx := Context{}
	_, err := Render("{{.Missing}", ctx)
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestWriteFiles(t *testing.T) {
	dir := t.TempDir()

	files := []File{
		{Path: "root.txt", Content: "hello"},
		{Path: "sub/nested.txt", Content: "world"},
		{Path: "sub/deep/file.txt", Content: "deep"},
	}

	if err := WriteFiles(dir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Path))
		if err != nil {
			t.Errorf("read %s: %v", f.Path, err)
			continue
		}
		if string(data) != f.Content {
			t.Errorf("%s content = %q, want %q", f.Path, string(data), f.Content)
		}
	}
}
