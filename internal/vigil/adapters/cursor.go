package adapters

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

// CursorAdapter handles Cursor IDE integration.
// Cursor is GUI-based — Vigil provides pre/post session hooks only.
// The adapter opens Cursor and blocks until the user closes it or stops the session.
type CursorAdapter struct{}

func (a *CursorAdapter) Name() string { return "cursor" }

func (a *CursorAdapter) ResolveCommand(cfg config.ToolConfig) []string {
	args := []string{"cursor"}
	if workspace, ok := cfg.Config["workspace"]; ok {
		args = append(args, workspace)
	} else {
		args = append(args, ".")
	}
	return args
}

func (a *CursorAdapter) Run(cfg config.ToolConfig, extraArgs []string) (*session.ToolRun, error) {
	args := a.ResolveCommand(cfg)
	args = append(args, extraArgs...)

	run := &session.ToolRun{
		Tool:      a.Name(),
		Command:   fmt.Sprintf("%v", args),
		StartTime: time.Now(),
	}

	// Cursor is GUI-based — launch with --wait so the process blocks.
	waitArgs := append([]string{"--wait"}, args[1:]...)
	cmd := exec.Command(args[0], waitArgs...)

	err := cmd.Run()
	run.EndTime = time.Now()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			run.ExitCode = exitErr.ExitCode()
		} else {
			return run, fmt.Errorf("running cursor: %w", err)
		}
	}

	return run, nil
}

func (a *CursorAdapter) Validate() error {
	_, err := exec.LookPath("cursor")
	if err != nil {
		return fmt.Errorf("cursor not found on PATH")
	}
	return nil
}

func (a *CursorAdapter) InstallHint() string {
	return "Install Cursor from https://cursor.sh and add 'cursor' to your PATH (Shell Command: Install 'cursor' command)"
}
