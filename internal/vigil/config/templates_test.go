package config

import (
	"strings"
	"testing"
)

func TestRenderTemplate_Default(t *testing.T) {
	content, err := RenderTemplate("default", TemplateData{Name: "my-project"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, `name: "my-project"`) {
		t.Error("expected project name in rendered template")
	}
	if !strings.Contains(s, "claude-code") {
		t.Error("expected claude-code in default template")
	}
}

func TestRenderTemplate_Python(t *testing.T) {
	content, err := RenderTemplate("python", TemplateData{Name: "py-app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, `name: "py-app"`) {
		t.Error("expected project name in rendered template")
	}
	if !strings.Contains(s, "codex") {
		t.Error("expected codex in python template")
	}
}

func TestRenderTemplate_Typescript(t *testing.T) {
	content, err := RenderTemplate("typescript", TemplateData{Name: "ts-app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "npm_") {
		t.Error("expected npm token pattern in typescript template")
	}
}

func TestRenderTemplate_Unknown(t *testing.T) {
	_, err := RenderTemplate("nonexistent", TemplateData{Name: "test"})
	if err == nil {
		t.Error("expected error for unknown template")
	}
}

func TestAvailableTemplates(t *testing.T) {
	templates := AvailableTemplates()
	if len(templates) != 3 {
		t.Errorf("expected 3 templates, got %d", len(templates))
	}
}
