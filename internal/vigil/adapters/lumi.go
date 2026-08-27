package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ceasarb/trovery-tools/internal/vigil/config"
	"github.com/ceasarb/trovery-tools/internal/vigil/session"
)

// LumiAdapter wraps the Lumi personal-agent harness — the first non-coding
// harness Vigil observes (PDR-010). Lumi takes its request as arguments, so
// this adapter is a passthrough: `vigil run lumi polish this note`.
type LumiAdapter struct{}

func (a *LumiAdapter) Name() string { return "lumi" }

func (a *LumiAdapter) ResolveCommand(cfg config.ToolConfig) []string {
	return []string{"lumi"}
}

func (a *LumiAdapter) Run(cfg config.ToolConfig, extraArgs []string) (*session.ToolRun, error) {
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
			return run, fmt.Errorf("running lumi: %w", err)
		}
	}

	return run, nil
}

func (a *LumiAdapter) Validate() error {
	_, err := exec.LookPath("lumi")
	if err != nil {
		return fmt.Errorf("lumi not found on PATH")
	}
	return nil
}

func (a *LumiAdapter) InstallHint() string {
	return "go install github.com/ceasarb/lumi/cmd/lumi@latest"
}
