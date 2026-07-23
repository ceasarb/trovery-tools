package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsValidLevel(t *testing.T) {
	valid := []string{"debug", "info", "warn", "warning", "error"}
	for _, l := range valid {
		if !IsValidLevel(l) {
			t.Errorf("IsValidLevel(%q) = false, want true", l)
		}
	}
	invalid := []string{"", "trace", "fatal", "verbose"}
	for _, l := range invalid {
		if IsValidLevel(l) {
			t.Errorf("IsValidLevel(%q) = true, want false", l)
		}
	}
}

func TestIsValidFormat(t *testing.T) {
	if !IsValidFormat("json") {
		t.Error("json should be valid")
	}
	if !IsValidFormat("text") {
		t.Error("text should be valid")
	}
	if IsValidFormat("yaml") {
		t.Error("yaml should be invalid")
	}
}

func TestInitJSON(t *testing.T) {
	var buf bytes.Buffer
	Init(Config{Level: "debug", Format: FormatJSON, Output: &buf})

	log := For(ComponentRuntime)
	log.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, `"component":"runtime"`) && !strings.Contains(output, `"component": "runtime"`) {
		t.Errorf("expected component field in JSON output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected message in output, got: %s", output)
	}
}

func TestInitText(t *testing.T) {
	var buf bytes.Buffer
	Init(Config{Level: "info", Format: FormatText, Output: &buf})

	log := For(ComponentProvider)
	log.Info("provider ready")

	output := buf.String()
	if !strings.Contains(output, "component=provider") {
		t.Errorf("expected component field in text output, got: %s", output)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	Init(Config{Level: "warn", Format: FormatText, Output: &buf})

	log := For(ComponentRuntime)
	log.Info("should not appear")
	log.Warn("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("info message should be filtered at warn level")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("warn message should appear at warn level")
	}
}

func TestForComponents(t *testing.T) {
	components := []Component{
		ComponentRuntime, ComponentServerMgr, ComponentProvider,
		ComponentOrchestrator, ComponentHTTPServer, ComponentMetrics, ComponentOTel,
	}
	for _, c := range components {
		log := For(c)
		if log == nil {
			t.Errorf("For(%q) returned nil", c)
		}
	}
}
