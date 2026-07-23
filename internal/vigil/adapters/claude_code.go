package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ceasarb/demigo-tools/internal/vigil/config"
	"github.com/ceasarb/demigo-tools/internal/vigil/session"
)

// ClaudeCodeAdapter wraps the Claude Code CLI.
type ClaudeCodeAdapter struct{}

func (a *ClaudeCodeAdapter) Name() string { return "claude-code" }

func (a *ClaudeCodeAdapter) ResolveCommand(cfg config.ToolConfig) []string {
	args := []string{"claude"}
	if model, ok := cfg.Config["model"]; ok {
		args = append(args, "--model", model)
	}
	if maxTokens, ok := cfg.Config["max_tokens"]; ok {
		args = append(args, "--max-tokens", maxTokens)
	}
	return args
}

func (a *ClaudeCodeAdapter) Run(cfg config.ToolConfig, extraArgs []string) (*session.ToolRun, error) {
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
			return run, fmt.Errorf("running claude-code: %w", err)
		}
	}

	return run, nil
}

func (a *ClaudeCodeAdapter) Validate() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH")
	}
	return nil
}

func (a *ClaudeCodeAdapter) InstallHint() string {
	return "npm install -g @anthropic-ai/claude-code"
}
