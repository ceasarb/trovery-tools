package adapters

import (
	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

// Adapter defines the interface for AI tool integrations.
type Adapter interface {
	// Name returns the tool identifier (e.g., "claude-code").
	Name() string

	// ResolveCommand builds the command to execute.
	ResolveCommand(cfg config.ToolConfig) []string

	// Run executes the tool and returns a ToolRun record.
	Run(cfg config.ToolConfig, extraArgs []string) (*session.ToolRun, error)

	// Validate checks if the tool is installed/available.
	Validate() error

	// InstallHint returns install instructions for the tool.
	InstallHint() string
}
