package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
)

// ForgeAdapter wraps Demigo Forge agent execution.
// Requires agent_path in the tool config.
type ForgeAdapter struct{}

func (a *ForgeAdapter) Name() string { return "forge-agent" }

func (a *ForgeAdapter) ResolveCommand(cfg config.ToolConfig) []string {
	args := []string{"demi-forge", "agent", "chat"}
	if cfg.AgentPath != "" {
		args = append(args, cfg.AgentPath)
	}
	return args
}

func (a *ForgeAdapter) Run(cfg config.ToolConfig, extraArgs []string) (*session.ToolRun, error) {
	args := a.ResolveCommand(cfg)
	args = append(args, extraArgs...)

	run := &session.ToolRun{
		Tool:      a.Name(),
		Command:   fmt.Sprintf("%v", args),
		StartTime: time.Now(),
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	run.EndTime = time.Now()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			run.ExitCode = exitErr.ExitCode()
		} else {
			return run, fmt.Errorf("running forge agent: %w", err)
		}
	}

	return run, nil
}

func (a *ForgeAdapter) Validate() error {
	_, err := exec.LookPath("demi-forge")
	if err != nil {
		return fmt.Errorf("demi-forge (Demigo Forge) not found on PATH")
	}
	return nil
}

func (a *ForgeAdapter) InstallHint() string {
	return "go install github.com/ceasarb/demigo-tools/cmd/demi-forge@latest"
}
