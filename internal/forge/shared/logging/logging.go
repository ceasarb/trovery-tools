package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Component identifies the subsystem producing a log message.
type Component string

const (
	ComponentRuntime     Component = "runtime"
	ComponentServerMgr   Component = "server_mgr"
	ComponentProvider    Component = "provider"
	ComponentOrchestrator Component = "orchestrator"
	ComponentHTTPServer  Component = "http_server"
	ComponentMetrics     Component = "metrics"
	ComponentOTel        Component = "otel"
)

// Format controls log output format.
type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// Config holds logging configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format Format // json, text
	Output io.Writer // defaults to os.Stderr
}

// logger is the package-level logger. Initialized to a no-op slog.Logger.
var logger *slog.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Init configures the global structured logger.
func Init(cfg Config) {
	level := parseLevel(cfg.Level)
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Format == FormatJSON {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// For returns a logger with the component field pre-set.
func For(component Component) *slog.Logger {
	return logger.With("component", string(component))
}

// Logger returns the current global logger.
func Logger() *slog.Logger {
	return logger
}

// parseLevel converts a string level name to slog.Level.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ParseLevel exports level parsing for use in tests and validation.
func ParseLevel(s string) slog.Level {
	return parseLevel(s)
}

// IsValidLevel checks if the given string is a recognized log level.
func IsValidLevel(s string) bool {
	switch strings.ToLower(s) {
	case "debug", "info", "warn", "warning", "error":
		return true
	default:
		return false
	}
}

// IsValidFormat checks if the given string is a recognized log format.
func IsValidFormat(s string) bool {
	switch strings.ToLower(s) {
	case "json", "text":
		return true
	default:
		return false
	}
}
