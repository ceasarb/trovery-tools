package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

// CodexAdapter wraps the OpenAI Codex CLI.
type CodexAdapter struct{}

func (a *CodexAdapter) Name() string { return "codex" }

func (a *CodexAdapter) ResolveCommand(cfg config.ToolConfig) []string {
	args := []string{"codex"}
	if model, ok := cfg.Config["model"]; ok {
		args = append(args, "--model", model)
	}
	return args
}

func (a *CodexAdapter) Run(cfg config.ToolConfig, extraArgs []string) (*session.ToolRun, error) {
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
			return run, fmt.Errorf("running codex: %w", err)
		}
	}

	return run, nil
}

func (a *CodexAdapter) Validate() error {
	_, err := exec.LookPath("codex")
	if err != nil {
		return fmt.Errorf("codex not found on PATH")
	}
	return nil
}

func (a *CodexAdapter) InstallHint() string {
	return "npm install -g @openai/codex"
}
