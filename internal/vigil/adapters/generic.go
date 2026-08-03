package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

// GenericAdapter wraps any CLI command.
type GenericAdapter struct {
	command []string
}

// NewGenericAdapter creates an adapter for an arbitrary command.
func NewGenericAdapter(command []string) *GenericAdapter {
	return &GenericAdapter{command: command}
}

func (a *GenericAdapter) Name() string { return "generic" }

func (a *GenericAdapter) ResolveCommand(cfg config.ToolConfig) []string {
	return a.command
}

func (a *GenericAdapter) Run(cfg config.ToolConfig, extraArgs []string) (*session.ToolRun, error) {
	args := append(a.command, extraArgs...)

	run := &session.ToolRun{
		Tool:      "generic",
		Command:   strings.Join(args, " "),
		StartTime: time.Now(),
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("no command specified")
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
			return run, fmt.Errorf("running command: %w", err)
		}
	}

	return run, nil
}

func (a *GenericAdapter) Validate() error {
	if len(a.command) == 0 {
		return fmt.Errorf("no command specified")
	}
	_, err := exec.LookPath(a.command[0])
	if err != nil {
		return fmt.Errorf("%s not found on PATH", a.command[0])
	}
	return nil
}

func (a *GenericAdapter) InstallHint() string {
	return "ensure the command is on your PATH"
}
